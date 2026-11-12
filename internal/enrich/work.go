package enrich

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/YakDriver/radar/internal/smithy"
)

// Verbs that, on a new operation, suggest a resource's lifecycle or a read.
var (
	createVerbs = []string{"Create", "Put"}
	deleteVerbs = []string{"Delete"}
	readVerbs   = []string{"Get", "Describe", "List", "BatchGet"}
	// pairedVerbs suggest a resource only together, as Terraform association and attachment resources do.
	pairedVerbs = [][2]string{{"Register", "Deregister"}, {"Associate", "Disassociate"}, {"Attach", "Detach"}, {"Enable", "Disable"}}
)

// work infers kinds of provider work from a diff. usages, if not nil, says which operations reach each shape.
func work(d *smithy.Diff, usages map[string]*smithy.Usage) []Work {
	out := []Work{}
	if d == nil {
		return out
	}
	out = append(out, operationWork(d.AddedOperations)...)

	for _, m := range d.AddedMembers {
		w := Work{Subject: m.Shape + "." + m.Member}
		u := usages[m.Shape]
		switch {
		case usages == nil:
			w.Kind, w.Basis = WorkNewArgument, "new member of an existing shape; the model wasn't loaded, so argument vs. attribute is unknown"
		case u == nil:
			// Errors and shapes used only by new operations aren't reached from an existing operation.
			continue
		case len(u.Input) > 0:
			w.Kind, w.Operations = WorkNewArgument, u.Input
			w.Basis = "new member of a shape in the input of " + list(u.Input)
		default:
			w.Kind, w.Operations = WorkNewAttribute, u.Output
			w.Basis = "new member of a shape only in the output of " + list(u.Output)
		}
		if w.Kind != "" && !newOnly(w.Operations, d.AddedOperations) {
			out = append(out, w)
		}
	}

	// Enum values are grouped by enum: the provider validates or documents the set, not each value.
	byEnum := map[string][]string{}
	var enums []string
	for _, m := range d.AddedEnumValues {
		if _, ok := byEnum[m.Shape]; !ok {
			enums = append(enums, m.Shape)
		}
		byEnum[m.Shape] = append(byEnum[m.Shape], m.Value)
	}
	for _, e := range enums {
		w := Work{Kind: WorkNewEnumValue, Subject: e,
			Basis: fmt.Sprintf("new values %s; usually no change if the provider validates with the SDK's enum type", list(byEnum[e]))}
		if u := usages[e]; u != nil {
			w.Operations = slices.Concat(u.Input, u.Output)
			slices.Sort(w.Operations)
			w.Operations = slices.Compact(w.Operations)
		} else if usages != nil {
			continue
		}
		if !newOnly(w.Operations, d.AddedOperations) {
			out = append(out, w)
		}
	}

	for _, m := range d.ChangedMembers {
		w := Work{Kind: WorkTypeChange, Subject: m.Shape + "." + m.Member,
			Basis: fmt.Sprintf("member type changed from %s to %s", m.OldValue, m.Value)}
		if u := usages[m.Shape]; u != nil {
			w.Operations = slices.Concat(u.Input, u.Output)
			slices.Sort(w.Operations)
			w.Operations = slices.Compact(w.Operations)
		}
		out = append(out, w)
	}
	for _, op := range d.RemovedOperations {
		out = append(out, Work{Kind: WorkRemoval, Subject: op, Operations: []string{op}, Basis: "operation removed"})
	}
	for _, m := range d.RemovedMembers {
		out = append(out, Work{Kind: WorkRemoval, Subject: m.Shape + "." + m.Member, Basis: "member removed"})
	}
	return out
}

// newOnly reports whether every operation is new, so the member or value is part of new operations' work.
func newOnly(ops, added []string) bool {
	if len(ops) == 0 {
		return false
	}
	for _, op := range ops {
		if !slices.Contains(added, op) {
			return false
		}
	}
	return true
}

// operationWork groups new operations by noun and infers a resource, data source, or other operation for each group.
func operationWork(ops []string) []Work {
	groups := map[string][]string{}
	var nouns []string
	for _, op := range ops {
		_, noun := splitVerb(op)
		noun = singular(noun)
		if _, ok := groups[noun]; !ok {
			nouns = append(nouns, noun)
		}
		groups[noun] = append(groups[noun], op)
	}
	var out []Work
	for _, noun := range nouns {
		g := groups[noun]
		has := func(verbs []string) bool {
			return slices.ContainsFunc(g, func(op string) bool { v, _ := splitVerb(op); return slices.Contains(verbs, v) })
		}
		w := Work{Subject: noun, Operations: g}
		pair := slices.IndexFunc(pairedVerbs, func(p [2]string) bool { return has(p[:1]) && has(p[1:]) })
		switch {
		case has(createVerbs) && has(deleteVerbs):
			w.Kind, w.Basis = WorkNewResource, "new create and delete operations for "+noun
		case has(createVerbs):
			w.Kind, w.Basis = WorkNewResource, "new create operation for "+noun+", without a delete"
		case pair >= 0:
			w.Kind, w.Basis = WorkNewResource, fmt.Sprintf("new %s and %s operations for %s", pairedVerbs[pair][0], pairedVerbs[pair][1], noun)
		case !slices.ContainsFunc(g, func(op string) bool { v, _ := splitVerb(op); return !slices.Contains(readVerbs, v) }):
			w.Kind, w.Basis = WorkNewDataSource, "only new read operations for "+noun
		default:
			w.Kind, w.Basis = WorkNewOperation, "new operations for "+noun+" that aren't create, delete, or read"
		}
		out = append(out, w)
	}
	return out
}

// splitVerb splits an operation name at its first word: CreateWidget -> Create, Widget. BatchGet is one verb.
func splitVerb(op string) (string, string) {
	if rest, ok := strings.CutPrefix(op, "BatchGet"); ok {
		return "BatchGet", rest
	}
	for i, r := range op {
		if i > 0 && unicode.IsUpper(r) {
			return op[:i], op[i:]
		}
	}
	return op, ""
}

// singular strips a plural "s" so ListWidgets groups with CreateWidget. Names ending in "ss" or "us" are kept.
func singular(noun string) string {
	if before, ok := strings.CutSuffix(noun, "ies"); ok {
		return before + "y"
	}
	if base, ok := strings.CutSuffix(noun, "yses"); ok {
		return base + "ysis"
	}
	// Aliases, Statuses, Addresses, Boxes: the "es" is the plural.
	if base, ok := strings.CutSuffix(noun, "es"); ok {
		for _, end := range []string{"ss", "us", "as", "x", "ch", "sh"} {
			if strings.HasSuffix(base, end) {
				return base
			}
		}
	}
	if strings.HasSuffix(noun, "s") && !strings.HasSuffix(noun, "ss") && !strings.HasSuffix(noun, "us") {
		return strings.TrimSuffix(noun, "s")
	}
	return noun
}

func list(ss []string) string {
	const max = 5
	if len(ss) > max {
		return strings.Join(ss[:max], ", ") + fmt.Sprintf(", and %d more", len(ss)-max)
	}
	return strings.Join(ss, ", ")
}
