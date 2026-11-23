package enrich

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/YakDriver/avizor/internal/smithy"
)

// Snake turns a PascalCase or camelCase name into snake_case: S3AccessPoint -> s3_access_point, KMSKey -> kms_key.
func Snake(s string) string {
	rs := []rune(acronymRE.ReplaceAllStringFunc(s, func(m string) string {
		sm := acronymRE.FindStringSubmatch(m)
		w := sm[1][:1] + strings.ToLower(sm[1][1:])
		if sm[2] != "" {
			// A lowercase letter after the acronym starts a word: MiBps is mib_ps.
			w += "_" + sm[2]
		}
		return w
	}))
	var b strings.Builder
	for i, r := range rs {
		if unicode.IsUpper(r) && i > 0 {
			prevLower := unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if prevLower || (unicode.IsUpper(rs[i-1]) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// acronymRE matches mixed-case acronyms that are one word in Terraform names, and a lowercase letter after one: OAuth
// (oauth2), IPv4 and IPv6 (ipv6), and MiB and GiB (warm_throughput_mib_ps, memory_gib_per_vcpu). Snake would split
// them where the case changes.
var acronymRE = regexp.MustCompile(`(OAuth|IPv|[MG]iB)([a-z]?)`)

// queryVerbs start operations whose input is filters and identifiers, which providers rarely expose as arguments.
var queryVerbs = []string{"Get", "Describe", "List", "BatchGet", "Search"}

// writes reports whether any of ops isn't a query.
func writes(ops []string) bool {
	return slices.ContainsFunc(ops, func(op string) bool {
		return !slices.ContainsFunc(queryVerbs, func(v string) bool { return strings.HasPrefix(op, v) })
	})
}

// MemberNames are the Terraform names a new member would likely have: its snake_case name, and the singular form,
// since providers name a block for a list of structures in the singular.
func MemberNames(member string) []string {
	n := Snake(member)
	out := []string{n}
	for _, suffix := range []string{"ies", "es", "s"} {
		if base, ok := strings.CutSuffix(n, suffix); ok && base != "" {
			if suffix == "ies" {
				base += "y"
			}
			out = append(out, base)
		}
	}
	return slices.Compact(out)
}

// findMembers records, for each new argument of a write operation, its likely Terraform name and the files calling
// its operations that already use that name, as a quoted string or a names.Attr constant. It's evidence that the
// argument is implemented; a name found for another field gives a false match, so a match never hides work by itself.
//
// A member of a nested structure counts only inside its parent's block: the value of a schema map entry keyed by a
// Terraform name for a member that targets the structure, or the model struct a field tagged with that name refers
// to. Common nested names such as tier_config are often on another block of the same file. Blocks and the names in
// them come from the parsed Go source, so comments and string contents don't count. Providers flatten two kinds of
// shape into the block above, so when the file has no block for the parent, the check moves up a level: from a
// union, whose members become blocks of its parent, and from a structure that an operation's input wraps (its only
// structure or union member), whose members become top-level arguments and count anywhere in the file. Otherwise a
// name used elsewhere in the file doesn't count. With no model, the whole file counts.
func (e *Enricher) findMembers(ws []Work, m *smithy.Model) {
	attrs := attrConstants(e.Checkout)
	type file struct {
		src  string
		ast  *ast.File
		fset *token.FileSet
	}
	files := map[string]*file{}
	read := func(f string) *file {
		if x, ok := files[f]; ok {
			return x
		}
		b, _ := os.ReadFile(filepath.Join(e.Checkout, f))
		x := &file{src: string(b), fset: token.NewFileSet()}
		// A file that doesn't parse has no blocks, so no nested member is found in it.
		x.ast, _ = parser.ParseFile(x.fset, f, b, parser.SkipObjectResolution)
		files[f] = x
		return x
	}
	inputs := operationInputs(m)
	for i := range ws {
		w := &ws[i]
		if w.Kind != WorkNewArgument || len(w.UsedBy) == 0 || !writes(w.Operations) {
			continue
		}
		shape, member, _ := strings.Cut(w.Subject, ".")
		names := MemberNames(member)
		w.Member = names[0]
		for _, f := range w.UsedBy {
			for _, t := range registeredTypes(read(f).src) {
				if !slices.Contains(w.Resources, t) {
					w.Resources = append(w.Resources, t)
				}
			}
		}
		slices.Sort(w.Resources)
		nested := m != nil && !inputs[shape]
		for _, f := range w.UsedBy {
			x := read(f)
			if !slices.ContainsFunc(names, func(n string) bool {
				return strings.Contains(x.src, `"`+n+`"`) || (attrs[n] != "" && hasIdent(x.src, "names."+attrs[n]))
			}) {
				continue
			}
			if !nested {
				w.FoundIn = append(w.FoundIn, f)
				continue
			}
			found, scope := inBlock(x.ast, m, shape, inputs, func(bs []ast.Node) bool {
				return slices.ContainsFunc(bs, func(b ast.Node) bool { return namesIn(b, names, attrs) })
			}, attrs)
			for _, n := range scope {
				if !slices.Contains(w.Parent, n) {
					w.Parent = append(w.Parent, n)
				}
			}
			if found {
				w.FoundIn = append(w.FoundIn, f)
			} else {
				w.OutsideBlock = append(w.OutsideBlock, f)
			}
		}
		slices.Sort(w.Parent)
	}
}

// maxClimb is how many levels above a member's shape inBlock looks for a block in the file.
const maxClimb = 4

// inBlock reports whether found matches the blocks of shape's parent in f, and the Terraform names of the blocks it
// checked, or none when the member was flattened into top-level arguments. When f has no block for a parent, it moves
// up a level only through flattened shapes; see findMembers.
func inBlock(f *ast.File, m *smithy.Model, shape string, inputs map[string]bool, found func([]ast.Node) bool, attrs map[string]string) (bool, []string) {
	level := []string{shape}
	var first []string
	for range maxClimb {
		up, names := parents(m, level)
		if first == nil {
			first = names
		}
		if bs := blocks(f, names, attrs); len(bs) > 0 {
			return found(bs), names
		}
		if slices.ContainsFunc(up, func(s string) bool { return inputs[s] && wraps(m, s, level) }) {
			return true, nil
		}
		if len(up) == 0 || !slices.ContainsFunc(level, func(s string) bool { return m.Shapes[s].Type == smithy.TypeUnion }) {
			break
		}
		level = up
	}
	// No block found at any level: report the immediate parent's names.
	return false, first
}

// wraps reports whether the only structure or union member of input targets one of shapes, as
// CreatePrivateVirtualInterfaceInput wraps NewPrivateVirtualInterface.
func wraps(m *smithy.Model, input string, shapes []string) bool {
	var nested []string
	for _, target := range m.Shapes[input].Members {
		if t := m.Shapes[target].Type; t == smithy.TypeStructure || t == smithy.TypeUnion {
			nested = append(nested, target)
		}
	}
	return len(nested) == 1 && slices.Contains(shapes, nested[0])
}

// operationInputs returns the model's operation input shapes. Their members are top-level arguments.
func operationInputs(m *smithy.Model) map[string]bool {
	out := map[string]bool{}
	if m == nil {
		return out
	}
	for _, op := range m.Operations() {
		if in := m.Shapes[op].Input; in != "" {
			out[in] = true
		}
	}
	return out
}

// parents returns the structures and unions with a member that targets one of shapes, directly or as the element of
// a list or map, and those members' likely Terraform names, both sorted.
func parents(m *smithy.Model, shapes []string) (up, names []string) {
	for name, sh := range m.Shapes {
		if sh.Type != smithy.TypeStructure && sh.Type != smithy.TypeUnion {
			continue
		}
		for member, target := range sh.Members {
			if !slices.Contains(shapes, target) && !slices.ContainsFunc(m.Shapes[target].Elems, func(e string) bool { return slices.Contains(shapes, e) }) {
				continue
			}
			if !slices.Contains(up, name) {
				up = append(up, name)
			}
			for _, n := range MemberNames(member) {
				if !slices.Contains(names, n) {
					names = append(names, n)
				}
			}
		}
	}
	slices.Sort(up)
	slices.Sort(names)
	return up, names
}

// blocks returns the parts of f that hold a block named by one of names: the value of each map entry with that key,
// as a string literal or a names.Attr constant, and the struct type each field tagged tfsdk:"name" refers to.
func blocks(f *ast.File, names []string, attrs map[string]string) []ast.Node {
	if f == nil || len(names) == 0 {
		return nil
	}
	structs := map[string]*ast.StructType{}
	ast.Inspect(f, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structs[ts.Name.Name] = st
			}
		}
		return true
	})
	var out []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.KeyValueExpr:
			if slices.ContainsFunc(names, func(name string) bool { return isName(n.Key, name, attrs) }) {
				out = append(out, n.Value)
				if st := structs[typeName(n.Value)]; st != nil {
					out = append(out, st)
				}
			}
		case *ast.Field:
			if slices.Contains(names, tfsdkTag(n)) {
				if st := structs[typeName(n.Type)]; st != nil {
					out = append(out, st)
				}
			}
		}
		return true
	})
	return out
}

// namesIn reports whether node uses one of names: as a string literal, a names.Attr constant, or a tfsdk tag.
func namesIn(node ast.Node, names []string, attrs map[string]string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.Field:
			if slices.Contains(names, tfsdkTag(n)) {
				found = true
			}
		case ast.Expr:
			if slices.ContainsFunc(names, func(name string) bool { return isName(n, name, attrs) }) {
				found = true
			}
		}
		return !found
	})
	return found
}

// isName reports whether e is the string literal name or the names.Attr constant for it.
func isName(e ast.Expr, name string, attrs map[string]string) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		v, err := strconv.Unquote(e.Value)
		return e.Kind == token.STRING && err == nil && v == name
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		return ok && x.Name == "names" && attrs[name] != "" && e.Sel.Name == attrs[name]
	}
	return false
}

// tfsdkTag returns the tfsdk struct tag of a field, or "".
func tfsdkTag(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return ""
	}
	return reflect.StructTag(tag).Get("tfsdk")
}

// typeName returns the type a field or schema value refers to: T in T, *T, ListNestedObjectValueOf[T], or
// SingleNestedChildObjectAttribute[T](ctx), or "".
func typeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return typeName(e.X)
	case *ast.IndexExpr:
		return typeName(e.Index)
	case *ast.IndexListExpr:
		if len(e.Indices) > 0 {
			return typeName(e.Indices[0])
		}
	case *ast.CallExpr:
		return typeName(e.Fun)
	case *ast.SelectorExpr:
		// A generic from another package (fwtypes.ListNestedObjectValueOf) is unwrapped by IndexExpr above.
		return ""
	}
	return ""
}

var (
	// notSettableRE finds documentation saying a member can't be set: "This field is read-only", "Read-only.", "ignored
	// on write", "isn't accepted in CreateTrainingJob requests", "output-only". Looser wording ("cannot be changed",
	// "the only valid value is") mostly describes real arguments, so it isn't matched.
	notSettableRE = regexp.MustCompile(`(?i)\b(?:this|the)\s+(?:field|property|parameter|value|member|attribute)\s+is\s+read[- ]only\b|^(?:the\s+|a\s+)?read[- ]only\b|\bignored\s+(?:on|when|in|for|during)\s+(?:write|input|requests?|calling|\w*(?:Create|Update|Put)\w*)|\b(?:isn't|is\s+not)\s+accepted\s+in\b|\boutput[- ]only\b`)
	tagRE         = regexp.MustCompile(`<[^>]*>`)
	spaceRE       = regexp.MustCompile(`\s+`)
)

// markNotSettable turns new arguments whose documentation says they can't be set into attributes, quoting the
// documentation in the basis. Athena's WorkGroupConfiguration.SystemTableStatuses is in the create and update input,
// but "is ignored on write".
func markNotSettable(ws []Work, m *smithy.Model) {
	if m == nil {
		return
	}
	for i := range ws {
		w := &ws[i]
		if w.Kind != WorkNewArgument {
			continue
		}
		shape, member, ok := strings.Cut(w.Subject, ".")
		if !ok {
			continue
		}
		doc := strings.TrimSpace(spaceRE.ReplaceAllString(tagRE.ReplaceAllString(m.Shapes[shape].Docs[member], " "), " "))
		loc := notSettableRE.FindStringIndex(doc)
		if loc == nil {
			continue
		}
		w.Kind = WorkNewAttribute
		w.Basis = fmt.Sprintf("%s; documented as not settable (%q)", w.Basis, doc[loc[0]:loc[1]])
	}
}

// existingMembers returns the member names of the model's structures and unions that the change didn't add.
func existingMembers(d *smithy.Diff, m *smithy.Model) map[string]bool {
	added := map[[2]string]bool{}
	for _, mc := range d.AddedMembers {
		added[[2]string{mc.Shape, mc.Member}] = true
	}
	newShape := map[string]bool{}
	for _, sh := range d.AddedShapes {
		newShape[sh.Name] = true
	}
	existing := map[string]bool{}
	for name, sh := range m.Shapes {
		if newShape[name] || (sh.Type != smithy.TypeStructure && sh.Type != smithy.TypeUnion) {
			continue
		}
		for member := range sh.Members {
			if !added[[2]string{name, member}] {
				existing[member] = true
			}
		}
	}
	return existing
}

// markSharedNames sets SharedName on new members whose name another shape's existing member already has.
func markSharedNames(ws []Work, d *smithy.Diff, m *smithy.Model) {
	if d == nil || m == nil {
		return
	}
	existing := existingMembers(d, m)
	for i := range ws {
		if ws[i].Kind != WorkNewArgument && ws[i].Kind != WorkNewAttribute {
			continue
		}
		if _, member, ok := strings.Cut(ws[i].Subject, "."); ok && existing[member] {
			ws[i].SharedName = true
		}
	}
}

// newMembers lists the member names a change introduces to existing shapes: new members from ws, and the members of
// a new structure or union that a new member targets, as AlarmTagsPropagation in a new ActionConfiguration. Names an
// existing shape already has are left out. Members of other new shapes belong to new operations, which their
// operation names identify.
func newMembers(ws []Work, d *smithy.Diff, m *smithy.Model) []string {
	var out []string
	add := func(name string) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, w := range ws {
		if (w.Kind == WorkNewArgument || w.Kind == WorkNewAttribute) && !w.SharedName {
			if _, member, ok := strings.Cut(w.Subject, "."); ok {
				add(member)
			}
		}
	}
	if d == nil || m == nil {
		return out
	}
	newShape := map[string]bool{}
	for _, sh := range d.AddedShapes {
		newShape[sh.Name] = true
	}
	existing := existingMembers(d, m)
	for _, w := range ws {
		// A shared name is left out above, but its new target's members can still be new.
		if w.Kind != WorkNewArgument && w.Kind != WorkNewAttribute {
			continue
		}
		shape, member, ok := strings.Cut(w.Subject, ".")
		if !ok {
			continue
		}
		target := m.Shapes[shape].Members[member]
		sh, ok := m.Shapes[target]
		if !ok || !newShape[target] || (sh.Type != smithy.TypeStructure && sh.Type != smithy.TypeUnion) {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(sh.Members)) {
			if !existing[name] {
				add(name)
			}
		}
	}
	return out
}

// hasIdent reports whether src contains id not followed by another identifier character, so names.AttrName doesn't
// match names.AttrNamePrefix.
func hasIdent(src, id string) bool {
	for i := 0; ; {
		j := strings.Index(src[i:], id)
		if j < 0 {
			return false
		}
		end := i + j + len(id)
		if end == len(src) || !(unicode.IsLetter(rune(src[end])) || unicode.IsDigit(rune(src[end])) || src[end] == '_') {
			return true
		}
		i = end
	}
}

var (
	attrMu    sync.Mutex
	attrCache = map[string]map[string]string{}
	attrRE    = regexp.MustCompile(`(?m)^\s*(Attr\w+)\s*=\s*"([a-z0-9_]+)"`)
)

// attrConstants maps Terraform attribute names to the provider's names.Attr constants, from the checkout's
// names/attr_consts_gen.go. Common names like "kms_key_arn" are written as names.AttrKMSKeyARN, not as strings.
func attrConstants(checkout string) map[string]string {
	attrMu.Lock()
	defer attrMu.Unlock()
	if m, ok := attrCache[checkout]; ok {
		return m
	}
	m := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(checkout, "names", "attr_consts_gen.go")); err == nil {
		for _, sm := range attrRE.FindAllStringSubmatch(string(b), -1) {
			m[sm[2]] = sm[1]
		}
	}
	attrCache[checkout] = m
	return m
}

// typeAnnotationRE matches the annotations that register a Terraform resource or data source type in a provider file.
var typeAnnotationRE = regexp.MustCompile(`(?m)^// @(?:SDK|Framework)(?:Resource|DataSource|EphemeralResource)\("([a-z0-9_]+)"`)

// registeredTypes returns the Terraform types src registers, in order.
func registeredTypes(src string) []string {
	var out []string
	for _, m := range typeAnnotationRE.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}
