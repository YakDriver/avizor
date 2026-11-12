// Package smithy reads Smithy JSON AST models and compares two versions of a model's API surface.
package smithy

import (
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
}

type astModel struct {
	Shapes map[string]astShape `json:"shapes"`
}

type astShape struct {
	Type    string                     `json:"type"`
	Members map[string]astMember       `json:"members"`
	Traits  map[string]json.RawMessage `json:"traits"`
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
			return strings.Trim(string(raw), `"`)
		}
		return name
	default:
		return shortName(mem.Target)
	}
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
}

// ShapeRef names a shape and its type.
type ShapeRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MemberChange is a member added to or removed from an existing shape. For enums, Value is the enum value; otherwise
// it's the member's target shape.
type MemberChange struct {
	Shape  string `json:"shape"`
	Member string `json:"member"`
	Value  string `json:"value,omitempty"`
}

// Empty reports whether the diff has no API surface change.
func (d *Diff) Empty() bool {
	return d.Count() == 0
}

// Count is the number of changes.
func (d *Diff) Count() int {
	return len(d.AddedOperations) + len(d.RemovedOperations) + len(d.AddedShapes) + len(d.RemovedShapes) +
		len(d.AddedMembers) + len(d.RemovedMembers) + len(d.AddedEnumValues) + len(d.RemovedEnumValues)
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

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
