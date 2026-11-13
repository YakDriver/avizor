package enrich

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/YakDriver/avizor/internal/smithy"
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

	// existing returns the shape's usages by operations that existed before the change. A shape reached only by new
	// operations is part of their work.
	existing := func(shape string) *smithy.Usage {
		u := usages[shape]
		if u == nil {
			return nil
		}
		keep := func(ops []string) []string {
			return slices.DeleteFunc(slices.Clone(ops), func(op string) bool { return slices.Contains(d.AddedOperations, op) })
		}
		e := &smithy.Usage{Input: keep(u.Input), Output: keep(u.Output)}
		if len(e.Input)+len(e.Output) == 0 {
			return nil
		}
		return e
	}
	all := func(u *smithy.Usage) []string {
		if u == nil {
			return nil
		}
		ops := slices.Concat(u.Input, u.Output)
		slices.Sort(ops)
		return slices.Compact(ops)
	}

	for _, m := range d.AddedMembers {
		w := Work{Subject: m.Shape + "." + m.Member}
		u := existing(m.Shape)
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
		out = append(out, w)
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
		u := existing(e)
		if u == nil && usages != nil {
			continue
		}
		w.Operations = all(u)
		out = append(out, w)
	}

	for _, m := range d.ChangedMembers {
		w := Work{Kind: WorkTypeChange, Subject: m.Shape + "." + m.Member,
			Basis: fmt.Sprintf("member type changed from %s to %s", m.OldValue, m.Value)}
		w.Operations = all(existing(m.Shape))
		out = append(out, w)
	}
	for _, op := range d.RemovedOperations {
		out = append(out, Work{Kind: WorkRemoval, Subject: op, Operations: []string{op}, Basis: "operation removed"})
	}
	// Removed members and enum values: the shape is in the new model only if it survived, so its usages there are the
	// operations still affected.
	for _, m := range d.RemovedMembers {
		out = append(out, Work{Kind: WorkRemoval, Subject: m.Shape + "." + m.Member, Operations: all(existing(m.Shape)), Basis: "member removed"})
	}
	byEnum, enums = map[string][]string{}, nil
	for _, m := range d.RemovedEnumValues {
		if _, ok := byEnum[m.Shape]; !ok {
			enums = append(enums, m.Shape)
		}
		byEnum[m.Shape] = append(byEnum[m.Shape], m.Value)
	}
	for _, e := range enums {
		out = append(out, Work{Kind: WorkRemoval, Subject: e, Operations: all(existing(e)), Basis: "enum values removed: " + list(byEnum[e])})
	}
	return out
}

// taggingOperations are the standard AWS tagging operations.
var taggingOperations = []string{"TagResource", "UntagResource", "ListTagsForResource", "TagResources", "UntagResources", "ListTagsForResources"}

// pluralVerbs take a plural noun: ListWidgets, BatchGetWidgets.
var pluralVerbs = []string{"List", "Describe", "BatchGet", "Search"}

// operationWork groups new operations by noun and infers a resource, data source, or other operation for each group.
func operationWork(ops []string) []Work {
	// Tagging operations add tag support to the service's resources, not a new type.
	var tagging []string
	ops = slices.DeleteFunc(slices.Clone(ops), func(op string) bool {
		if slices.Contains(taggingOperations, op) {
			tagging = append(tagging, op)
			return true
		}
		return false
	})
	// Nouns of operations that take a singular noun, so a plural can be matched to one of them exactly.
	singulars := map[string]bool{}
	for _, op := range ops {
		if v, noun := splitVerb(op); !slices.Contains(pluralVerbs, v) {
			singulars[noun] = true
		}
	}
	groups := map[string][]string{}
	var nouns []string
	for _, op := range ops {
		v, noun := splitVerb(op)
		if slices.Contains(pluralVerbs, v) {
			noun = singularOf(noun, singulars)
		}
		if _, ok := groups[noun]; !ok {
			nouns = append(nouns, noun)
		}
		groups[noun] = append(groups[noun], op)
	}
	var out []Work
	if len(tagging) > 0 {
		out = append(out, Work{Kind: WorkTagging, Subject: "tags", Operations: tagging, Basis: "new tagging operations"})
	}
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

// singularOf returns the singular of a plural noun: the matching noun of another new operation if there is one
// (ListAliases with CreateAlias, ListSchemas with CreateSchema), otherwise a guess.
func singularOf(noun string, singulars map[string]bool) string {
	cands := []string{noun}
	if base, ok := strings.CutSuffix(noun, "ies"); ok {
		cands = append(cands, base+"y")
	}
	if base, ok := strings.CutSuffix(noun, "es"); ok {
		cands = append(cands, base)
	}
	if base, ok := strings.CutSuffix(noun, "s"); ok {
		cands = append(cands, base)
	}
	for _, c := range cands {
		if singulars[c] {
			return c
		}
	}
	return singular(noun)
}

// singular guesses the singular of a plural noun, for read operations with no singular counterpart. Names ending in
// "ss", "us", "is", or "as" are kept: Access, Status, Analysis, Alias. That leaves some plurals (Schemas) unchanged,
// which is better than mangling a singular.
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
	if strings.HasSuffix(noun, "s") && !strings.HasSuffix(noun, "ss") && !strings.HasSuffix(noun, "us") && !strings.HasSuffix(noun, "is") && !strings.HasSuffix(noun, "as") {
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
