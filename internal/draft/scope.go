package draft

import (
	gocmp "cmp"
	"fmt"
	"maps"
	"path"
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
	// NoRead is set when the model has no read operation for the resource, as for a Put-only account setting.
	NoRead bool
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
		r.NoRead = w.Spec != nil && len(s.Operations.Read) == 0 && len(s.Operations.List) == 0

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

// collection reports whether a field is a list, set, or map, but not a list of at most one, which is a single block.
func collection(f enrich.Field) bool {
	single := f.Single || f.Constraints.LengthMax != nil && *f.Constraints.LengthMax == 1
	return f.Type == "list" && !single || f.Type == "set" && !single || f.Type == "map"
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
	if s.ExistingRead != "" {
		out = append(out, "Read is `"+s.ExistingRead+"`, which the model already has.")
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
	for _, suffix := range []string{"Input", "Output", "Request", "Response", "Result", "Message"} {
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

// scopeInput is what changes renders from.
type scopeInput struct {
	en       *enrich.Result
	checkout string
	prefix   string
	// releaseNotes are the notes the body quotes; mentions is text that says what the launch is about (the notes, the
	// announcement's headline, and blog post titles), whose members are listed first.
	releaseNotes []string
	mentions     string
	// newTypes are the draft's new resources with no read operation of their own.
	newTypes []string
}

// scopeChanges are the rendered changes of existing resources.
type scopeChanges struct {
	lines, notes []string
	// targets are the resources the lines are in, and names the Terraform names for a title: the arguments' in a
	// known resource, or with none, the attributes'.
	targets, names []string
	// leftOut are members the lines leave out, with why, for the reviewer.
	leftOut []Inferred
	// typeNotes are notes for the new resources and data sources, by name.
	typeNotes map[string][]string
}

// changes renders new arguments, attributes, enum values, and other work on existing resources, with notes.
//
// When every line is about one resource and there's no new resource or data source, the title names the resource, so
// lines leave out its name. Lines leave out the API fields when the release notes name all of their operations.
//
// With the code searched in an implemented service, some members are left out: an attribute of operations no provider
// file calls, a request member of query operations only (unless a data source calls them), a Reset flag, and an
// update's Add and Remove members, which a note covers. An argument of operations no provider file calls is in a
// note naming the operations, since its resource doesn't exist yet.
func changes(in scopeInput, decide decider) scopeChanges {
	en := in.en
	out := scopeChanges{typeNotes: map[string][]string{}}
	var attrNames []string
	cs := groupChanges(en.Work)
	slices.SortStableFunc(cs, func(a, b *change) int {
		return gocmp.Compare(boolInt(!mentioned(a, in.mentions)), boolInt(!mentioned(b, in.mentions)))
	})
	searched := en.CodeSearched && en.Service.Status == enrich.ServiceImplemented
	leaveOut := func(c *change, why string) {
		out.leftOut = append(out.leftOut, Inferred{Field: "left_out", Value: c.name, Basis: why})
	}
	unwritten := map[string][]string{} // operations → arguments no resource writes
	var unwrittenOps []string
	addRemove := map[string][]string{} // update operations → their Add and Remove members

	seenPlace := map[string]bool{}
	var headed []int // lines that start with their one resource
	for _, c := range cs {
		// An argument is in the resources that write it; the outputs that return it may be other resources'.
		ws := slices.DeleteFunc(slices.Clone(c.works), func(w enrich.Work) bool {
			return c.argument && w.Kind != enrich.WorkNewArgument
		})
		// A request member of query operations is a data source's argument, if any.
		query := c.argument && allVerbs(inputOperations(ws), queryVerbs...)
		rs := changeResources(ws, in.checkout, in.prefix, query)
		// Called means some provider file calls the operations, even one that registers no resource, such as a helper.
		isCalled := called(ws)
		member := memberName(c.works[0])
		if searched && c.argument {
			ops := inputOperations(ws)
			switch {
			case strings.HasPrefix(c.name, "reset_") && c.field != nil && c.field.Type == "boolean":
				leaveOut(c, "a flag that resets the value, which removing the argument does")
				continue
			case (strings.HasPrefix(c.name, "add_") || strings.HasPrefix(c.name, "remove_")) && allVerbs(ops, "Update", "Modify"):
				k := strings.Join(ops, ", ")
				addRemove[k] = append(addRemove[k], member)
				leaveOut(c, "an update's list of values to add or remove; a note covers it")
				continue
			case query && len(rs) == 0:
				leaveOut(c, "a request member of "+joinAnd(quoted(ops))+", which no data source calls")
				continue
			case !isCalled:
				k := strings.Join(ops, ", ")
				if _, ok := unwritten[k]; !ok {
					unwrittenOps = append(unwrittenOps, k)
				}
				unwritten[k] = append(unwritten[k], c.name)
				continue
			}
		}
		// An attribute named like a new resource with no read of its own is what that resource reads, as
		// GetAccountResponse.DeliveryAttributes for aws_sesv2_account_delivery_attributes, though an existing resource
		// calls GetAccount too.
		// The attribute must come from reads, and no calling resource may be named exactly for the read's noun.
		if !c.argument && strings.Contains(c.name, "_") && allVerbs(outputOperations(c.works), queryVerbs...) && !exactlyNamed(rs, outputOperations(c.works), in.prefix) {
			if i := slices.IndexFunc(in.newTypes, func(n string) bool { return strings.HasSuffix(n, "_"+c.name) }); i >= 0 {
				t := in.newTypes[i]
				out.typeNotes[t] = append(out.typeNotes[t], fmt.Sprintf("Read `%s` from %s.", c.name, joinAnd(c.subjects)))
				continue
			}
		}
		if searched && !c.argument && !isCalled {
			leaveOut(c, "no resource or data source calls "+joinAnd(quoted(outputOperations(c.works))))
			continue
		}

		var bare []string
		for _, r := range rs {
			bare = append(bare, strings.TrimSuffix(r, dataSourceMark))
		}
		keep := ownedBy(in.checkout, bare)
		paths := c.pathsIn(keep)
		var wheres []string
		for _, p := range paths {
			wheres = append(wheres, strings.TrimPrefix(p+"."+c.name, "."))
		}
		if len(wheres) == 0 {
			wheres = []string{c.name}
		}
		where := wheres[0]
		head := "`" + where + "`"
		if len(rs) > 0 {
			head = heads(rs)
		}
		kind := "attribute"
		if c.argument {
			kind = "argument"
		}
		if len(wheres) > 1 {
			kind += "s"
		}
		line := fmt.Sprintf("- %s: new %s %s", head, kind, joinAnd(quoted(wheres)))
		if len(rs) == 1 {
			headed = append(headed, len(out.lines))
		}
		if c.field != nil {
			line += " (" + fieldText(*c.field, c.argument) + ")"
		}
		if !namedOperations(in.releaseNotes, c.works) {
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
		if c.field != nil {
			if single := singleScalar(*c.field); single != "" {
				line += decide(where, fmt.Sprintf(decideSetOrBlock, single))
			}
			if c.argument && c.returns && !c.field.Required && (c.field.Default != "" || collection(*c.field)) {
				line += decide(where, decideComputed)
			}
		}
		out.lines = append(out.lines, line)
		if c.field != nil {
			out.lines = append(out.lines, fieldLines(c.field.Fields, 1, c.argument, where, "", decide)...)
		}
		for _, r := range rs {
			if !slices.Contains(out.targets, r) {
				out.targets = append(out.targets, r)
			}
		}
		switch {
		case len(rs) == 0:
		case c.argument:
			out.names = append(out.names, wheres...)
		default:
			attrNames = append(attrNames, wheres...)
		}

		for _, w := range c.works {
			for _, p := range w.Schema {
				k := fmt.Sprintf("%s:%d", p.File, p.Line)
				if seenPlace[k] || !keep(p.File) {
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
				out.notes = append(out.notes, n+".")
			}
			// A member another shape already had may be implemented: say where its name is used.
			if w.SharedName && w.Kind == enrich.WorkNewArgument && len(w.FoundIn) > 0 {
				out.notes = append(out.notes, fmt.Sprintf("`%s` is already used in %s, so the argument may exist.", w.Member, joinAnd(quoted(w.FoundIn))))
			}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(addRemove)) {
		out.notes = append(out.notes, fmt.Sprintf("Update (%s) takes %s, so it applies the difference between the configured and current lists.",
			quotedList(k), joinAnd(quoted(addRemove[k]))))
	}
	for _, k := range unwrittenOps {
		verb := "is"
		if len(unwritten[k]) > 1 {
			verb = "are"
		}
		out.notes = append(out.notes, fmt.Sprintf("%s %s in %s, which no resource calls yet.", joinAnd(quoted(unwritten[k])), verb, quotedList(k)))
	}

	newType := slices.ContainsFunc(en.Work, func(w enrich.Work) bool {
		return w.Kind == enrich.WorkNewResource || w.Kind == enrich.WorkNewDataSource
	})
	if len(out.targets) == 1 && len(headed) == len(out.lines)-nestedLines(out.lines) && !newType {
		for _, i := range headed {
			rest := strings.TrimPrefix(out.lines[i], "- "+heads(out.targets[:1])+": new ")
			out.lines[i] = "- New " + rest
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
			out.lines = append(out.lines, line)
		case enrich.WorkNewOperation:
			ops := "operation"
			if len(w.Operations) > 1 {
				ops = "operations"
			}
			out.lines = append(out.lines, "- New "+ops+" "+joinAnd(quoted(w.Operations))+"."+decide(w.Subject, "resource, argument, or none"))
		case enrich.WorkTagging:
			if !slices.ContainsFunc(en.Work, func(w enrich.Work) bool { return w.Kind == enrich.WorkNewResource }) {
				out.lines = append(out.lines, "- New tagging operations "+joinAnd(quoted(w.Operations))+".")
			}
		case enrich.WorkTypeChange, enrich.WorkRemoval:
			out.lines = append(out.lines, "- `"+w.Subject+"`: "+w.Basis+".")
		}
	}
	if len(out.names) == 0 {
		out.names = attrNames
	}
	return out
}

// groupChanges groups new members into changes: the create, update, and read members of one noun's shapes, with the
// same name and parent blocks. An attribute with the name and blocks of another change joins it: an export's
// ExportDescription.FilterSpecification is what read returns for the export input's FilterSpecification, and a
// CertificateSummary member repeats the CertificateDetail one.
func groupChanges(works []enrich.Work) []*change {
	var cs []*change
	byKey := map[string]*change{}
	for _, w := range works {
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
		c.add(w)
	}

	// An argument of an operation input with no block joins the argument of the same name in a block the input's noun
	// names: PutConfigurationSetDeliveryOptionsRequest.SharedPoolDeliveryMode is
	// delivery_options.shared_pool_delivery_mode, since the Put sets the block's members.
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		if !c.argument || len(c.paths) > 0 {
			continue
		}
		shape, _, _ := strings.Cut(c.works[0].Subject, ".")
		if shapeNoun(shape) == shape {
			// Not an operation's input: a nested shape can have no block too.
			continue
		}
		noun := enrich.Snake(shapeNoun(shape))
		j := slices.IndexFunc(cs, func(o *change) bool {
			return o != c && o.argument && o.name == c.name && sharedCaller(c.works, o.works) && slices.ContainsFunc(o.paths, func(p string) bool {
				block := p[strings.LastIndex(p, ".")+1:]
				return block != "" && (noun == block || strings.HasSuffix(noun, "_"+block))
			})
		})
		if j < 0 {
			continue
		}
		for _, w := range c.works {
			cs[j].add(w)
		}
		cs = slices.Delete(cs, i, i+1)
		i--
	}

	// Arguments of the same name and blocks that a provider file writes with both are one field: a create input's and an
	// update input's of different nouns (CreateHostedZoneRequest.ChangeGuardrails and
	// UpdateHostedZoneFeaturesRequest.ChangeGuardrails), or of a block's create and update shapes (AgentAccessConfig and
	// AgentAccessConfigForUpdate). Their fields must match, so blocks of different shapes with a member of the same
	// name stay apart. Only the arguments' callers count, since a read result reaches more files, and sweepers and list
	// resources, which call every resource's operations, don't.
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		if !c.argument {
			continue
		}
		j := slices.IndexFunc(cs[:i], func(o *change) bool {
			return o.argument && o.name == c.name && slices.Equal(o.paths, c.paths) && sharedCaller(withoutListFiles(arguments(c.works)), withoutListFiles(arguments(o.works))) &&
				sameParents(c.works, o.works) && (sameScalar(o.field, c.field) || sameShape(o.field, c.field))
		})
		if j < 0 {
			continue
		}
		for _, w := range c.works {
			cs[j].add(w)
		}
		cs = slices.Delete(cs, i, i+1)
		i--
	}

	// An attribute-only change joins an argument of the same name and blocks: the argument's value as read returns it,
	// when its operations return the argument or the fields have the same shape. Attribute-only changes of the same
	// name and blocks join when one is from a summary shape, which repeats the full shape's members.
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		if c.argument {
			continue
		}
		ops := outputOperations(c.works)
		j := slices.IndexFunc(cs, func(o *change) bool {
			if o == c || !o.argument || o.name != c.name || !slices.Equal(o.paths, c.paths) {
				return false
			}
			returns := outputOperations(o.works)
			return !slices.ContainsFunc(ops, func(op string) bool { return !slices.Contains(returns, op) }) ||
				(sameShape(o.field, c.field) || sameScalar(o.field, c.field)) && readBy(c.works, o.works)
		})
		if j < 0 && fromSummary(c) {
			// A summary repeats the members of the full shape, the argument's or an attribute's, of the same resource:
			// the summary's callers, if any, also call the full shape's operations. List resources and sweepers call
			// only the list, so they don't count: ScopeSummary is what scope_list.go lists of aws_networksecuritymanager_scope.
			related := func(o *change) bool {
				return o != c && o.name == c.name && slices.Equal(o.paths, c.paths) && (sameScalar(o.field, c.field) || sameShape(o.field, c.field)) &&
					(!called(withoutListFiles(c.works)) || readBy(withoutListFiles(c.works), o.works))
			}
			j = slices.IndexFunc(cs, func(o *change) bool { return related(o) && o.argument })
			if j < 0 {
				j = slices.IndexFunc(cs, func(o *change) bool { return related(o) && !o.argument && !fromSummary(o) })
			}
		}
		if j < 0 {
			continue
		}
		for _, w := range c.works {
			cs[j].add(w)
		}
		cs = slices.Delete(cs, i, i+1)
		i--
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
	return cs
}

// sameParents reports whether two sets of works' members are in shapes that can be the same block: operation inputs
// (top-level members), or nested shapes whose names are the same apart from a create or update suffix or prefix
// (AgentAccessConfig and AgentAccessConfigForUpdate, CreateOntapVolumeConfiguration and UpdateOntapVolumeConfiguration). CloudFront's CustomOriginConfig and VpcOriginConfig both have OriginHttpVersion, in
// different blocks. Only the arguments' shapes count; a change can also hold read results.
func sameParents(a, b []enrich.Work) bool {
	parent := func(ws []enrich.Work) string {
		i := max(slices.IndexFunc(ws, func(w enrich.Work) bool { return w.Kind == enrich.WorkNewArgument }), 0)
		shape, _, _ := strings.Cut(ws[i].Subject, ".")
		if operationShape(shape, ws[i].Operations) {
			return ""
		}
		for _, suffix := range []string{"ForUpdate", "ForCreate", "Update", "Create"} {
			if rest, ok := strings.CutSuffix(shape, suffix); ok && rest != "" {
				return rest
			}
		}
		for _, verb := range []string{"Create", "Update", "Modify"} {
			if rest, ok := strings.CutPrefix(shape, verb); ok && rest != "" {
				return rest
			}
		}
		return shape
	}
	return parent(a) == parent(b)
}

// operationShape reports whether shape is one of the operations' input or output shapes: CreateHostedZoneRequest for
// CreateHostedZone. CustomMemoryStrategyInput ends like one but is a nested structure.
func operationShape(shape string, ops []string) bool {
	for _, suffix := range []string{"Input", "Output", "Request", "Response", "Result", "Message"} {
		if rest, ok := strings.CutSuffix(shape, suffix); ok && slices.Contains(ops, rest) {
			return true
		}
	}
	return false
}

// fromSummary reports whether a change's members are all from summary shapes, such as CertificateSummary, or from
// list results, which only List and Search operations return.
func fromSummary(c *change) bool {
	return !slices.ContainsFunc(c.works, func(w enrich.Work) bool {
		shape, _, _ := strings.Cut(w.Subject, ".")
		return !strings.HasSuffix(shape, "Summary") && !allVerbs(w.Operations, "List", "Search")
	})
}

// sameScalar reports whether two fields are the same scalar type, and for enums, have the same values.
func sameScalar(a, b *enrich.Field) bool {
	return a != nil && b != nil && a.Type == b.Type && len(a.Fields) == 0 && len(b.Fields) == 0 && slices.Equal(a.Values, b.Values)
}

// sharedCaller reports whether a provider file calls operations of both sets of works: the same resource has them.
func sharedCaller(a, b []enrich.Work) bool {
	return slices.ContainsFunc(a, func(x enrich.Work) bool {
		return slices.ContainsFunc(x.UsedBy, func(f string) bool {
			return slices.ContainsFunc(b, func(y enrich.Work) bool { return slices.Contains(y.UsedBy, f) })
		})
	})
}

// arguments returns the new argument works, leaving out read results that a change also holds.
func arguments(ws []enrich.Work) []enrich.Work {
	return slices.DeleteFunc(slices.Clone(ws), func(w enrich.Work) bool { return w.Kind != enrich.WorkNewArgument })
}

// withoutListFiles returns works without their list resource and sweeper callers (*_list.go, sweep.go, and
// list_pages_gen.go), which only list.
func withoutListFiles(ws []enrich.Work) []enrich.Work {
	out := make([]enrich.Work, len(ws))
	for i, w := range ws {
		w.UsedBy = slices.DeleteFunc(slices.Clone(w.UsedBy), func(f string) bool {
			base := path.Base(f)
			return strings.HasSuffix(base, "_list.go") || base == "sweep.go" || base == "list_pages_gen.go"
		})
		out[i] = w
	}
	return out
}

// called reports whether a provider file calls one of the works' operations.
func called(ws []enrich.Work) bool {
	return slices.ContainsFunc(ws, func(w enrich.Work) bool { return len(w.UsedBy) > 0 })
}

// readBy reports whether every file that calls the attributes' operations also calls the arguments': the same
// resource reads the value it writes.
func readBy(attrs, args []enrich.Work) bool {
	var argFiles []string
	for _, w := range args {
		argFiles = append(argFiles, w.UsedBy...)
	}
	n := 0
	for _, w := range attrs {
		for _, f := range w.UsedBy {
			if !slices.Contains(argFiles, f) {
				return false
			}
			n++
		}
	}
	return n > 0
}

// sameShape reports whether two fields are the same structure, list, or map: the same type and nested field names.
// Scalars of the same type are often unrelated, so they aren't the same shape.
func sameShape(a, b *enrich.Field) bool {
	if a == nil || b == nil || a.Type != b.Type || a.Elem != b.Elem || len(a.Fields) == 0 || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i].Name != b.Fields[i].Name || a.Fields[i].Type != b.Fields[i].Type {
			return false
		}
	}
	return true
}

// add adds a work to a change.
func (c *change) add(w enrich.Work) {
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
		return
	}
	c.attribute = true
	if c.field == nil {
		c.field, c.fieldFrom = w.Field, w.Subject
	}
}

// pathsIn returns the change's parent block paths in the files keep accepts. Several paths in one resource's files
// are blocks of the same shape, so the member goes in each: SageMaker's production_variants and
// shadow_production_variants.
func (c *change) pathsIn(keep func(file string) bool) []string {
	var out []string
	for _, w := range c.works {
		for _, p := range w.Schema {
			if p.Path != "" && keep(p.File) && !slices.Contains(out, p.Path) {
				out = append(out, p.Path)
			}
		}
	}
	slices.Sort(out)
	return out
}

// mentioned reports whether text names a change's member: as written (HierarchyFilter) or as words (hierarchy
// filter), in any case.
func mentioned(c *change, text string) bool {
	if text == "" {
		return false
	}
	text = strings.ToLower(text)
	m := strings.ToLower(memberName(c.works[0]))
	words := strings.ReplaceAll(c.name, "_", " ")
	return wordIn(text, m) || strings.Contains(words, " ") && wordIn(text, words)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// memberName is a member work's API member name.
func memberName(w enrich.Work) string {
	_, m, _ := strings.Cut(w.Subject, ".")
	return m
}

// queryVerbs start operations whose input is filters and identifiers.
var queryVerbs = []string{"Get", "Describe", "List", "BatchGet", "Search"}

// allVerbs reports whether every operation starts with one of verbs.
func allVerbs(ops []string, verbs ...string) bool {
	return len(ops) > 0 && !slices.ContainsFunc(ops, func(op string) bool {
		return !slices.ContainsFunc(verbs, func(v string) bool { return strings.HasPrefix(op, v) })
	})
}

// inputOperations are the operations whose input reaches the works' members, sorted.
func inputOperations(ws []enrich.Work) []string {
	var ops []string
	for _, w := range ws {
		for _, op := range w.Operations {
			if !slices.Contains(ops, op) {
				ops = append(ops, op)
			}
		}
	}
	slices.Sort(ops)
	return ops
}

// outputOperations are the operations whose output reaches the works' members, sorted.
func outputOperations(ws []enrich.Work) []string {
	var ops []string
	for _, w := range ws {
		src := w.Operations
		if w.Kind == enrich.WorkNewArgument {
			src = w.Returns
		}
		for _, op := range src {
			if !slices.Contains(ops, op) {
				ops = append(ops, op)
			}
		}
	}
	slices.Sort(ops)
	return ops
}

// ownedBy returns whether a file belongs to one of the resources names: it registers one of them, or no type at all,
// as a schema helper does. A file registering only other types is another resource's. Without a checkout or names,
// every file is kept.
func ownedBy(checkout string, names []string) func(file string) bool {
	if checkout == "" || len(names) == 0 {
		return func(string) bool { return true }
	}
	return func(file string) bool {
		ts := resourceNames(checkout, []string{file})
		return len(ts) == 0 || slices.ContainsFunc(ts, func(t typeName) bool { return slices.Contains(names, t.Name) })
	}
}

// nestedLines counts the lines that are a field's nested fields, which start with spaces.
func nestedLines(lines []string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, " ") {
			n++
		}
	}
	return n
}

// quotedList quotes the items of a ", "-joined list and joins them with "and".
func quotedList(s string) string {
	return joinAnd(quoted(strings.Split(s, ", ")))
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

// changeResources returns the Terraform types in the files that call all of the works' operations, or, if no file
// calls them all, in any of the files. Resources are preferred to data sources, or with dataSourcesOnly, only data
// sources are kept. Of several, those whose names best
// match the operations' nouns are kept: aws_rds_cluster, not aws_rds_global_cluster, for ModifyDBCluster, whose
// file also calls it.
func changeResources(ws []enrich.Work, checkout, prefix string, dataSourcesOnly bool) []string {
	var common, all []typeName
	first := true
	var ops []string
	for _, w := range ws {
		ops = append(ops, w.Operations...)
		if len(w.UsedBy) == 0 {
			continue
		}
		rs := resourceNames(checkout, w.UsedBy)
		if dataSourcesOnly {
			rs = slices.DeleteFunc(rs, func(t typeName) bool { return !t.DataSource })
		}
		for _, r := range rs {
			if !slices.Contains(all, r) {
				all = append(all, r)
			}
		}
		if first {
			common, first = rs, false
			continue
		}
		common = slices.DeleteFunc(common, func(r typeName) bool { return !slices.Contains(rs, r) })
	}
	ts := common
	if len(ts) == 0 {
		ts = all
	}
	switch {
	case dataSourcesOnly:
	case slices.ContainsFunc(ts, func(t typeName) bool { return !t.DataSource }):
		ts = slices.DeleteFunc(slices.Clone(ts), func(t typeName) bool { return t.DataSource })
	}
	var names []string
	for _, t := range ts {
		n := t.Name
		if t.DataSource {
			// A data source can share its resource's name, so it's marked.
			n += dataSourceMark
		}
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return shapeNamed(bestNamed(names, ops, prefix), ws, prefix)
}

// shapeNamed keeps, of several resources, those whose names have the most words of the members' parent shape, when
// it's a nested shape rather than an operation's input or output: aws_fsx_ontap_volume for
// CreateOntapVolumeConfiguration.CloneConfiguration, not aws_fsx_openzfs_volume, and aws_fsx_openzfs_snapshot for
// Snapshot.SnapshotType, not every resource whose operations return a snapshot. Verbs and generic words don't count.
// With no resource named for a word, all are kept.
func shapeNamed(names []string, ws []enrich.Work, prefix string) []string {
	if len(names) < 2 {
		return names
	}
	var shapeWords []string
	for _, w := range ws {
		shape, _, _ := strings.Cut(w.Subject, ".")
		if shapeNoun(shape) != shape {
			return names
		}
		for word := range strings.SplitSeq(enrich.Snake(shape), "_") {
			if !slices.Contains(genericWords, word) && !slices.ContainsFunc(shapeVerbs, func(v string) bool { return strings.EqualFold(v, word) }) &&
				!slices.Contains(shapeWords, word) {
				shapeWords = append(shapeWords, word)
			}
		}
	}
	counts := make([]int, len(names))
	for i, n := range names {
		for _, word := range strings.Split(strings.TrimPrefix(strings.TrimSuffix(n, dataSourceMark), prefix), "_")[1:] {
			if slices.Contains(shapeWords, word) {
				counts[i]++
			}
		}
	}
	best := slices.Max(counts)
	if best == 0 {
		return names
	}
	var out []string
	for i, n := range names {
		if counts[i] == best {
			out = append(out, n)
		}
	}
	return out
}

// heads formats a line's resources: "`aws_x`, `aws_y` data source".
func heads(rs []string) string {
	var qs []string
	for _, r := range rs {
		if name, ok := strings.CutSuffix(r, dataSourceMark); ok {
			qs = append(qs, "`"+name+"`"+dataSourceMark)
			continue
		}
		qs = append(qs, "`"+r+"`")
	}
	return strings.Join(qs, ", ")
}

// dataSourceMark follows a data source's name among a change's resources.
const dataSourceMark = " data source"

// genericWords are resource name words that don't name the noun: aws_s3_bucket_replication_configuration is about
// BucketReplication.
var genericWords = []string{"configuration", "config", "settings", "attributes", "options"}

// bestNamed keeps, for each operation, the names that best match its noun: the fewest words the noun doesn't have,
// then the most it has. A name's first word after the prefix is the service's, so it isn't counted. The names kept
// for any operation are returned in order; with none matched, all are. For ModifyDBCluster, aws_rds_cluster matches
// and aws_rds_global_cluster has a word more. For CreateWebACL and CreateRuleGroup, aws_wafv2_web_acl and
// aws_wafv2_rule_group match exactly, and aws_wafv2_web_acl_rule_group_association has more words than either noun.
func bestNamed(names, ops []string, prefix string) []string {
	if len(names) < 2 {
		return names
	}
	type score struct{ extra, matched int }
	better := func(a, b score) int {
		return gocmp.Or(gocmp.Compare(a.extra, b.extra), gocmp.Compare(b.matched, a.matched))
	}
	words := make([][]string, len(names))
	for i, n := range names {
		words[i] = strings.Split(strings.TrimPrefix(strings.TrimSuffix(n, dataSourceMark), prefix), "_")[1:]
	}
	won := make([]bool, len(names))
	for _, op := range slices.Compact(slices.Sorted(slices.Values(ops))) {
		noun := strings.Split(enrich.Snake(opNoun(op)), "_")
		scores := make([]score, len(names))
		for i := range names {
			for _, w := range words[i] {
				switch {
				case slices.Contains(noun, w):
					scores[i].matched++
				case !slices.Contains(genericWords, w):
					scores[i].extra++
				}
			}
		}
		matched := slices.DeleteFunc(slices.Clone(scores), func(s score) bool { return s.matched == 0 })
		if len(matched) == 0 {
			continue
		}
		best := slices.MinFunc(matched, better)
		for i := range names {
			if scores[i] == best {
				won[i] = true
			}
		}
	}
	var out []string
	for i, n := range names {
		if won[i] {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return names
	}
	return out
}

// exactlyNamed reports whether one of the resources is named for one of the operations' nouns with no other words,
// as aws_widget_account for GetAccount. Such a resource owns what the operation reads.
func exactlyNamed(rs, ops []string, prefix string) bool {
	return slices.ContainsFunc(rs, func(r string) bool {
		words := strings.Split(strings.TrimPrefix(strings.TrimSuffix(r, dataSourceMark), prefix), "_")[1:]
		return slices.ContainsFunc(ops, func(op string) bool {
			noun := strings.Split(enrich.Snake(opNoun(op)), "_")
			return len(words) > 0 && !slices.ContainsFunc(words, func(w string) bool { return !slices.Contains(noun, w) && !slices.Contains(genericWords, w) })
		})
	})
}

// opNoun is an operation's name without its first word: DBCluster for ModifyDBCluster, Get for BatchGetX's X.
func opNoun(op string) string {
	if rest, ok := strings.CutPrefix(op, "BatchGet"); ok {
		return rest
	}
	for i, r := range op {
		if i > 0 && 'A' <= r && r <= 'Z' {
			return op[i:]
		}
	}
	return op
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
