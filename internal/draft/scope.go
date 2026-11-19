package draft

import (
	"fmt"
	"slices"
	"strings"

	"github.com/YakDriver/avizor/internal/enrich"
)

// Resource is a new resource or data source in a draft's scope.
type Resource struct {
	// Kind is "resource" or "data source".
	Kind string
	Name string
	// Operations are the operations in lifecycle order, in backticks: "`CreateX`, `GetX`".
	Operations string
	// Decide is a judgment call about the whole resource, as a marker, or "".
	Decide string
	// Lines are the arguments and attributes, as Markdown list items.
	Lines []string
	Notes []string
}

// decider records a judgment call and returns its marker for the body.
type decider func(where, choice string) string

func (d *Draft) decider() decider {
	return func(where, choice string) string {
		d.Decide = append(d.Decide, where+": "+choice)
		return " <!-- decide: " + choice + " -->"
	}
}

// Markers for judgment calls.
const (
	decideSetOrBlock = "set of %ss or nested block"
	decideComputed   = "optional, or optional and computed"
)

// resources renders the new resources and data sources.
func resources(en *enrich.Result, pkg, prefix string, decide decider) []Resource {
	var out []Resource
	for _, w := range en.Work {
		if w.Kind != enrich.WorkNewResource && w.Kind != enrich.WorkNewDataSource {
			continue
		}
		s := w.Spec
		if s == nil {
			// Without the model, only the operations are known.
			s = &enrich.Spec{Operations: enrich.SpecOperations{Other: w.Operations}}
		}
		r := Resource{Kind: "resource", Name: prefix + pkg + "_" + enrich.Snake(w.Subject)}
		if w.Kind == enrich.WorkNewDataSource {
			r.Kind = "data source"
			r.Decide = decide(r.Name, "data source, part of another resource, or none")
		}
		var ops []string
		for _, op := range slices.Concat(s.Operations.Create, s.Operations.Read, s.Operations.Update, s.Operations.Delete, s.Operations.List) {
			ops = append(ops, "`"+op+"`")
		}
		r.Operations = strings.Join(ops, ", ")

		// A data source's arguments are its read input, required or optional too.
		argument := true
		var args []enrich.Field
		tags := false
		for _, f := range s.Arguments {
			if s.Tagging != nil && s.Tagging.Member != "" && f.Member == s.Tagging.Member {
				tags = true
				continue
			}
			args = append(args, f)
		}
		if argument && s.Tagging != nil && s.Tagging.Kind == "tag_resource" {
			tags = true
		}
		if len(args) > 0 || tags {
			r.Lines = append(r.Lines, "- Arguments:")
			r.Lines = append(r.Lines, fieldLines(args, 1, argument, r.Name, w.Subject, decide)...)
			if tags {
				r.Lines = append(r.Lines, "  - `tags`")
			}
		}
		if len(s.Attributes) > 0 {
			var names, abbrev []string
			for _, f := range s.Attributes {
				names = append(names, "`"+f.Name+"`")
				if abbreviates(f.Name, w.Subject) {
					abbrev = append(abbrev, "`"+f.Member+"`")
				}
			}
			line := "- Attributes: " + strings.Join(names, ", ") + "."
			if len(abbrev) > 0 {
				line += decide(r.Name, "Terraform names for "+strings.Join(abbrev, ", ")+", which abbreviate the resource's name")
			}
			r.Lines = append(r.Lines, line)
		}
		r.Notes = resourceNotes(s, r.Name, decide)
		out = append(out, r)
	}
	return out
}

// fieldLines renders fields as Markdown list items at a depth, with their nested fields below them.
func fieldLines(fs []enrich.Field, depth int, argument bool, where, subject string, decide decider) []string {
	var out []string
	indent := strings.Repeat("  ", depth)
	for _, f := range fs {
		line := fmt.Sprintf("%s- `%s` (%s)", indent, f.Name, fieldText(f, argument))
		line += fieldDecisions(f, argument, where, subject, decide)
		out = append(out, line)
		out = append(out, fieldLines(f.Fields, depth+1, argument, where, subject, decide)...)
	}
	return out
}

func fieldText(f enrich.Field, argument bool) string {
	return strings.Replace(enrich.FieldText(f, argument), "create-only", "forces replacement", 1)
}

// fieldDecisions returns the markers for a field's judgment calls.
func fieldDecisions(f enrich.Field, argument bool, where, subject string, decide decider) string {
	var out string
	if single := singleScalar(f); single != "" {
		out += decide(where+" "+f.Name, fmt.Sprintf(decideSetOrBlock, single))
	}
	if argument && !f.Required && f.Returned && (f.Default != "" || collection(f)) {
		out += decide(where+" "+f.Name, decideComputed)
	}
	if subject != "" && abbreviates(f.Name, subject) {
		out += decide(where+" "+f.Name, "Terraform name for `"+f.Member+"`, which abbreviates the resource's name")
	}
	return out
}

// singleScalar returns the type of the one field of a list or set of structures whose element has only that field, a
// scalar: such a list is simpler as a set of the scalar.
func singleScalar(f enrich.Field) string {
	if (f.Type != "list" && f.Type != "set") || f.Elem != "structure" || len(f.Fields) != 1 {
		return ""
	}
	switch t := f.Fields[0].Type; t {
	case "string", "enum", "integer", "long", "boolean":
		if t == "enum" {
			return "string"
		}
		return t
	}
	return ""
}

func collection(f enrich.Field) bool {
	return f.Type == "list" || f.Type == "set" || f.Type == "map"
}

// abbreviates reports whether a field's name, without a trailing "_id" or "_arn", is made of the subject's words and at
// least one abbreviation of consecutive subject words: "fca_policy_id" for NetworkingFlexCostAllocationPolicy.
func abbreviates(name, subject string) bool {
	words := strings.Split(enrich.Snake(subject), "_")
	toks := strings.Split(name, "_")
	if len(toks) > 1 && slices.Contains([]string{"id", "ids", "arn", "arns"}, toks[len(toks)-1]) {
		toks = toks[:len(toks)-1]
	}
	abbrev := false
	for _, t := range toks {
		switch {
		case slices.Contains(words, t):
		case len(t) >= 2 && initials(words, t):
			abbrev = true
		default:
			return false
		}
	}
	return abbrev
}

// initials reports whether t is the initials of consecutive words.
func initials(words []string, t string) bool {
	for i := 0; i+len(t) <= len(words); i++ {
		ok := true
		for j := range len(t) {
			if words[i+j] == "" || words[i+j][0] != t[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// resourceNotes states the model's lifecycle facts: waiting, update, delete, tagging, quoted documentation, and other
// operations.
func resourceNotes(s *enrich.Spec, name string, decide decider) []string {
	var out []string
	q := func(vs []string) string {
		var qs []string
		for _, v := range vs {
			qs = append(qs, "`"+v+"`")
		}
		return strings.Join(qs, ", ")
	}
	if st := s.Status; st != nil && len(st.Pending) > 0 {
		n := fmt.Sprintf("`%s` has pending values %s", st.Member, q(st.Pending))
		if len(st.Target) > 0 {
			n += " and target " + q(st.Target)
		}
		if len(st.Failed) > 0 {
			n += "; failed " + q(st.Failed)
		}
		if st.Reason != "" {
			n += ", with the reason in `" + st.Reason + "`"
		}
		out = append(out, n+". Wait for the target value.")
	}
	if u := s.Update; u != nil {
		switch {
		case u.Kind == "none":
			out = append(out, "No update operation, so every argument forces replacement.")
		case u.Kind == "put":
			out = append(out, "The create operation is a Put, which also updates.")
		case u.Kind == "add_remove":
			out = append(out, "Updates add and remove "+q(u.Members)+".")
		case len(u.Replaces) > 0:
			out = append(out, "Updates replace the current values, as documented for "+q(u.Replaces)+".")
		}
	}
	if d := s.Delete; d != nil {
		switch {
		case d.Kind == "none":
			out = append(out, "No delete operation."+decide(name, "resource without a delete, or none"))
		case d.Plural && d.Max > 0:
			out = append(out, fmt.Sprintf("Delete is `%s`, which takes up to %d IDs in `%s`.", d.Operation, d.Max, d.Member))
		case d.Plural:
			out = append(out, fmt.Sprintf("Delete is `%s`, which takes a list in `%s`.", d.Operation, d.Member))
		}
	}
	if t := s.Tagging; t != nil {
		switch {
		case t.Kind == "tag_specifications" && len(t.ResourceTypes) > 0:
			out = append(out, "Tags are set with `TagSpecifications` (new resource type "+q(t.ResourceTypes)+").")
		case t.Member != "":
			out = append(out, "Tags are set on create with `"+t.Member+"`.")
		case t.Kind == "tag_resource":
			out = append(out, "Tags are set with `TagResource`.")
		}
	}
	for _, n := range s.Notes {
		out = append(out, fmt.Sprintf("`%s`: %q", n.Source, n.Text))
	}
	if len(s.Operations.Other) > 0 && s.Operations.Create == nil && s.Operations.Read == nil && s.Operations.List == nil {
		// No spec: the operations weren't sorted by role.
		return []string{"Operations: " + q(s.Operations.Other) + "."}
	}
	if len(s.Operations.Other) > 0 {
		choice := "resources, arguments, or none"
		if slices.ContainsFunc(s.Operations.Other, func(op string) bool {
			return slices.ContainsFunc([]string{"Associate", "Attach", "Register", "Enable"}, func(v string) bool { return strings.HasPrefix(op, v) })
		}) {
			choice = "one resource per association, an argument, or none"
		}
		out = append(out, "Other operations: "+q(s.Operations.Other)+"."+decide(name, choice))
	}
	return out
}

// change is a new member of existing resources: the arguments and attributes that describe one Terraform field, the
// create, update, and read members of one noun's shapes.
type change struct {
	name string
	// paths are the parent blocks' paths from the schema; with several, the block is a judgment call.
	paths    []string
	subjects []string
	// returned are the attribute subjects of an argument, and the operations whose output reaches its shape.
	returned  []string
	works     []enrich.Work
	field     *enrich.Field
	fieldFrom string
	argument  bool
	attribute bool
	// returns is set when read returns an argument: a matching attribute, or an output reaching its shape.
	returns bool
}

// shapeVerbs start operation input and output shape names.
var shapeVerbs = []string{"Create", "Update", "Modify", "Put", "Get", "Describe", "List", "Delete", "Register", "Start", "Set", "Associate"}

// shapeNoun is the noun of an operation's input or output shape (CreateClusterInput → Cluster), or the shape.
func shapeNoun(shape string) string {
	n := shape
	for _, suffix := range []string{"Input", "Output", "Request", "Response", "Result"} {
		if rest, ok := strings.CutSuffix(n, suffix); ok && rest != "" {
			n = rest
			break
		}
	}
	if n == shape {
		return shape
	}
	for _, v := range shapeVerbs {
		if rest, ok := strings.CutPrefix(n, v); ok && rest != "" {
			return rest
		}
	}
	return n
}

// createInput reports whether a subject is a create or put input, whose requiredness is the argument's.
func createInput(subject string) bool {
	return strings.HasPrefix(subject, "Create") || strings.HasPrefix(subject, "Put")
}

// changes renders new arguments, attributes, enum values, and other work on existing resources, with notes. It also
// returns the resources the arguments and attributes are in, and the names for a title: the arguments' in a known
// resource, or with none, the attributes'.
//
// When every line is about one resource and there's no new resource or data source, the title names the resource, so
// lines leave out its name. Lines leave out the API fields
// when the release notes name all of their operations.
func changes(en *enrich.Result, checkout string, releaseNotes []string, decide decider) (lines, notes, targets []string, names []string) {
	var attrNames []string
	defer func() {
		if len(names) == 0 {
			names = attrNames
		}
	}()
	var cs []*change
	byKey := map[string]*change{}
	for _, w := range en.Work {
		if w.Kind != enrich.WorkNewArgument && w.Kind != enrich.WorkNewAttribute {
			continue
		}
		shape, member, _ := strings.Cut(w.Subject, ".")
		name := cmp(w.Member, enrich.Snake(member))
		var paths []string
		for _, p := range w.Schema {
			if !slices.Contains(paths, p.Path) {
				paths = append(paths, p.Path)
			}
		}
		slices.Sort(paths)
		key := shapeNoun(shape) + "|" + strings.Join(paths, ",") + "|" + name
		c := byKey[key]
		if c == nil {
			c = &change{name: name, paths: paths}
			byKey[key] = c
			cs = append(cs, c)
		}
		c.works = append(c.works, w)
		if w.Kind == enrich.WorkNewArgument {
			// The argument's field is the create input's, then any input's, never an output's.
			if w.Field != nil && (!c.argument || c.field == nil || createInput(w.Subject) && !createInput(c.fieldFrom)) {
				c.field, c.fieldFrom = w.Field, w.Subject
			}
			c.argument = true
			if len(w.Returns) > 0 {
				c.returns = true
			}
		} else {
			c.attribute = true
			if c.field == nil {
				c.field, c.fieldFrom = w.Field, w.Subject
			}
		}
	}

	for _, c := range cs {
		var outputs []string
		for _, w := range c.works {
			if !c.argument || w.Kind == enrich.WorkNewArgument {
				c.subjects = append(c.subjects, "`"+w.Subject+"`")
				for _, op := range w.Returns {
					if !slices.Contains(outputs, "`"+op+"`") {
						outputs = append(outputs, "`"+op+"`")
					}
				}
			} else {
				c.returned = append(c.returned, "`"+w.Subject+"`")
			}
		}
		c.returned = append(c.returned, outputs...)
		c.returns = c.returns || c.argument && c.attribute
	}

	seenPlace := map[string]bool{}
	var headed []int // lines that start with their one resource
	for _, c := range cs {
		where := c.name
		if len(c.paths) == 1 && c.paths[0] != "" {
			where = c.paths[0] + "." + c.name
		}
		// An argument is in the resources that write it; the outputs that return it may be other resources'.
		rs := changeResources(slices.DeleteFunc(slices.Clone(c.works), func(w enrich.Work) bool {
			return c.argument && w.Kind != enrich.WorkNewArgument
		}), checkout)
		var head string
		switch len(rs) {
		case 0:
			head = "`" + where + "`"
		default:
			head = "`" + strings.Join(rs, "`, `") + "`"
		}
		kind := "attribute"
		if c.argument {
			kind = "argument"
		}
		line := fmt.Sprintf("- %s: new %s `%s`", head, kind, where)
		if len(rs) == 1 {
			headed = append(headed, len(lines))
		}
		if c.field != nil {
			line += " (" + fieldText(*c.field, c.argument) + ")"
		}
		if !namedOperations(releaseNotes, c.works) {
			line += ", from " + joinAnd(c.subjects)
			if len(c.returned) > 0 {
				line += "; returned by " + joinAnd(c.returned)
			}
		}
		line += "."
		switch {
		case len(rs) == 0:
			line += decide(where, "which resource")
		case len(rs) > 1:
			line += decide(where, "which of these resources")
		}
		if len(c.paths) > 1 {
			var ps []string
			for _, p := range c.paths {
				ps = append(ps, "`"+p+"."+c.name+"`")
			}
			line += decide(where, "which block: "+joinOr(ps))
		}
		if c.field != nil {
			if single := singleScalar(*c.field); single != "" {
				line += decide(where, fmt.Sprintf(decideSetOrBlock, single))
			}
			if c.argument && c.returns && !c.field.Required && (c.field.Default != "" || collection(*c.field)) {
				line += decide(where, decideComputed)
			}
		}
		lines = append(lines, line)
		if c.field != nil {
			lines = append(lines, fieldLines(c.field.Fields, 1, c.argument, where, "", decide)...)
		}
		for _, r := range rs {
			if !slices.Contains(targets, r) {
				targets = append(targets, r)
			}
		}
		switch {
		case len(rs) == 0:
		case c.argument:
			names = append(names, where)
		default:
			attrNames = append(attrNames, where)
		}

		for _, w := range c.works {
			for _, p := range w.Schema {
				k := fmt.Sprintf("%s:%d", p.File, p.Line)
				if seenPlace[k] {
					continue
				}
				seenPlace[k] = true
				n := fmt.Sprintf("`%s` is a %s in `%s:%d` (%s)", p.Path, p.Kind, p.File, p.Line, p.Framework)
				switch {
				case p.ForceNew != "":
					n += "; `" + p.ForceNew + "` forces replacement"
				case p.ForceNewIf != "":
					n += "; `" + p.ForceNewIf + "` forces replacement on a condition"
				}
				notes = append(notes, n+".")
			}
			if w.SharedName && w.Kind == enrich.WorkNewArgument {
				notes = append(notes, fmt.Sprintf("Another shape already had `%s`, as when an update operation gains a member create has, so the argument may exist.", w.Subject[strings.Index(w.Subject, ".")+1:]))
			}
		}
	}

	newType := slices.ContainsFunc(en.Work, func(w enrich.Work) bool {
		return w.Kind == enrich.WorkNewResource || w.Kind == enrich.WorkNewDataSource
	})
	if len(targets) == 1 && len(headed) == len(cs) && !newType {
		for _, i := range headed {
			rest := strings.TrimPrefix(lines[i], "- `"+targets[0]+"`: new ")
			lines[i] = "- New " + rest
		}
	}

	for _, w := range en.Work {
		switch w.Kind {
		case enrich.WorkNewEnumValue:
			values := "values"
			if len(w.Values) == 1 {
				values = "value"
			}
			line := fmt.Sprintf("- `%s`: new %s %s.", w.Subject, values, joinAnd(quoted(w.Values)))
			if w.EnumSearched {
				line += " " + enumFacts(w.EnumUses)
			}
			lines = append(lines, line)
		case enrich.WorkNewOperation:
			ops := "operation"
			if len(w.Operations) > 1 {
				ops = "operations"
			}
			lines = append(lines, "- New "+ops+" "+joinAnd(quoted(w.Operations))+"."+decide(w.Subject, "resource, argument, or none"))
		case enrich.WorkTagging:
			if !slices.ContainsFunc(en.Work, func(w enrich.Work) bool { return w.Kind == enrich.WorkNewResource }) {
				lines = append(lines, "- New tagging operations "+joinAnd(quoted(w.Operations))+".")
			}
		case enrich.WorkTypeChange, enrich.WorkRemoval:
			lines = append(lines, "- `"+w.Subject+"`: "+w.Basis+".")
		}
	}
	return lines, notes, targets, names
}

// namedOperations reports whether the release notes name, as whole words, every operation the works' members come
// from or are returned by: "CreateCluster" for "CreateClusterInput.metricGroups" when CreateCluster is one of the work's
// operations. A member of a shape that isn't an operation's own input or output, such as a nested structure, isn't named.
func namedOperations(releaseNotes []string, works []enrich.Work) bool {
	text := strings.Join(releaseNotes, "\n")
	for _, w := range works {
		shape, _, _ := strings.Cut(w.Subject, ".")
		op := slices.IndexFunc(w.Operations, func(o string) bool {
			return slices.ContainsFunc([]string{"Input", "Output", "Request", "Response"}, func(suffix string) bool { return shape == o+suffix })
		})
		if op < 0 || !wordIn(text, w.Operations[op]) {
			return false
		}
		for _, r := range w.Returns {
			if !wordIn(text, r) {
				return false
			}
		}
	}
	return len(works) > 0
}

// wordIn reports whether w is in text with no letter or digit on either side.
func wordIn(text, w string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], w)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(w)
		if (start == 0 || !alnum(text[start-1])) && (end == len(text) || !alnum(text[end])) {
			return true
		}
		i = start + 1
	}
}

func alnum(b byte) bool {
	return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9'
}

// changeResources returns the Terraform resources in the files that call all of the works' operations, or, if no file
// calls them all, in any of the files.
func changeResources(ws []enrich.Work, checkout string) []string {
	var common, all []string
	first := true
	for _, w := range ws {
		if len(w.UsedBy) == 0 {
			continue
		}
		rs := resourceNames(checkout, w.UsedBy)
		for _, r := range rs {
			if !slices.Contains(all, r) {
				all = append(all, r)
			}
		}
		if first {
			common, first = rs, false
			continue
		}
		common = slices.DeleteFunc(common, func(r string) bool { return !slices.Contains(rs, r) })
	}
	if len(common) > 0 {
		return common
	}
	slices.Sort(all)
	return all
}

func enumFacts(uses []enrich.EnumUse) string {
	if len(uses) == 0 {
		return "The provider package doesn't use the type."
	}
	var validated, used []string
	for _, u := range uses {
		ref := fmt.Sprintf("`%s:%d`", u.File, u.Line)
		if u.Validated {
			validated = append(validated, ref)
		} else {
			used = append(used, ref)
		}
	}
	var parts []string
	if len(validated) > 0 {
		parts = append(parts, "validated with the SDK type in "+joinAnd(validated))
	}
	if len(used) > 0 {
		parts = append(parts, "used in "+joinAnd(used))
	}
	return "The type is " + strings.Join(parts, " and ") + "."
}

func quoted(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = "`" + v + "`"
	}
	return out
}

// joinOr joins "a", "b", or "c".
func joinOr(ss []string) string {
	if len(ss) == 2 {
		return ss[0] + " or " + ss[1]
	}
	return strings.Replace(joinAnd(ss), ", and ", ", or ", 1)
}

// joinAnd joins "a", "b", and "c".
func joinAnd(ss []string) string {
	switch len(ss) {
	case 0:
		return ""
	case 1:
		return ss[0]
	case 2:
		return ss[0] + " and " + ss[1]
	}
	return strings.Join(ss[:len(ss)-1], ", ") + ", and " + ss[len(ss)-1]
}
