package enrich

import (
	"github.com/YakDriver/avizor/internal/smithy"
	"maps"

	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// Snake turns a PascalCase or camelCase name into snake_case: S3AccessPoint -> s3_access_point, KMSKey -> kms_key.
func Snake(s string) string {
	rs := []rune(s)
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
// A member of a nested structure counts only inside its parent's block: the schema entry or the model struct of a
// Terraform name for a member that targets the structure. Common nested names such as tier_config are often on
// another block of the same file. Providers flatten two kinds of shape into the block above, so when the file has no
// block for the parent, the check moves up a level: from a union, whose members become blocks of its parent, and from
// a structure that an operation's input wraps (its only structure or union member), whose members become top-level
// arguments and count anywhere in the file. Otherwise a name used elsewhere in the file doesn't count. With no model, the whole file counts.
func (e *Enricher) findMembers(ws []Work, m *smithy.Model) {
	attrs := attrConstants(e.Checkout)
	files := map[string]string{}
	read := func(f string) string {
		if s, ok := files[f]; ok {
			return s
		}
		b, _ := os.ReadFile(filepath.Join(e.Checkout, f))
		files[f] = string(b)
		return files[f]
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
		nested := m != nil && !inputs[shape]
		if nested {
			_, w.Parent = parents(m, []string{shape})
		}
		for _, f := range w.UsedBy {
			src := read(f)
			uses := func(s string) bool {
				return slices.ContainsFunc(names, func(n string) bool {
					return strings.Contains(s, `"`+n+`"`) || (attrs[n] != "" && hasIdent(s, "names."+attrs[n]))
				})
			}
			switch {
			case !uses(src):
			case !nested || inBlock(src, m, shape, inputs, attrs, uses):
				w.FoundIn = append(w.FoundIn, f)
			default:
				w.OutsideBlock = append(w.OutsideBlock, f)
			}
		}
	}
}

// maxClimb is how many levels above a member's shape inBlock looks for a block in the file.
const maxClimb = 4

// inBlock reports whether uses matches inside a block of shape's parent in src. When src has no block for a parent,
// it moves up a level only through flattened shapes; see findMembers.
func inBlock(src string, m *smithy.Model, shape string, inputs map[string]bool, attrs map[string]string, uses func(string) bool) bool {
	level := []string{shape}
	for range maxClimb {
		up, names := parents(m, level)
		if bs := blocks(src, names, attrs); len(bs) > 0 {
			return slices.ContainsFunc(bs, uses)
		}
		if slices.ContainsFunc(up, func(s string) bool { return inputs[s] && wraps(m, s, level) }) {
			return true
		}
		if len(up) == 0 || !slices.ContainsFunc(level, func(s string) bool { return m.Shapes[s].Type == smithy.TypeUnion }) {
			return false
		}
		level = up
	}
	return false
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

var (
	// genericRE finds the type argument of a generic type, as in ListNestedObjectValueOf[fooModel].
	genericRE = regexp.MustCompile(`\[\s*\*?([A-Za-z_]\w*)\s*\]`)
	identRE   = regexp.MustCompile(`^\s*\*?([A-Za-z_]\w*)`)
)

// blocks returns the parts of src that hold a block named by one of names: the value of each schema map entry with
// that key, and the struct each model field tagged tfsdk:"name" refers to.
func blocks(src string, names []string, attrs map[string]string) []string {
	var out []string
	structOf := func(typ string) {
		if t := genericRE.FindStringSubmatch(typ); t != nil {
			typ = t[1]
		} else if t := identRE.FindStringSubmatch(typ); t != nil {
			typ = t[1]
		}
		if body, ok := structBody(src, typ); ok {
			out = append(out, body)
		}
	}
	for _, n := range names {
		keys := []string{`"` + n + `"`}
		if attrs[n] != "" {
			keys = append(keys, "names."+attrs[n])
		}
		for _, key := range keys {
			for i := 0; ; {
				j := strings.Index(src[i:], key)
				if j < 0 {
					break
				}
				start := i + j + len(key)
				i = start
				if key[0] != '"' && start < len(src) && isIdentByte(src[start]) {
					continue
				}
				rest := strings.TrimLeft(src[start:], " \t")
				if !strings.HasPrefix(rest, ":") {
					continue
				}
				value := rest[1:]
				if v, ok := entryValue(value); ok {
					out = append(out, v)
					structOf(v)
				}
			}
		}
		tag := `tfsdk:"` + n + `"`
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, tag) {
				continue
			}
			// A model field: Name Type `tfsdk:"name"`.
			if f := strings.Fields(strings.TrimSpace(line[:strings.Index(line, "`")])); len(f) == 2 {
				structOf(f[1])
			}
		}
	}
	return out
}

// entryValue returns a map entry's value from just after its colon: up to the comma or closing brace that ends it,
// with nested braces, brackets, parentheses, strings, and comments skipped.
func entryValue(s string) (string, bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '`', '\'':
			end := skipQuoted(s, i)
			if end < 0 {
				return "", false
			}
			i = end
		case '/':
			if i+1 < len(s) && (s[i+1] == '/' || s[i+1] == '*') {
				i = skipComment(s, i)
			}
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth == 0 {
				return s[:i], true
			}
			depth--
		case ',':
			if depth == 0 {
				return s[:i], true
			}
		}
	}
	return "", false
}

// structBody returns the body of "type name struct { ... }" in src.
func structBody(src, name string) (string, bool) {
	loc := regexp.MustCompile(`(?m)^\s*(?:type\s+)?` + regexp.QuoteMeta(name) + `\s+struct\s*\{`).FindStringIndex(src)
	if loc == nil {
		return "", false
	}
	v, ok := entryValue(src[loc[1]:])
	return v, ok
}

// skipQuoted returns the index of the quote closing the string or rune literal that starts at i, or -1.
func skipQuoted(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\' && q != '`':
			j++
		case s[j] == q:
			return j
		}
	}
	return -1
}

// skipComment returns the index of the last byte of the comment that starts at i.
func skipComment(s string, i int) int {
	if s[i+1] == '/' {
		if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
			return i + j
		}
		return len(s) - 1
	}
	if j := strings.Index(s[i+2:], "*/"); j >= 0 {
		return i + 2 + j + 1
	}
	return len(s) - 1
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
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
