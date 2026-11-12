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
	check("AddedMembers", d.AddedMembers, []MemberChange{{"CreateWidgetRequest", "Size", "Size"}})
	check("RemovedMembers", d.RemovedMembers, []MemberChange{{"CreateWidgetRequest", "Legacy", "String"}})
	check("AddedEnumValues", d.AddedEnumValues, []MemberChange{{"Color", "BLUE", "blue"}})
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
