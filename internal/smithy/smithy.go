// Package smithy reads Smithy JSON AST models and compares two versions of a model's API surface.
package smithy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Shape types.
const (
	TypeService   = "service"
	TypeOperation = "operation"
	TypeStructure = "structure"
	TypeUnion     = "union"
	TypeEnum      = "enum"
	TypeIntEnum   = "intEnum"
)

// Model is the part of a Smithy JSON AST model Radar uses.
type Model struct {
	// SDKID is the service's SDK ID, e.g. "FSx" or "S3 Control".
	SDKID string
	// Shapes are keyed by short name (without namespace).
	Shapes map[string]Shape
}

// Shape is one shape. Members are keyed by member name. For enums, Members map names to enum values.
type Shape struct {
	Type    string
	Members map[string]string
	// Input and Output are an operation's input and output shapes.
	Input, Output string
	// Elems are a list's member target and a map's key and value targets.
	Elems []string
}

type astModel struct {
	Shapes map[string]astShape `json:"shapes"`
}

type astShape struct {
	Type    string                     `json:"type"`
	Members map[string]astMember       `json:"members"`
	Traits  map[string]json.RawMessage `json:"traits"`
	Input   *astMember                 `json:"input"`
	Output  *astMember                 `json:"output"`
	Member  *astMember                 `json:"member"`
	Key     *astMember                 `json:"key"`
	Value   *astMember                 `json:"value"`
}

type astMember struct {
	Target string                     `json:"target"`
	Traits map[string]json.RawMessage `json:"traits"`
}

// Parse reads a Smithy JSON AST model. Shapes from other namespaces (prelude references) aren't in the file, so all
// shapes are keyed by their name after "#".
func Parse(data []byte) (*Model, error) {
	var ast astModel
	if err := json.Unmarshal(data, &ast); err != nil {
		return nil, fmt.Errorf("parsing Smithy model: %w", err)
	}
	m := &Model{Shapes: make(map[string]Shape, len(ast.Shapes))}
	for id, s := range ast.Shapes {
		name := shortName(id)
		sh := Shape{Type: s.Type}
		if s.Input != nil {
			sh.Input = shortName(s.Input.Target)
		}
		if s.Output != nil {
			sh.Output = shortName(s.Output.Target)
		}
		for _, e := range []*astMember{s.Member, s.Key, s.Value} {
			if e != nil {
				sh.Elems = append(sh.Elems, shortName(e.Target))
			}
		}
		if len(s.Members) > 0 {
			sh.Members = make(map[string]string, len(s.Members))
			for mn, mem := range s.Members {
				sh.Members[mn] = memberValue(s.Type, mn, mem)
			}
		}
		m.Shapes[name] = sh

		if s.Type == TypeService {
			if raw, ok := s.Traits["aws.api#service"]; ok {
				var svc struct {
					SDKID string `json:"sdkId"`
				}
				if err := json.Unmarshal(raw, &svc); err == nil {
					m.SDKID = svc.SDKID
				}
			}
		}
	}
	if m.SDKID == "" {
		return nil, fmt.Errorf("parsing Smithy model: no service shape with an aws.api#service sdkId")
	}
	return m, nil
}

// memberValue is what identifies a member for comparison: an enum's value, or a structure member's target.
func memberValue(shapeType, name string, mem astMember) string {
	switch shapeType {
	case TypeEnum, TypeIntEnum:
		if raw, ok := mem.Traits["smithy.api#enumValue"]; ok {
			if v, ok := decodeEnumValue(raw); ok {
				return v
			}
		}
		return name
	default:
		return shortName(mem.Target)
	}
}

// decodeEnumValue decodes a string enum value (unescaping it) or an intEnum's number, kept as written.
func decodeEnumValue(raw json.RawMessage) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	}
	return "", false
}

// preludeTypes maps Smithy prelude shape names to their types, for members that target them.
var preludeTypes = map[string]string{
	"Blob": "blob", "Boolean": "boolean", "String": "string", "Byte": "byte", "Short": "short", "Integer": "integer",
	"Long": "long", "Float": "float", "Double": "double", "BigInteger": "bigInteger", "BigDecimal": "bigDecimal",
	"Timestamp": "timestamp", "Document": "document", "PrimitiveBoolean": "boolean", "PrimitiveByte": "byte",
	"PrimitiveShort": "short", "PrimitiveInteger": "integer", "PrimitiveLong": "long", "PrimitiveFloat": "float",
	"PrimitiveDouble": "double", "Unit": "structure",
}

// typeOf returns the type of a shape by short name, from the model or the prelude.
func (m *Model) typeOf(name string) string {
	if s, ok := m.Shapes[name]; ok {
		return s.Type
	}
	return preludeTypes[name]
}

func shortName(id string) string {
	if _, n, ok := strings.Cut(id, "#"); ok {
		return n
	}
	return id
}

// Diff is the API surface change between two versions of a model. Documentation and trait-only changes, such as a
// new protocol, aren't included.
type Diff struct {
	AddedOperations   []string       `json:"added_operations,omitempty"`
	RemovedOperations []string       `json:"removed_operations,omitempty"`
	AddedShapes       []ShapeRef     `json:"added_shapes,omitempty"`
	RemovedShapes     []ShapeRef     `json:"removed_shapes,omitempty"`
	AddedMembers      []MemberChange `json:"added_members,omitempty"`
	RemovedMembers    []MemberChange `json:"removed_members,omitempty"`
	AddedEnumValues   []MemberChange `json:"added_enum_values,omitempty"`
	RemovedEnumValues []MemberChange `json:"removed_enum_values,omitempty"`
	// ChangedMembers are existing members whose target changed to a shape of a different type, such as string to
	// integer. ChangedEnumValues are enum members whose value changed.
	ChangedMembers    []MemberChange `json:"changed_members,omitempty"`
	ChangedEnumValues []MemberChange `json:"changed_enum_values,omitempty"`
	// RetargetedMembers are existing members whose target changed to another shape of the same type, such as one
	// string shape to another. The value's type is unchanged, though its constraints (length, range, pattern) may
	// differ. They're evidence, not API changes: they don't count toward Count or Empty.
	RetargetedMembers []MemberChange `json:"retargeted_members,omitempty"`
}

// ShapeRef names a shape and its type.
type ShapeRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MemberChange is a member added to or removed from an existing shape. For enums, Value is the enum value; otherwise
// it's the member's target shape. OldValue is set for changed members.
type MemberChange struct {
	Shape    string `json:"shape"`
	Member   string `json:"member"`
	Value    string `json:"value,omitempty"`
	OldValue string `json:"old_value,omitempty"`
}

// Empty reports whether the diff has no API surface change. Retargeted members alone are not an API change.
func (d *Diff) Empty() bool {
	return d.Count() == 0
}

// Count is the number of changes.
func (d *Diff) Count() int {
	return len(d.AddedOperations) + len(d.RemovedOperations) + len(d.AddedShapes) + len(d.RemovedShapes) +
		len(d.AddedMembers) + len(d.RemovedMembers) + len(d.AddedEnumValues) + len(d.RemovedEnumValues) +
		len(d.ChangedMembers) + len(d.ChangedEnumValues)
}

// Compare returns the API surface change from old to new. old may be nil for a new service.
func Compare(old, new *Model) *Diff {
	d := &Diff{}
	var oldShapes map[string]Shape
	if old != nil {
		oldShapes = old.Shapes
	}

	for _, name := range sortedKeys(new.Shapes) {
		ns := new.Shapes[name]
		os, existed := oldShapes[name]
		if ns.Type == TypeService {
			continue
		}
		if !existed || os.Type != ns.Type {
			if ns.Type == TypeOperation {
				d.AddedOperations = append(d.AddedOperations, name)
			} else {
				d.AddedShapes = append(d.AddedShapes, ShapeRef{Name: name, Type: ns.Type})
			}
			continue
		}
		added, removed := memberDiff(os.Members, ns.Members)
		for _, m := range added {
			c := MemberChange{Shape: name, Member: m, Value: ns.Members[m]}
			if isEnum(ns.Type) {
				d.AddedEnumValues = append(d.AddedEnumValues, c)
			} else if ns.Type != TypeOperation {
				d.AddedMembers = append(d.AddedMembers, c)
			}
		}
		for _, m := range changed(os.Members, ns.Members) {
			c := MemberChange{Shape: name, Member: m, Value: ns.Members[m], OldValue: os.Members[m]}
			switch {
			case isEnum(ns.Type):
				d.ChangedEnumValues = append(d.ChangedEnumValues, c)
			case ns.Type == TypeOperation:
			case old.typeOf(c.OldValue) == new.typeOf(c.Value) && new.typeOf(c.Value) != "":
				d.RetargetedMembers = append(d.RetargetedMembers, c)
			default:
				d.ChangedMembers = append(d.ChangedMembers, c)
			}
		}
		for _, m := range removed {
			c := MemberChange{Shape: name, Member: m, Value: os.Members[m]}
			if isEnum(ns.Type) {
				d.RemovedEnumValues = append(d.RemovedEnumValues, c)
			} else if ns.Type != TypeOperation {
				d.RemovedMembers = append(d.RemovedMembers, c)
			}
		}
	}
	for _, name := range sortedKeys(oldShapes) {
		os := oldShapes[name]
		if os.Type == TypeService {
			continue
		}
		if ns, ok := new.Shapes[name]; ok && ns.Type == os.Type {
			continue
		}
		if os.Type == TypeOperation {
			d.RemovedOperations = append(d.RemovedOperations, name)
		} else {
			d.RemovedShapes = append(d.RemovedShapes, ShapeRef{Name: name, Type: os.Type})
		}
	}
	return d
}

// Operations returns the model's operation names, sorted.
func (m *Model) Operations() []string {
	var out []string
	for name, s := range m.Shapes {
		if s.Type == TypeOperation {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Usage is how operations reach a shape: through their input, their output, or both.
type Usage struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

// Usages returns, for each shape reachable from an operation's input or output, the operations that reach it, sorted.
// Structure and union members and list and map elements are followed.
func (m *Model) Usages() map[string]*Usage {
	out := map[string]*Usage{}
	for _, op := range m.Operations() {
		s := m.Shapes[op]
		for _, root := range []struct {
			name  string
			input bool
		}{{s.Input, true}, {s.Output, false}} {
			if root.name == "" {
				continue
			}
			for name := range m.reach(root.name) {
				u := out[name]
				if u == nil {
					u = &Usage{}
					out[name] = u
				}
				if root.input {
					u.Input = append(u.Input, op)
				} else {
					u.Output = append(u.Output, op)
				}
			}
		}
	}
	for _, u := range out {
		u.Input = slices.Compact(u.Input)
		u.Output = slices.Compact(u.Output)
	}
	return out
}

// reach returns the model shapes reachable from name, including name.
func (m *Model) reach(name string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{name}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		s, ok := m.Shapes[n]
		if !ok || seen[n] {
			continue
		}
		seen[n] = true
		if s.Type == TypeStructure || s.Type == TypeUnion {
			for _, t := range s.Members {
				stack = append(stack, t)
			}
		}
		stack = append(stack, s.Elems...)
	}
	return seen
}

func isEnum(t string) bool {
	return t == TypeEnum || t == TypeIntEnum
}

func memberDiff(old, new map[string]string) (added, removed []string) {
	for _, k := range sortedKeys(new) {
		if _, ok := old[k]; !ok {
			added = append(added, k)
		}
	}
	for _, k := range sortedKeys(old) {
		if _, ok := new[k]; !ok {
			removed = append(removed, k)
		}
	}
	return added, removed
}

// changed returns members present in both with different values.
func changed(old, new map[string]string) []string {
	var out []string
	for _, k := range sortedKeys(new) {
		if v, ok := old[k]; ok && v != new[k] {
			out = append(out, k)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
