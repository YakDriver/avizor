package smithy

import (
	"slices"
	"testing"
)

const oldModel = `{
  "smithy": "2.0",
  "shapes": {
    "com.amazonaws.widget#WidgetService": {
      "type": "service",
      "traits": {"aws.api#service": {"sdkId": "Widget"}, "aws.protocols#awsJson1_1": {}}
    },
    "com.amazonaws.widget#CreateWidget": {"type": "operation", "input": {"target": "com.amazonaws.widget#CreateWidgetRequest"}},
    "com.amazonaws.widget#DeleteWidget": {"type": "operation"},
    "com.amazonaws.widget#CreateWidgetRequest": {
      "type": "structure",
      "members": {"Name": {"target": "smithy.api#String"}, "Legacy": {"target": "smithy.api#String"}}
    },
    "com.amazonaws.widget#Color": {
      "type": "enum",
      "members": {"RED": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "red"}}}
    },
    "com.amazonaws.widget#Gone": {"type": "string"}
  }
}`

const newModel = `{
  "smithy": "2.0",
  "shapes": {
    "com.amazonaws.widget#WidgetService": {
      "type": "service",
      "traits": {"aws.api#service": {"sdkId": "Widget"}, "smithy.protocols#rpcv2Cbor": {}}
    },
    "com.amazonaws.widget#CreateWidget": {"type": "operation", "input": {"target": "com.amazonaws.widget#CreateWidgetRequest"},
      "traits": {"smithy.api#documentation": "changed docs"}},
    "com.amazonaws.widget#TagWidget": {"type": "operation"},
    "com.amazonaws.widget#CreateWidgetRequest": {
      "type": "structure",
      "members": {"Name": {"target": "smithy.api#String"}, "Size": {"target": "com.amazonaws.widget#Size"}}
    },
    "com.amazonaws.widget#Size": {"type": "integer"},
    "com.amazonaws.widget#Color": {
      "type": "enum",
      "members": {
        "RED": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "red"}},
        "BLUE": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "blue"}}
      }
    }
  }
}`

func TestCompare(t *testing.T) {
	t.Parallel()

	o, err := Parse([]byte(oldModel))
	if err != nil {
		t.Fatal(err)
	}
	n, err := Parse([]byte(newModel))
	if err != nil {
		t.Fatal(err)
	}
	if n.SDKID != "Widget" {
		t.Errorf("SDKID = %q", n.SDKID)
	}
	if got := n.Operations(); !slices.Equal(got, []string{"CreateWidget", "TagWidget"}) {
		t.Errorf("Operations = %v", got)
	}

	d := Compare(o, n)
	check := func(name string, got, want any) {
		t.Helper()
		if !equalJSONish(got, want) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("AddedOperations", d.AddedOperations, []string{"TagWidget"})
	check("RemovedOperations", d.RemovedOperations, []string{"DeleteWidget"})
	check("AddedShapes", d.AddedShapes, []ShapeRef{{"Size", "integer"}})
	check("RemovedShapes", d.RemovedShapes, []ShapeRef{{"Gone", "string"}})
	check("AddedMembers", d.AddedMembers, []MemberChange{{Shape: "CreateWidgetRequest", Member: "Size", Value: "Size"}})
	check("RemovedMembers", d.RemovedMembers, []MemberChange{{Shape: "CreateWidgetRequest", Member: "Legacy", Value: "String"}})
	check("AddedEnumValues", d.AddedEnumValues, []MemberChange{{Shape: "Color", Member: "BLUE", Value: "blue"}})
	if d.Count() != 7 {
		t.Errorf("Count = %d, want 7", d.Count())
	}

	// Trait and documentation changes alone are no API change.
	if d := Compare(o, o); !d.Empty() {
		t.Errorf("self-compare = %+v", d)
	}

	// A new service: everything is added.
	if d := Compare(nil, o); len(d.AddedOperations) != 2 || len(d.AddedShapes) != 3 {
		t.Errorf("new service diff = %+v", d)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte(`{"shapes": {}}`)); err == nil {
		t.Error("expected error for a model without a service")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func equalJSONish(a, b any) bool {
	switch x := a.(type) {
	case []string:
		return slices.Equal(x, b.([]string))
	case []ShapeRef:
		return slices.Equal(x, b.([]ShapeRef))
	case []MemberChange:
		return slices.Equal(x, b.([]MemberChange))
	}
	return false
}

// TestCompareChangedMembers checks that a member whose target or value changes is reported, with the old value, and that
// a change to another shape of the same type is recorded but not counted as an API change.
func TestCompareChangedMembers(t *testing.T) {
	t.Parallel()

	model := func(nameTarget, sizeTarget, red string) *Model {
		t.Helper()
		m, err := Parse([]byte(`{"shapes": {
  "com.amazonaws.w#Svc": {"type": "service", "traits": {"aws.api#service": {"sdkId": "W"}}},
  "com.amazonaws.w#Req": {"type": "structure", "members": {
    "Name": {"target": "` + nameTarget + `"},
    "Size": {"target": "` + sizeTarget + `"}
  }},
  "com.amazonaws.w#ShortName": {"type": "string"},
  "com.amazonaws.w#LongName": {"type": "string"},
  "com.amazonaws.w#Color": {"type": "enum", "members": {"RED": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "` + red + `"}}}}
}}`))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	// String to Integer is a type change; one string shape to another is a retarget.
	d := Compare(model("com.amazonaws.w#ShortName", "smithy.api#String", "red"), model("com.amazonaws.w#LongName", "smithy.api#Integer", "rouge"))
	if !slices.Equal(d.ChangedMembers, []MemberChange{{Shape: "Req", Member: "Size", Value: "Integer", OldValue: "String"}}) {
		t.Errorf("ChangedMembers = %+v", d.ChangedMembers)
	}
	if !slices.Equal(d.ChangedEnumValues, []MemberChange{{Shape: "Color", Member: "RED", Value: "rouge", OldValue: "red"}}) {
		t.Errorf("ChangedEnumValues = %+v", d.ChangedEnumValues)
	}
	if !slices.Equal(d.RetargetedMembers, []MemberChange{{Shape: "Req", Member: "Name", Value: "LongName", OldValue: "ShortName"}}) {
		t.Errorf("RetargetedMembers = %+v", d.RetargetedMembers)
	}
	if d.Count() != 2 {
		t.Errorf("Count = %d, want 2 (retargets don't count)", d.Count())
	}

	// A retarget alone is no API change.
	d = Compare(model("com.amazonaws.w#ShortName", "smithy.api#String", "red"), model("com.amazonaws.w#LongName", "smithy.api#String", "red"))
	if !d.Empty() || len(d.RetargetedMembers) != 1 {
		t.Errorf("retarget-only diff = %+v", d)
	}
}

func TestEnumValueDecoding(t *testing.T) {
	t.Parallel()

	m, err := Parse([]byte(`{"shapes": {
  "com.amazonaws.w#Svc": {"type": "service", "traits": {"aws.api#service": {"sdkId": "W"}}},
  "com.amazonaws.w#E": {"type": "enum", "members": {
    "SPACE": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "A\u0020B"}},
    "QUOTE": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "say \"hi\" \\ bye"}},
    "NONE": {"target": "smithy.api#Unit"}
  }},
  "com.amazonaws.w#I": {"type": "intEnum", "members": {
    "ONE": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": 1}}
  }}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	for shape, want := range map[string]map[string]string{
		"E": {"SPACE": "A B", "QUOTE": `say "hi" \ bye`, "NONE": "NONE"},
		"I": {"ONE": "1"},
	} {
		for member, v := range want {
			if got := m.Shapes[shape].Members[member]; got != v {
				t.Errorf("%s.%s = %q, want %q", shape, member, got, v)
			}
		}
	}
}
