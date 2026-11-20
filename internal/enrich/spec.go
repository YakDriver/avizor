package enrich

import (
	"cmp"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/YakDriver/avizor/internal/smithy"
)

// Spec is what the model says about an inferred resource or data source: its operations by role, arguments,
// attributes, status, delete and update shapes, tagging, and documentation sentences that state design facts. It's
// model facts, not inferences about the Terraform schema, except for snake_case names and create-only, which compares
// the create input with the update inputs.
type Spec struct {
	Operations SpecOperations `json:"operations"`
	// Identifier are the read operation's required input members.
	Identifier []string `json:"identifier,omitempty"`
	// Arguments are the create operation's input members, or the read operation's for a data source.
	Arguments []Field `json:"arguments,omitempty"`
	// Attributes are members of the read result that aren't arguments.
	Attributes []Field     `json:"attributes,omitempty"`
	Status     *SpecStatus `json:"status,omitempty"`
	Delete     *SpecDelete `json:"delete,omitempty"`
	Update     *SpecUpdate `json:"update,omitempty"`
	Tagging    *SpecTags   `json:"tagging,omitempty"`
	// Notes are documentation sentences that state design facts, quoted.
	Notes []SpecNote `json:"notes,omitempty"`
}

// SpecOperations are a resource's operations by role.
type SpecOperations struct {
	Create []string `json:"create,omitempty"`
	Read   []string `json:"read,omitempty"`
	Update []string `json:"update,omitempty"`
	Delete []string `json:"delete,omitempty"`
	List   []string `json:"list,omitempty"`
	Other  []string `json:"other,omitempty"`
}

// Field is an argument or attribute.
type Field struct {
	// Name is the likely Terraform name; Member the API member.
	Name   string `json:"name"`
	Member string `json:"member"`
	// Type is the Smithy type of the member's target: string, integer, boolean, timestamp, enum, list, map,
	// structure, union. Elem is a list's or map's element type.
	Type     string `json:"type"`
	Elem     string `json:"elem,omitempty"`
	Required bool   `json:"required,omitempty"`
	// CreateOnly is set for a create input member that no update operation takes, so it likely forces replacement.
	CreateOnly bool   `json:"create_only,omitempty"`
	Default    string `json:"default,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
	// Ignored is set when the member's documentation says it can't be set or is ignored on write.
	Ignored     bool               `json:"ignored,omitempty"`
	Constraints smithy.Constraints `json:"constraints,omitzero"`
	// Values are an enum's values.
	Values []string `json:"values,omitempty"`
	// Returned is set for a resource argument that the read result also has.
	Returned bool `json:"returned,omitempty"`
	// Fields are a structure's, union's, or list or map element structure's members.
	Fields []Field `json:"fields,omitempty"`
	// Recursive is the structure or union, or the list or map element, when one of its members refers back to it, as
	// DynamoDB's AttributeValue does. Its members aren't listed.
	Recursive string `json:"recursive,omitempty"`
}

// SpecStatus is the read result's status member and its values, grouped.
type SpecStatus struct {
	Member  string   `json:"member"`
	Pending []string `json:"pending,omitempty"`
	Target  []string `json:"target,omitempty"`
	Failed  []string `json:"failed,omitempty"`
	Delete  []string `json:"delete,omitempty"`
	Other   []string `json:"other,omitempty"`
	// Reason is a member that explains a failure, as StatusReason.
	Reason string `json:"reason,omitempty"`
}

// SpecDelete is how the resource is deleted: Kind is "delete", "cancel", another ending verb in lower case
// ("disassociate"), or "none". Plural is set when the operation takes a list of identifiers, up to Max.
type SpecDelete struct {
	Kind      string `json:"kind"`
	Operation string `json:"operation,omitempty"`
	Plural    bool   `json:"plural,omitempty"`
	Member    string `json:"member,omitempty"`
	Max       int    `json:"max,omitempty"`
}

// SpecUpdate is how the resource is updated: Kind is "none", "put" (the create operation is a Put, which also
// updates), "add_remove" (members to add and to remove), or "fields". Replaces lists the update operations and input
// members whose documentation says the values they take replace the current ones.
type SpecUpdate struct {
	Kind     string   `json:"kind"`
	Replaces []string `json:"replaces,omitempty"`
	Members  []string `json:"members,omitempty"`
}

// SpecTags is how the resource is tagged: Kind is "tags" (a create input member), "tag_specifications" (EC2, with the
// new ResourceType values), "tag_resource" (the service's TagResource operation), or "none".
type SpecTags struct {
	Kind          string   `json:"kind"`
	Member        string   `json:"member,omitempty"`
	ResourceTypes []string `json:"resource_types,omitempty"`
}

// SpecNote is a quoted documentation sentence and where it's from: an operation or Shape.Member.
type SpecNote struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

const (
	specDepth = 3
	maxNotes  = 15
	// maxValues is how many enum values FieldText lists.
	maxValues = 12
)

// num formats a constraint bound without an exponent: 2147483647, 0.5.
func num(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// pagingMembers are input members for paging and dry runs, which aren't resource arguments.
var pagingMembers = []string{"NextToken", "MaxResults", "nextToken", "maxResults", "DryRun", "MaxItems", "Marker"}

// buildSpecs adds a spec to each new resource and data source.
func buildSpecs(ws []Work, m *smithy.Model, d *smithy.Diff) {
	if m == nil {
		return
	}
	var resourceTypes []string
	if d != nil {
		for _, v := range d.AddedEnumValues {
			if v.Shape == "ResourceType" {
				resourceTypes = append(resourceTypes, v.Value)
			}
		}
	}
	for i := range ws {
		if ws[i].Kind == WorkNewResource || ws[i].Kind == WorkNewDataSource {
			ws[i].Spec = buildSpec(m, ws[i], resourceTypes)
		}
	}
}

func buildSpec(m *smithy.Model, w Work, resourceTypes []string) *Spec {
	s := &Spec{Operations: roles(w.Subject, w.Operations)}
	ops := s.Operations

	create, read := first(ops.Create), first(ops.Read)
	if create == "" && read == "" {
		// A list-only data source (ListContactFlowAliases) reads with its list.
		read = first(ops.List)
	}
	updateInputs := map[string]bool{}
	for _, op := range ops.Update {
		for name := range m.Shapes[m.Shapes[op].Input].Members {
			updateInputs[name] = true
		}
	}
	if read != "" {
		in := m.Shapes[m.Shapes[read].Input]
		for _, name := range sortedNames(in.Members) {
			if in.Info[name].Required {
				s.Identifier = append(s.Identifier, name)
			}
		}
	}
	// A Put that creates also updates: only the identifier can't change.
	putCreates := strings.HasPrefix(create, "Put")
	if argOp := cmp.Or(create, read); argOp != "" {
		s.Arguments = fields(m, m.Shapes[argOp].Input, 1)
		if create != "" {
			for i := range s.Arguments {
				a := &s.Arguments[i]
				switch {
				case slices.Contains(s.Identifier, a.Member):
					a.CreateOnly = true
				case putCreates, a.Member == "Tags", a.Member == "tags", a.Member == "TagSpecifications":
					// Tags are usually updated with tagging operations.
				default:
					a.CreateOnly = !updateInputs[a.Member]
				}
			}
		}
	}
	if read != "" {
		result := readResult(m, m.Shapes[read].Output)
		args := map[string]bool{}
		for _, f := range s.Arguments {
			args[strings.ToLower(f.Member)] = true
		}
		if create == "" {
			// For a data source, the arguments are the read input; everything in the result is an attribute.
			args = map[string]bool{}
		}
		if create != "" && (args["tagspecifications"] || args["tags"]) {
			args["tags"] = true
		}
		returned := map[string]bool{}
		for _, f := range fields(m, result, 2) {
			returned[strings.ToLower(f.Member)] = true
			if !args[strings.ToLower(f.Member)] {
				s.Attributes = append(s.Attributes, f)
			}
		}
		if create != "" {
			for i := range s.Arguments {
				s.Arguments[i].Returned = returned[strings.ToLower(s.Arguments[i].Member)]
			}
		}
		s.Status = status(m, result)
	}
	if create != "" {
		s.Delete = deleteShape(m, ops.Delete)
		s.Update = updateShape(m, ops.Update)
		if putCreates && s.Update.Kind == "none" {
			s.Update = &SpecUpdate{Kind: "put"}
		}
		outputs := []string{m.Shapes[create].Output}
		if read != "" {
			outputs = append(outputs, readResult(m, m.Shapes[read].Output))
		}
		s.Tagging = tagging(m, m.Shapes[create].Input, resourceTypes, slices.ContainsFunc(outputs, func(o string) bool { return hasARN(m, o) }))
	}
	noteOps, noteInputs := slices.Concat(ops.Create, ops.Update, ops.Delete, ops.Read), slices.Concat(argInputs(m, ops.Create), argInputs(m, ops.Update))
	if create == "" && read != "" {
		// A data source's arguments are its read input.
		if !slices.Contains(noteOps, read) {
			noteOps = append(noteOps, read)
		}
		noteInputs = argInputs(m, []string{read})
	}
	s.Notes = notes(m, noteOps, noteInputs)
	return s
}

// roles sorts a group's operations by role. An operation whose noun is the group's subject is singular; a Describe,
// Get, or List of another noun (the plural) lists. With no singular read, a plural Describe or Get reads, as EC2's do.
func roles(subject string, ops []string) SpecOperations {
	var r SpecOperations
	has := func(verbs ...string) bool {
		return slices.ContainsFunc(ops, func(op string) bool { v, _ := splitVerb(op); return slices.Contains(verbs, v) })
	}
	hasCreate := has("Create")
	for _, op := range ops {
		v, noun := splitVerb(op)
		singularNoun := noun == subject
		switch {
		case v == "Create", v == "Put" && !hasCreate, !hasCreate && !has("Put") && slices.Contains([]string{"Register", "Associate", "Attach", "Enable"}, v):
			r.Create = append(r.Create, op)
		case (v == "Get" || v == "Describe") && singularNoun:
			r.Read = append(r.Read, op)
		case v == "List" || v == "BatchGet" || v == "Search" || v == "Describe" || v == "Get" && !singularNoun:
			r.List = append(r.List, op)
		case v == "Update" || v == "Modify" || v == "Put":
			r.Update = append(r.Update, op)
		case v == "Delete" || v == "Cancel", !hasCreate && !has("Put") && slices.Contains([]string{"Deregister", "Disassociate", "Detach", "Disable"}, v):
			r.Delete = append(r.Delete, op)
		default:
			r.Other = append(r.Other, op)
		}
	}
	if len(r.Read) == 0 {
		if i := slices.IndexFunc(r.List, func(op string) bool { v, _ := splitVerb(op); return v == "Describe" || v == "Get" }); i >= 0 {
			r.Read = append(r.Read, r.List[i])
			r.List = slices.Delete(r.List, i, i+1)
		}
	}
	return r
}

func first(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

// fields returns a structure's members as fields, down to specDepth levels.
func fields(m *smithy.Model, shape string, depth int) []Field {
	sh, ok := m.Shapes[shape]
	if !ok || (sh.Type != smithy.TypeStructure && sh.Type != smithy.TypeUnion) {
		return nil
	}
	var out []Field
	for _, name := range sortedNames(sh.Members) {
		info := sh.Info[name]
		if info.IdempotencyToken || slices.Contains(pagingMembers, name) {
			continue
		}
		out = append(out, fieldOf(m, sh, name, depth))
	}
	return out
}

// memberFields adds the model's field to each new argument and attribute.
func memberFields(ws []Work, m *smithy.Model) {
	if m == nil {
		return
	}
	for i := range ws {
		w := &ws[i]
		if w.Kind != WorkNewArgument && w.Kind != WorkNewAttribute {
			continue
		}
		shape, member, _ := strings.Cut(w.Subject, ".")
		if sh, ok := m.Shapes[shape]; ok {
			if _, ok := sh.Members[member]; ok {
				f := fieldOf(m, sh, member, 1)
				w.Field = &f
			}
		}
	}
}

// fieldOf returns a structure's member as a field, with nested fields down to specDepth levels.
func fieldOf(m *smithy.Model, sh smithy.Shape, name string, depth int) Field {
	info := sh.Info[name]
	target := sh.Members[name]
	f := Field{Name: Snake(name), Member: name, Type: m.TypeOf(target), Required: info.Required, Default: info.Default,
		Deprecated: info.Deprecated, Constraints: m.Shapes[target].Constraints.Merge(info.Constraints)}
	if doc := text(sh.Docs[name]); notSettableRE.MatchString(doc) || ignoresRE.MatchString(doc) {
		f.Ignored = true
	}
	ts := m.Shapes[target]
	switch f.Type {
	case smithy.TypeEnum, smithy.TypeIntEnum:
		f.Values = enumValues(ts)
	case smithy.TypeStructure, smithy.TypeUnion:
		switch {
		case recursive(m, target):
			f.Recursive = target
		case depth < specDepth:
			f.Fields = fields(m, target, depth+1)
		}
	case "list", "set", "map":
		elem := ts.Elems[len(ts.Elems)-1]
		f.Elem = m.TypeOf(elem)
		switch f.Elem {
		case smithy.TypeEnum, smithy.TypeIntEnum:
			f.Values = enumValues(m.Shapes[elem])
		case smithy.TypeStructure, smithy.TypeUnion:
			switch {
			case recursive(m, elem):
				f.Recursive = elem
			case depth < specDepth:
				f.Fields = fields(m, elem, depth+1)
			}
		}
	}
	return f
}

// recursive reports whether a structure or union has a member that targets it, directly or as a list or map element.
func recursive(m *smithy.Model, shape string) bool {
	for _, t := range m.Shapes[shape].Members {
		if t == shape || slices.Contains(m.Shapes[t].Elems, shape) {
			return true
		}
	}
	return false
}

func enumValues(sh smithy.Shape) []string {
	var vs []string
	for _, v := range sh.Members {
		vs = append(vs, v)
	}
	slices.Sort(vs)
	return vs
}

func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// readResult returns the structure a read operation's output describes: its only member other than paging, when
// that's a structure (DescribeWidgetOutput.Widget) or a list of structures (a plural Describe), or else the output
// itself, as a flat output with the resource's fields.
func readResult(m *smithy.Model, output string) string {
	out := m.Shapes[output]
	var payload []string
	for _, name := range sortedNames(out.Members) {
		if !slices.Contains(pagingMembers, name) {
			payload = append(payload, out.Members[name])
		}
	}
	if len(payload) != 1 {
		return output
	}
	t := payload[0]
	switch m.TypeOf(t) {
	case smithy.TypeStructure:
		return t
	case "list":
		if e := m.Shapes[t].Elems; len(e) == 1 && m.TypeOf(e[0]) == smithy.TypeStructure {
			return e[0]
		}
	}
	return output
}

var (
	statusNameRE = regexp.MustCompile(`(?i)(status|state)$`)
	reasonNameRE = regexp.MustCompile(`(?i)(status|state)(reason|message)$|^(status|state)?(reason|message)$|failurereason`)
	failedRE     = regexp.MustCompile(`FAIL|ERROR`)
	deleteRE     = regexp.MustCompile(`^DELET`)
	pendingRE    = regexp.MustCompile(`ING$|PENDING|INPROGRESS|QUEUED`)
	targetValues = []string{"ACTIVE", "AVAILABLE", "INSERVICE", "READY", "ENABLED", "CREATED", "COMPLETED", "COMPLETE", "SUCCEEDED", "RUNNING", "HEALTHY", "OK", "ATTACHED", "ASSOCIATED", "DEPLOYED", "PROVISIONED", "UPDATED"}
)

// status finds the result's enum status member and groups its values. A member named Status or State is preferred,
// then one ending in either.
func status(m *smithy.Model, result string) *SpecStatus {
	sh := m.Shapes[result]
	var cands []string
	for _, name := range sortedNames(sh.Members) {
		if t := m.TypeOf(sh.Members[name]); t == smithy.TypeEnum && statusNameRE.MatchString(name) {
			cands = append(cands, name)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	slices.SortStableFunc(cands, func(a, b string) int {
		return cmp.Compare(len(a), len(b))
	})
	s := &SpecStatus{Member: cands[0]}
	for _, v := range enumValues(m.Shapes[sh.Members[s.Member]]) {
		n := strings.ToUpper(strings.NewReplacer("_", "", "-", "", " ", "").Replace(v))
		switch {
		case failedRE.MatchString(n):
			s.Failed = append(s.Failed, v)
		case deleteRE.MatchString(n):
			s.Delete = append(s.Delete, v)
		case slices.Contains(targetValues, n):
			s.Target = append(s.Target, v)
		case pendingRE.MatchString(n):
			s.Pending = append(s.Pending, v)
		default:
			s.Other = append(s.Other, v)
		}
	}
	for _, name := range sortedNames(sh.Members) {
		if reasonNameRE.MatchString(name) && m.TypeOf(sh.Members[name]) == "string" {
			s.Reason = name
			break
		}
	}
	return s
}

// deleteShape says how a resource is deleted, from its first delete operation.
func deleteShape(m *smithy.Model, ops []string) *SpecDelete {
	if len(ops) == 0 {
		return &SpecDelete{Kind: "none"}
	}
	op := ops[0]
	v, _ := splitVerb(op)
	d := &SpecDelete{Kind: strings.ToLower(v), Operation: op}
	in := m.Shapes[m.Shapes[op].Input]
	for _, name := range sortedNames(in.Members) {
		t := in.Members[name]
		if !in.Info[name].Required || m.TypeOf(t) != "list" {
			continue
		}
		d.Plural, d.Member = true, name
		if c := m.Shapes[t].Constraints.Merge(in.Info[name].Constraints); c.LengthMax != nil {
			d.Max = int(*c.LengthMax)
		}
		break
	}
	return d
}

// ignoresRE finds "ignores this field in CreateX" wording, which notSettableRE doesn't match.
var ignoresRE = regexp.MustCompile(`(?i)\bignores (?:this|the) (?:field|parameter|member|value)\b`)

var addRemoveRE = regexp.MustCompile(`(?i)(ToAdd|ToRemove|ToDelete)$|^(add|remove|delete)$`)

// updateShape says how a resource is updated.
func updateShape(m *smithy.Model, ops []string) *SpecUpdate {
	if len(ops) == 0 {
		return &SpecUpdate{Kind: "none"}
	}
	u := &SpecUpdate{Kind: "fields"}
	for _, op := range ops {
		in := m.Shapes[m.Shapes[op].Input]
		for _, name := range sortedNames(in.Members) {
			if addRemoveRE.MatchString(name) {
				u.Kind = "add_remove"
				u.Members = append(u.Members, name)
			}
		}
		if replacesRE.MatchString(text(m.Shapes[op].Doc)) {
			u.Replaces = append(u.Replaces, op)
		}
		for _, name := range sortedNames(in.Members) {
			if replacesRE.MatchString(text(in.Docs[name])) {
				u.Replaces = append(u.Replaces, m.Shapes[op].Input+"."+name)
			}
		}
	}
	return u
}

var replacesRE = regexp.MustCompile(`(?i)\breplaces? (?:all|the|any|its|their|your|current|existing)\b|\breplaces? [^.]{0,40}\bcurrent\b`)

// tagging says how the resource is tagged. TagResource takes an ARN, so a resource is tagged with it only when its
// create or read output has an ARN: account settings set with a Put have none.
func tagging(m *smithy.Model, input string, resourceTypes []string, arn bool) *SpecTags {
	in := m.Shapes[input]
	switch {
	case in.Members["Tags"] != "":
		return &SpecTags{Kind: "tags", Member: "Tags"}
	case in.Members["tags"] != "":
		return &SpecTags{Kind: "tags", Member: "tags"}
	case in.Members["TagSpecifications"] != "":
		return &SpecTags{Kind: "tag_specifications", Member: "TagSpecifications", ResourceTypes: resourceTypes}
	case arn && m.Shapes["TagResource"].Type == smithy.TypeOperation:
		return &SpecTags{Kind: "tag_resource"}
	}
	return &SpecTags{Kind: "none"}
}

// hasARN reports whether a shape, or a structure it holds, has a member named for an ARN.
func hasARN(m *smithy.Model, shape string) bool {
	isARN := func(name string) bool { return strings.HasSuffix(strings.ToLower(name), "arn") }
	for name, t := range m.Shapes[shape].Members {
		if isARN(name) {
			return true
		}
		if m.TypeOf(t) == smithy.TypeStructure && slices.ContainsFunc(sortedNames(m.Shapes[t].Members), isARN) {
			return true
		}
	}
	return false
}

// argInputs returns the input shapes of operations.
func argInputs(m *smithy.Model, ops []string) []string {
	var out []string
	for _, op := range ops {
		if in := m.Shapes[op].Input; in != "" {
			out = append(out, in)
		}
	}
	return out
}

// designRE finds documentation sentences that state design facts: what can't change, what's ignored, what replaces
// what, what an omitted value does, asynchronous operations, required states, and read-only values.
var designRE = regexp.MustCompile(`(?i)\b(?:can't|cannot|can not|isn't possible to|you can't)\b[^.]{0,60}\b(?:change|modif|updat|remov|renam|delet)|` +
	`\bafter (?:you create|creation|it's created|it is created|the \w+ is created)|` +
	`\bignore[sd]?\b|\breplaces? (?:all|the|any|its|their|your|current|existing)\b|` +
	`\bif you (?:omit|don't specify|do not specify)\b|\b(?:is|are|runs?) asynchronous|\basynchronously\b|` +
	`\bmust be in (?:the )?\w+ (?:state|status)\b|` +
	`\bimmutable\b|\bread[- ]only\b|\boutput[- ]only\b|\bonly (?:once|one)\b`)

// notes quotes design sentences from operations' documentation, then from the members of inputs, down to specDepth.
func notes(m *smithy.Model, ops, inputs []string) []SpecNote {
	var out []SpecNote
	seen := map[string]bool{}
	add := func(source, doc string) {
		for _, sentence := range sentences(text(doc)) {
			if len(out) >= maxNotes || seen[sentence] || !designRE.MatchString(sentence) {
				continue
			}
			seen[sentence] = true
			out = append(out, SpecNote{Source: source, Text: sentence})
		}
	}
	for _, op := range ops {
		add(op, m.Shapes[op].Doc)
	}
	visited := map[string]bool{}
	var walk func(shape string, depth int)
	walk = func(shape string, depth int) {
		sh, ok := m.Shapes[shape]
		if !ok || visited[shape] || depth > specDepth {
			return
		}
		visited[shape] = true
		for _, name := range sortedNames(sh.Members) {
			if (sh.Type == smithy.TypeStructure || sh.Type == smithy.TypeUnion) && !sh.Info[name].IdempotencyToken &&
				!slices.Contains(pagingMembers, name) {
				add(shape+"."+name, sh.Docs[name])
			}
			t := sh.Members[name]
			walk(t, depth+1)
			for _, e := range m.Shapes[t].Elems {
				walk(e, depth+1)
			}
		}
	}
	for _, in := range inputs {
		walk(in, 1)
	}
	return out
}

var (
	tagRemoveRE = regexp.MustCompile(`<[^>]*>`)
	spacesRE    = regexp.MustCompile(`\s+`)
)

// text turns documentation HTML into plain text.
func text(doc string) string {
	return strings.TrimSpace(spacesRE.ReplaceAllString(html.UnescapeString(tagRemoveRE.ReplaceAllString(doc, " ")), " "))
}

// sentences splits text at a period, question mark, or exclamation mark followed by a space and a capital letter or
// digit, so "e.g. the" and "v1.2" stay whole. Long sentences are cut at 300 characters.
func sentences(t string) []string {
	var out []string
	start := 0
	for i := 0; i+2 < len(t); i++ {
		if (t[i] == '.' || t[i] == '?' || t[i] == '!') && t[i+1] == ' ' && (t[i+2] >= 'A' && t[i+2] <= 'Z' || t[i+2] >= '0' && t[i+2] <= '9') {
			out = append(out, t[start:i+1])
			start = i + 2
		}
	}
	if start < len(t) {
		out = append(out, t[start:])
	}
	for i, s := range out {
		if len(s) > 300 {
			out[i] = s[:297] + "..."
		}
	}
	return out
}

// constraintText describes constraints: "1 to 63 characters", "1 to 20 items", "0 to 3600", "unique".
func constraintText(f Field) string {
	var parts []string
	bounds := func(lo, hi *float64, unit string) {
		switch {
		case lo != nil && hi != nil:
			parts = append(parts, fmt.Sprintf("%s to %s%s", num(*lo), num(*hi), unit))
		case lo != nil:
			parts = append(parts, fmt.Sprintf("at least %s%s", num(*lo), unit))
		case hi != nil:
			parts = append(parts, fmt.Sprintf("up to %s%s", num(*hi), unit))
		}
	}
	c := f.Constraints
	unit := ""
	switch f.Type {
	case "string":
		unit = " characters"
	case "list", "set", "map":
		unit = " items"
	}
	bounds(c.LengthMin, c.LengthMax, unit)
	bounds(c.RangeMin, c.RangeMax, "")
	if c.Unique {
		parts = append(parts, "unique")
	}
	if c.Pattern != "" {
		p := c.Pattern
		if len(p) > 60 {
			p = p[:57] + "..."
		}
		parts = append(parts, "pattern "+p)
	}
	return strings.Join(parts, ", ")
}

// FieldText is an argument in a few words: "required, create-only; string, 1 to 63 characters". For an attribute
// (argument false), required and optional are left out.
func FieldText(f Field, argument bool) string {
	var flags []string
	switch {
	case !argument:
	case f.Required:
		flags = append(flags, "required")
	default:
		flags = append(flags, "optional")
	}
	if f.Ignored {
		flags = append(flags, "documented as ignored on write")
	}
	if f.CreateOnly {
		flags = append(flags, "create-only")
	}
	if f.Deprecated {
		flags = append(flags, "deprecated")
	}
	typ := f.Type
	single := (f.Type == "list" || f.Type == "set") && f.Constraints.LengthMax != nil && *f.Constraints.LengthMax == 1
	switch {
	case single:
		// A list of at most one is a single block.
		typ = "single " + f.Elem
	case f.Elem != "":
		typ += " of " + f.Elem
	}
	if f.Recursive != "" {
		typ += " `" + f.Recursive + "`, recursive"
	}
	desc := []string{typ}
	c := f
	if single {
		c.Constraints.LengthMin, c.Constraints.LengthMax = nil, nil
	}
	if ct := constraintText(c); ct != "" {
		desc = append(desc, ct)
	}
	switch {
	case len(f.Values) > maxValues:
		desc = append(desc, fmt.Sprintf("%s, and %d more", strings.Join(f.Values[:maxValues], " | "), len(f.Values)-maxValues))
	case len(f.Values) > 0:
		desc = append(desc, strings.Join(f.Values, " | "))
	}
	if f.Default != "" {
		desc = append(desc, "default "+f.Default)
	}
	if len(flags) == 0 {
		return strings.Join(desc, ", ")
	}
	return strings.Join(flags, ", ") + "; " + strings.Join(desc, ", ")
}
