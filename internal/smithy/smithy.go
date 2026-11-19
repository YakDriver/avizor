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

// Model is the part of a Smithy JSON AST model Avizor uses.
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
	// Docs are structure and union members' documentation, as written (HTML), for members that have it.
	Docs map[string]string
	// Doc is the shape's own documentation (HTML), as for an operation.
	Doc string
	// Constraints are the shape's own length, range, pattern, and uniqueItems traits.
	Constraints Constraints
	// Info has structure and union members' traits that matter for a resource's schema, for members with any.
	Info map[string]MemberInfo
}

// Constraints are a shape's or member's validation traits. Length applies to strings, lists, and maps; Range to
// numbers.
type Constraints struct {
	LengthMin *float64 `json:"length_min,omitempty"`
	LengthMax *float64 `json:"length_max,omitempty"`
	RangeMin  *float64 `json:"range_min,omitempty"`
	RangeMax  *float64 `json:"range_max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Unique    bool     `json:"unique,omitempty"`
}

// Empty reports whether no constraint is set.
func (c Constraints) Empty() bool {
	return c == Constraints{}
}

// Merge returns c with any constraint o sets. A member's traits override its target's.
func (c Constraints) Merge(o Constraints) Constraints {
	if o.LengthMin != nil || o.LengthMax != nil {
		c.LengthMin, c.LengthMax = o.LengthMin, o.LengthMax
	}
	if o.RangeMin != nil || o.RangeMax != nil {
		c.RangeMin, c.RangeMax = o.RangeMin, o.RangeMax
	}
	if o.Pattern != "" {
		c.Pattern = o.Pattern
	}
	c.Unique = c.Unique || o.Unique
	return c
}

// MemberInfo is a structure or union member's traits.
type MemberInfo struct {
	Required bool `json:"required,omitempty"`
	// IdempotencyToken marks a client token, which a provider generates rather than exposes.
	IdempotencyToken bool `json:"idempotency_token,omitempty"`
	// Default is the member's default value as JSON, if it has one.
	Default     string      `json:"default,omitempty"`
	Deprecated  bool        `json:"deprecated,omitempty"`
	Constraints Constraints `json:"constraints,omitzero"`
}

func (i MemberInfo) empty() bool {
	return i == MemberInfo{}
}

// constraintsOf reads validation traits.
func constraintsOf(traits map[string]json.RawMessage) Constraints {
	var c Constraints
	var bounds struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	}
	if raw, ok := traits["smithy.api#length"]; ok && json.Unmarshal(raw, &bounds) == nil {
		c.LengthMin, c.LengthMax = bounds.Min, bounds.Max
	}
	bounds.Min, bounds.Max = nil, nil
	if raw, ok := traits["smithy.api#range"]; ok && json.Unmarshal(raw, &bounds) == nil {
		c.RangeMin, c.RangeMax = bounds.Min, bounds.Max
	}
	if raw, ok := traits["smithy.api#pattern"]; ok {
		_ = json.Unmarshal(raw, &c.Pattern)
	}
	_, c.Unique = traits["smithy.api#uniqueItems"]
	return c
}

// infoOf reads a member's traits.
func infoOf(traits map[string]json.RawMessage) MemberInfo {
	i := MemberInfo{Constraints: constraintsOf(traits)}
	_, i.Required = traits["smithy.api#required"]
	_, i.IdempotencyToken = traits["smithy.api#idempotencyToken"]
	_, i.Deprecated = traits["smithy.api#deprecated"]
	if raw, ok := traits["smithy.api#default"]; ok {
		i.Default = string(raw)
	}
	return i
}

// docOf returns a documentation trait, or "".
func docOf(traits map[string]json.RawMessage) string {
	var doc string
	if raw, ok := traits["smithy.api#documentation"]; ok {
		_ = json.Unmarshal(raw, &doc)
	}
	return doc
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

	// Service and resource bindings, and other references, for the service closure.
	Operations           []astMember          `json:"operations"`
	Resources            []astMember          `json:"resources"`
	Errors               []astMember          `json:"errors"`
	CollectionOperations []astMember          `json:"collectionOperations"`
	Mixins               []astMember          `json:"mixins"`
	Identifiers          map[string]astMember `json:"identifiers"`
	Properties           map[string]astMember `json:"properties"`
	Create               *astMember           `json:"create"`
	Put                  *astMember           `json:"put"`
	Read                 *astMember           `json:"read"`
	Update               *astMember           `json:"update"`
	Delete               *astMember           `json:"delete"`
	List                 *astMember           `json:"list"`
}

// refs returns the shape IDs a shape refers to.
func (s *astShape) refs() []string {
	var out []string
	add := func(ms ...*astMember) {
		for _, m := range ms {
			if m != nil {
				out = append(out, m.Target)
			}
		}
	}
	add(s.Input, s.Output, s.Member, s.Key, s.Value, s.Create, s.Put, s.Read, s.Update, s.Delete, s.List)
	for _, list := range [][]astMember{s.Operations, s.Resources, s.Errors, s.CollectionOperations, s.Mixins} {
		for i := range list {
			add(&list[i])
		}
	}
	for _, m := range []map[string]astMember{s.Members, s.Identifiers, s.Properties} {
		for _, k := range sortedKeys(m) {
			mem := m[k]
			add(&mem)
		}
	}
	return out
}

// closure returns the IDs of the shapes the model's service shapes reach, which are the shapes the SDK is generated
// from. Shapes outside it, such as another namespace's operations no service binds, aren't part of the API.
func (ast *astModel) closure() map[string]bool {
	seen := map[string]bool{}
	var stack []string
	for _, id := range sortedKeys(ast.Shapes) {
		if ast.Shapes[id].Type == TypeService {
			stack = append(stack, id)
		}
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		s, ok := ast.Shapes[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, s.refs()...)
	}
	return seen
}

type astMember struct {
	Target string                     `json:"target"`
	Traits map[string]json.RawMessage `json:"traits"`
}

// Parse reads a Smithy JSON AST model, keying shapes by their name after "#". It keeps the shapes in the service's
// namespace, and the shapes of other namespaces that the service closure reaches. The SDK is generated from the
// closure, and Smithy requires shape names in it to be unique. Other namespaces' unreachable shapes, such as
// operations no service binds, aren't part of the API, and their names can collide with the service's. Should names
// still collide, the service's namespace wins, then the first shape ID in sorted order, so parsing is deterministic.
func Parse(data []byte) (*Model, error) {
	var ast astModel
	if err := json.Unmarshal(data, &ast); err != nil {
		return nil, fmt.Errorf("parsing Smithy model: %w", err)
	}
	inClosure := ast.closure()
	own := map[string]bool{}
	for id, s := range ast.Shapes {
		if s.Type == TypeService {
			own[namespace(id)] = true
		}
	}
	ids := sortedKeys(ast.Shapes)
	slices.SortStableFunc(ids, func(a, b string) int {
		return cmpBool(!own[namespace(a)], !own[namespace(b)])
	})
	m := &Model{Shapes: make(map[string]Shape, len(ast.Shapes))}
	for _, id := range ids {
		if !own[namespace(id)] && !inClosure[id] {
			continue
		}
		s := ast.Shapes[id]
		name := shortName(id)
		if _, dup := m.Shapes[name]; dup {
			continue
		}
		sh := Shape{Type: s.Type, Doc: docOf(s.Traits), Constraints: constraintsOf(s.Traits)}
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
				if s.Type != TypeStructure && s.Type != TypeUnion {
					continue
				}
				if doc := docOf(mem.Traits); doc != "" {
					if sh.Docs == nil {
						sh.Docs = map[string]string{}
					}
					sh.Docs[mn] = doc
				}
				if info := infoOf(mem.Traits); !info.empty() {
					if sh.Info == nil {
						sh.Info = map[string]MemberInfo{}
					}
					sh.Info[mn] = info
				}
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

// TypeOf returns the type of a shape by short name, from the model or the prelude, or "" if it's in neither.
func (m *Model) TypeOf(name string) string {
	return m.typeOf(name)
}

// typeOf returns the type of a shape by short name, from the model or the prelude.
func (m *Model) typeOf(name string) string {
	if s, ok := m.Shapes[name]; ok {
		return s.Type
	}
	return preludeTypes[name]
}

func namespace(id string) string {
	ns, _, _ := strings.Cut(id, "#")
	return ns
}

// cmpBool orders false before true.
func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
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
