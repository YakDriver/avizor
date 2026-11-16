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
func (e *Enricher) findMembers(ws []Work) {
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
	for i := range ws {
		w := &ws[i]
		if w.Kind != WorkNewArgument || len(w.UsedBy) == 0 || !writes(w.Operations) {
			continue
		}
		_, member, _ := strings.Cut(w.Subject, ".")
		names := MemberNames(member)
		w.Member = names[0]
		for _, f := range w.UsedBy {
			src := read(f)
			if slices.ContainsFunc(names, func(n string) bool {
				return strings.Contains(src, `"`+n+`"`) || (attrs[n] != "" && hasIdent(src, "names."+attrs[n]))
			}) {
				w.FoundIn = append(w.FoundIn, f)
			}
		}
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
		if (w.Kind != WorkNewArgument && w.Kind != WorkNewAttribute) || w.SharedName {
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
