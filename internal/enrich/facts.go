package enrich

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/YakDriver/avizor/internal/smithy"
)

// SchemaPlace is where a new member's parent block is defined in a provider file: its Terraform path from the
// resource's top level, its schema kind, and the nearest block on the path, the parent's own included, that forces
// replacement. These are facts about the code; whether the new member should force replacement is the reader's call.
type SchemaPlace struct {
	File string `json:"file"`
	Line int    `json:"line"`
	// Path is the parent block's path, as configuration.vpc_egress_configuration.
	Path string `json:"path"`
	// Kind is the schema type: ListNestedBlock, SingleNestedAttribute, TypeList (MaxItems 1), and so on.
	Kind string `json:"kind"`
	// Framework is "plugin-framework" or "sdkv2".
	Framework string `json:"framework"`
	// ForceNew is the path of the nearest block on Path, the parent included, that forces replacement (ForceNew or a
	// RequiresReplace plan modifier), or "".
	ForceNew string `json:"force_new,omitempty"`
}

// EnumUse is a line in the provider package that uses an enum's SDK type. Validated is set when the use validates
// or types values with the SDK type (enum.Validate[T], enum.FrameworkValidate[T], fwtypes.StringEnum[T], ...), so
// a new value is accepted without a code change. Whether the new value matters is the reader's call.
type EnumUse struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Validated bool   `json:"validated,omitempty"`
	Code      string `json:"code"`
}

// schemaPlaces finds the parent block of each new nested argument and attribute in the files that call its
// operations.
func (e *Enricher) schemaPlaces(ws []Work, m *smithy.Model) {
	if m == nil {
		return
	}
	attrs := attrConstants(e.Checkout)
	inputs := operationInputs(m)
	outputs := operationOutputs(m)
	type file struct {
		ast       *ast.File
		fset      *token.FileSet
		framework string
	}
	files := map[string]*file{}
	read := func(f string) *file {
		if x, ok := files[f]; ok {
			return x
		}
		x := &file{fset: token.NewFileSet()}
		if b, err := os.ReadFile(filepath.Join(e.Checkout, f)); err == nil {
			x.ast, _ = parser.ParseFile(x.fset, f, b, parser.SkipObjectResolution)
		}
		if x.ast != nil {
			x.framework = frameworkOf(x.ast)
		}
		files[f] = x
		return x
	}
	for i := range ws {
		w := &ws[i]
		if (w.Kind != WorkNewArgument && w.Kind != WorkNewAttribute) || len(w.UsedBy) == 0 {
			continue
		}
		shape, _, _ := strings.Cut(w.Subject, ".")
		if inputs[shape] || outputs[shape] {
			// A top-level member: its parent is the resource.
			continue
		}
		for _, f := range w.UsedBy {
			x := read(f)
			if x.ast == nil {
				continue
			}
			names := parentBlockNames(x.ast, m, shape, inputs, attrs)
			for _, p := range placesOf(x.ast, x.fset, names, attrs) {
				p.File, p.Framework = f, x.framework
				w.Schema = append(w.Schema, p)
			}
		}
	}
}

// operationOutputs returns the model's operation output shapes.
func operationOutputs(m *smithy.Model) map[string]bool {
	out := map[string]bool{}
	for _, op := range m.Operations() {
		if o := m.Shapes[op].Output; o != "" {
			out[o] = true
		}
	}
	return out
}

// parentBlockNames returns the Terraform names of shape's parent blocks in f, moving up through flattened shapes as
// inBlock does, or nil if f has none.
func parentBlockNames(f *ast.File, m *smithy.Model, shape string, inputs map[string]bool, attrs map[string]string) []string {
	level := []string{shape}
	for range maxClimb {
		up, names := parents(m, level)
		if len(blocks(f, names, attrs)) > 0 {
			return names
		}
		if slices.ContainsFunc(up, func(s string) bool { return inputs[s] && wraps(m, s, level) }) {
			return nil
		}
		if len(up) == 0 || !slices.ContainsFunc(level, func(s string) bool { return m.Shapes[s].Type == smithy.TypeUnion }) {
			return nil
		}
		level = up
	}
	return nil
}

// placesOf returns the schema definitions in f whose key is one of names, with their paths.
func placesOf(f *ast.File, fset *token.FileSet, names []string, attrs map[string]string) []SchemaPlace {
	if len(names) == 0 {
		return nil
	}
	var out []SchemaPlace
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok || !slices.ContainsFunc(names, func(name string) bool { return isName(kv.Key, name, attrs) }) {
			return true
		}
		kind, def := schemaKind(kv.Value)
		if def == nil {
			return true
		}
		// The path is the keys of the schema definitions that enclose this one, outermost first.
		var path []string
		forceNew := ""
		var enclosing []*ast.KeyValueExpr
		for _, a := range stack {
			// Only named entries count: "name": schema.X{...}. Elem, NestedObject, and Attributes are structure.
			if k, ok := a.(*ast.KeyValueExpr); ok && isSchemaKey(k.Key) {
				if _, d := schemaKind(k.Value); d != nil {
					enclosing = append(enclosing, k)
				}
			}
		}
		for _, k := range enclosing {
			path = append(path, keyName(k.Key, attrs))
			if _, d := schemaKind(k.Value); forcesNew(d) {
				forceNew = strings.Join(path, ".")
			}
		}
		out = append(out, SchemaPlace{Line: fset.Position(kv.Pos()).Line, Path: strings.Join(path, "."), Kind: kind, ForceNew: forceNew})
		return true
	})
	return out
}

// schemaKind returns the schema type of a schema definition and the composite literal that defines it, or nil if v
// isn't one: schema.ListNestedBlock{...}, &schema.Schema{Type: schema.TypeList, ...}.
func schemaKind(v ast.Expr) (string, *ast.CompositeLit) {
	if u, ok := v.(*ast.UnaryExpr); ok && u.Op == token.AND {
		v = u.X
	}
	cl, ok := v.(*ast.CompositeLit)
	if !ok {
		return "", nil
	}
	switch t := cl.Type.(type) {
	case nil:
		// An SDKv2 map value with its type left out: "name": {Type: schema.TypeList, ...}.
		if !hasSchemaType(cl) {
			return "", nil
		}
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); !ok || x.Name != "schema" {
			return "", nil
		}
		if t.Sel.Name != "Schema" {
			return t.Sel.Name, cl
		}
	default:
		return "", nil
	}
	// SDKv2: the Type field, and MaxItems 1 for a single block.
	kind, single := "Schema", false
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch keyIdent(kv.Key) {
		case "Type":
			if s, ok := kv.Value.(*ast.SelectorExpr); ok {
				kind = s.Sel.Name
			}
		case "MaxItems":
			if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Value == "1" {
				single = true
			}
		}
	}
	if single {
		kind += " (MaxItems 1)"
	}
	return kind, cl
}

// hasSchemaType reports whether a composite literal has a Type: schema.TypeX field.
func hasSchemaType(cl *ast.CompositeLit) bool {
	for _, el := range cl.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok && keyIdent(kv.Key) == "Type" {
			if s, ok := kv.Value.(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "Type") {
				if x, ok := s.X.(*ast.Ident); ok && x.Name == "schema" {
					return true
				}
			}
		}
	}
	return false
}

// forcesNew reports whether a schema definition has ForceNew: true or a RequiresReplace plan modifier.
func forcesNew(cl *ast.CompositeLit) bool {
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch keyIdent(kv.Key) {
		case "ForceNew":
			if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "true" {
				return true
			}
		case "PlanModifiers":
			found := false
			ast.Inspect(kv.Value, func(n ast.Node) bool {
				if s, ok := n.(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "RequiresReplace") {
					found = true
				}
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}

// isSchemaKey reports whether a map key names an attribute or block: a string literal or a names.Attr constant.
func isSchemaKey(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		return ok && x.Name == "names"
	}
	return false
}

func keyIdent(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// keyName returns a schema key's Terraform name: a string literal, or a names.Attr constant's value.
func keyName(e ast.Expr, attrs map[string]string) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		if v, err := strconv.Unquote(e.Value); err == nil {
			return v
		}
	case *ast.SelectorExpr:
		for name, c := range attrs {
			if c == e.Sel.Name {
				return name
			}
		}
		return e.Sel.Name
	}
	return "?"
}

// frameworkOf says which plugin library a file's schema uses, from its imports.
func frameworkOf(f *ast.File) string {
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		switch {
		case strings.HasPrefix(p, "github.com/hashicorp/terraform-plugin-framework/"):
			return "plugin-framework"
		case strings.HasPrefix(p, "github.com/hashicorp/terraform-plugin-sdk/v2/"):
			return "sdkv2"
		}
	}
	return ""
}

// validatedRE matches uses that validate or type values with the SDK enum type.
var validatedRE = regexp.MustCompile(`\benum\.(?:Validate|FrameworkValidate|Values|EnumValues)\[|\bfwtypes\.(?:StringEnum|StringEnumType|ListOfStringEnum|SetOfStringEnum|ListOfStringEnumType|SetOfStringEnumType)\[`)

// enumUses records, for each new enum value entry, the lines in the provider package that use the enum's SDK type.
func (e *Enricher) enumUses(ctx context.Context, pkg string, ws []Work) error {
	var enums []string
	for _, w := range ws {
		if w.Kind == WorkNewEnumValue {
			enums = append(enums, goName(w.Subject))
		}
	}
	if len(enums) == 0 {
		return nil
	}
	if e.rgPath == "" {
		p, err := exec.LookPath("rg")
		if err != nil {
			return fmt.Errorf("ripgrep (rg) not found")
		}
		e.rgPath = p
	}
	dir := filepath.Join(e.Checkout, "internal", "service", pkg)
	// A use of the type from the SDK's types package, imported as awstypes, types, or <service>types. Struct fields of
	// the same name (output.StateReasonCode) don't match.
	pattern := `\b[a-zA-Z0-9]*[tT]ypes\.(` + strings.Join(enums, "|") + `)\b`
	cmd := exec.CommandContext(ctx, e.rgPath, "--no-heading", "--with-filename", "--line-number", "--glob", "*.go", "--glob", "!*_test.go", "-e", pattern, dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			return fmt.Errorf("rg: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	re := regexp.MustCompile(pattern)
	uses := map[string][]EnumUse{}
	for line := range strings.Lines(string(out)) {
		// "<path>:<line>:<code>"
		path, rest, ok := strings.Cut(strings.TrimRight(line, "\n"), ":")
		if !ok {
			continue
		}
		num, code, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(e.Checkout, path)
		if err != nil {
			rel = path
		}
		code = strings.TrimSpace(code)
		for _, sm := range re.FindAllStringSubmatch(code, -1) {
			u := EnumUse{File: rel, Line: n, Validated: validatedRE.MatchString(code), Code: code}
			if l := uses[sm[1]]; len(l) == 0 || l[len(l)-1].File != u.File || l[len(l)-1].Line != u.Line {
				uses[sm[1]] = append(l, u)
			}
		}
	}
	for i := range ws {
		if ws[i].Kind == WorkNewEnumValue {
			ws[i].EnumSearched = true
			ws[i].EnumUses = uses[goName(ws[i].Subject)]
			slices.SortFunc(ws[i].EnumUses, func(a, b EnumUse) int {
				if c := strings.Compare(a.File, b.File); c != 0 {
					return c
				}
				return a.Line - b.Line
			})
		}
	}
	return nil
}

// goName is the SDK for Go's exported name for a shape: its first letter upper-cased.
func goName(shape string) string {
	r, n := utf8.DecodeRuneInString(shape)
	return string(unicode.ToUpper(r)) + shape[n:]
}
