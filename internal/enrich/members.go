package enrich

import (
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
