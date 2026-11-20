package enrich

import (
	"slices"
	"strings"
	"testing"

	"github.com/YakDriver/avizor/internal/smithy"
)

// specModel is a small model shaped like SageMaker's inference router (2026-11-16) and EC2's flex cost allocation
// policy (2026-11-13).
const specModel = `{
  "smithy": "2.0",
  "shapes": {
    "com.amazonaws.x#Svc": {"type": "service", "traits": {"aws.api#service": {"sdkId": "X"}}},
    "com.amazonaws.x#CreateRouter": {"type": "operation", "input": {"target": "com.amazonaws.x#CreateRouterRequest"},
      "traits": {"smithy.api#documentation": "<p>Creates a router. The router's status is <code>Creating</code> until it's ready.</p>"}},
    "com.amazonaws.x#DescribeRouter": {"type": "operation", "input": {"target": "com.amazonaws.x#DescribeRouterRequest"}, "output": {"target": "com.amazonaws.x#DescribeRouterOutput"}},
    "com.amazonaws.x#UpdateRouter": {"type": "operation", "input": {"target": "com.amazonaws.x#UpdateRouterRequest"},
      "traits": {"smithy.api#documentation": "<p>Updates a router. The targets that you specify replace all of the router's existing targets.</p>"}},
    "com.amazonaws.x#DeleteRouter": {"type": "operation", "input": {"target": "com.amazonaws.x#DeleteRouterRequest"}},
    "com.amazonaws.x#ListRouters": {"type": "operation"},
    "com.amazonaws.x#CreateRouterRequest": {"type": "structure", "members": {
      "RouterName": {"target": "com.amazonaws.x#Name", "traits": {"smithy.api#required": {}}},
      "Targets": {"target": "com.amazonaws.x#Targets", "traits": {"smithy.api#required": {}}},
      "ClientToken": {"target": "smithy.api#String", "traits": {"smithy.api#idempotencyToken": {}}},
      "Tags": {"target": "com.amazonaws.x#TagList"}}},
    "com.amazonaws.x#UpdateRouterRequest": {"type": "structure", "members": {
      "RouterName": {"target": "com.amazonaws.x#Name", "traits": {"smithy.api#required": {}}},
      "Targets": {"target": "com.amazonaws.x#Targets", "traits": {"smithy.api#required": {},
        "smithy.api#documentation": "<p>The complete list of targets. This list replaces the current set.</p>"}}}},
    "com.amazonaws.x#DescribeRouterRequest": {"type": "structure", "members": {
      "RouterName": {"target": "com.amazonaws.x#Name", "traits": {"smithy.api#required": {}}}}},
    "com.amazonaws.x#DescribeRouterOutput": {"type": "structure", "members": {
      "RouterName": {"target": "com.amazonaws.x#Name"},
      "RouterArn": {"target": "smithy.api#String"},
      "Targets": {"target": "com.amazonaws.x#Targets"},
      "Status": {"target": "com.amazonaws.x#RouterStatus"},
      "StatusReason": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#DeleteRouterRequest": {"type": "structure", "members": {
      "RouterName": {"target": "com.amazonaws.x#Name", "traits": {"smithy.api#required": {}}}}},
    "com.amazonaws.x#Name": {"type": "string", "traits": {"smithy.api#length": {"min": 1, "max": 63}, "smithy.api#pattern": "^[a-z-]+$"}},
    "com.amazonaws.x#Targets": {"type": "list", "member": {"target": "com.amazonaws.x#Target"}, "traits": {"smithy.api#length": {"min": 1, "max": 20}}},
    "com.amazonaws.x#Target": {"type": "structure", "members": {
      "Name": {"target": "com.amazonaws.x#Name", "traits": {"smithy.api#required": {}}},
      "RoutingStatus": {"target": "smithy.api#String", "traits": {"smithy.api#documentation": "<p>SageMaker AI ignores this field in <code>CreateRouter</code> requests.</p>"}},
      "Preference": {"target": "com.amazonaws.x#Preference"}}},
    "com.amazonaws.x#Preference": {"type": "enum", "members": {
      "LOCAL": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "LocalRegionFirst"}},
      "POOL": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "InstancePoolOrder"}}}},
    "com.amazonaws.x#RouterStatus": {"type": "enum", "members": {
      "A": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "Creating"}},
      "B": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "Updating"}},
      "C": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "InService"}},
      "D": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "Failed"}},
      "E": {"target": "smithy.api#Unit", "traits": {"smithy.api#enumValue": "Deleting"}}}},
    "com.amazonaws.x#TagList": {"type": "list", "member": {"target": "com.amazonaws.x#Tag"}},
    "com.amazonaws.x#Tag": {"type": "structure", "members": {"Key": {"target": "smithy.api#String"}, "Value": {"target": "smithy.api#String"}}},

    "com.amazonaws.x#ListAliases": {"type": "operation", "input": {"target": "com.amazonaws.x#ListAliasesRequest"}, "output": {"target": "com.amazonaws.x#ListAliasesResponse"}},
    "com.amazonaws.x#ListAliasesRequest": {"type": "structure", "members": {
      "FlowId": {"target": "smithy.api#String", "traits": {"smithy.api#required": {}, "smithy.api#documentation": "<p>The flow. You can't change it after you create the alias.</p>"}},
      "NextToken": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#ListAliasesResponse": {"type": "structure", "members": {
      "Aliases": {"target": "com.amazonaws.x#AliasList"}, "NextToken": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#AliasList": {"type": "list", "member": {"target": "com.amazonaws.x#Alias"}},
    "com.amazonaws.x#Alias": {"type": "structure", "members": {"AliasArn": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#CreatePolicy": {"type": "operation", "input": {"target": "com.amazonaws.x#CreatePolicyRequest"}},
    "com.amazonaws.x#ModifyPolicy": {"type": "operation", "input": {"target": "com.amazonaws.x#ModifyPolicyRequest"}},
    "com.amazonaws.x#DeletePolicies": {"type": "operation", "input": {"target": "com.amazonaws.x#DeletePoliciesRequest"}},
    "com.amazonaws.x#DescribePolicies": {"type": "operation", "input": {"target": "com.amazonaws.x#DescribePoliciesRequest"}, "output": {"target": "com.amazonaws.x#DescribePoliciesResult"}},
    "com.amazonaws.x#CreatePolicyRequest": {"type": "structure", "members": {
      "ResourceType": {"target": "smithy.api#String", "traits": {"smithy.api#required": {}}},
      "Description": {"target": "smithy.api#String"},
      "TagSpecifications": {"target": "com.amazonaws.x#TagList"}}},
    "com.amazonaws.x#ModifyPolicyRequest": {"type": "structure", "members": {
      "PolicyId": {"target": "smithy.api#String", "traits": {"smithy.api#required": {}}},
      "Description": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#DeletePoliciesRequest": {"type": "structure", "members": {
      "PolicyIds": {"target": "com.amazonaws.x#PolicyIds", "traits": {"smithy.api#required": {}}}}},
    "com.amazonaws.x#PolicyIds": {"type": "list", "member": {"target": "smithy.api#String"}, "traits": {"smithy.api#length": {"max": 50}}},
    "com.amazonaws.x#DescribePoliciesRequest": {"type": "structure", "members": {"NextToken": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#DescribePoliciesResult": {"type": "structure", "members": {
      "Policies": {"target": "com.amazonaws.x#PolicySet"}, "NextToken": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#PolicySet": {"type": "list", "member": {"target": "com.amazonaws.x#Policy"}},
    "com.amazonaws.x#Policy": {"type": "structure", "members": {
      "PolicyId": {"target": "smithy.api#String"}, "ResourceType": {"target": "smithy.api#String"},
      "Description": {"target": "smithy.api#String"}, "Tags": {"target": "com.amazonaws.x#TagList"}}}
  }
}`

func TestBuildSpec(t *testing.T) {
	t.Parallel()

	m, err := smithy.Parse([]byte(specModel))
	if err != nil {
		t.Fatal(err)
	}
	d := &smithy.Diff{
		AddedOperations: []string{"CreateRouter", "DescribeRouter", "UpdateRouter", "DeleteRouter", "ListRouters", "ListAliases",
			"CreatePolicy", "ModifyPolicy", "DeletePolicies", "DescribePolicies"},
		AddedEnumValues: []smithy.MemberChange{{Shape: "ResourceType", Member: "FCA", Value: "fca-policy"}},
	}
	ws := work(d, m.Usages())
	buildSpecs(ws, m, d)
	byNoun := map[string]*Spec{}
	for _, w := range ws {
		if w.Spec != nil {
			byNoun[w.Subject] = w.Spec
		}
	}
	if len(byNoun) != 3 {
		t.Fatalf("specs for %v, want Router, Alias, and Policy (the plural delete and describe join Policy)", ws)
	}

	// A list-only data source reads with its list.
	a := byNoun["Alias"]
	if len(a.Arguments) != 1 || a.Arguments[0].Member != "FlowId" || len(a.Attributes) != 1 || a.Attributes[0].Member != "AliasArn" ||
		len(a.Notes) != 1 || a.Notes[0].Source != "ListAliasesRequest.FlowId" {
		t.Errorf("Alias spec = %+v", a)
	}

	r := byNoun["Router"]
	if !slices.Equal(r.Operations.Create, []string{"CreateRouter"}) || !slices.Equal(r.Operations.Read, []string{"DescribeRouter"}) ||
		!slices.Equal(r.Operations.List, []string{"ListRouters"}) {
		t.Errorf("Router operations = %+v", r.Operations)
	}
	if !slices.Equal(r.Identifier, []string{"RouterName"}) {
		t.Errorf("Router identifier = %v", r.Identifier)
	}
	args := map[string]Field{}
	for _, f := range r.Arguments {
		args[f.Member] = f
	}
	if _, ok := args["ClientToken"]; ok {
		t.Error("ClientToken is an argument; idempotency tokens aren't")
	}
	if a := args["RouterName"]; !a.Required || !a.CreateOnly || a.Name != "router_name" || constraintText(a) != "1 to 63 characters, pattern ^[a-z-]+$" {
		t.Errorf("RouterName = %+v (%s)", a, constraintText(a))
	}
	targets := args["Targets"]
	if targets.CreateOnly || targets.Elem != smithy.TypeStructure || constraintText(targets) != "1 to 20 items" || len(targets.Fields) != 3 {
		t.Errorf("Targets = %+v", targets)
	}
	for _, f := range targets.Fields {
		switch f.Member {
		case "RoutingStatus":
			if !f.Ignored {
				t.Error("Target.RoutingStatus isn't marked ignored on write")
			}
		case "Preference":
			if !slices.Equal(f.Values, []string{"InstancePoolOrder", "LocalRegionFirst"}) {
				t.Errorf("Preference values = %v", f.Values)
			}
		}
	}
	if args["Tags"].CreateOnly {
		t.Error("Tags is create-only; tags are updated with tagging operations")
	}
	var attrs []string
	for _, f := range r.Attributes {
		attrs = append(attrs, f.Member)
	}
	if !slices.Equal(attrs, []string{"RouterArn", "Status", "StatusReason"}) {
		t.Errorf("Router attributes = %v", attrs)
	}
	if st := r.Status; st == nil || st.Member != "Status" || !slices.Equal(st.Pending, []string{"Creating", "Updating"}) ||
		!slices.Equal(st.Target, []string{"InService"}) || !slices.Equal(st.Failed, []string{"Failed"}) ||
		!slices.Equal(st.Delete, []string{"Deleting"}) || st.Reason != "StatusReason" {
		t.Errorf("Router status = %+v", r.Status)
	}
	if r.Delete.Kind != "delete" || r.Delete.Plural || r.Update.Kind != "fields" || !slices.Equal(r.Update.Replaces, []string{"UpdateRouter", "UpdateRouterRequest.Targets"}) || r.Tagging.Kind != "tags" {
		t.Errorf("Router delete %+v, update %+v, tagging %+v", r.Delete, r.Update, r.Tagging)
	}
	if !slices.ContainsFunc(r.Notes, func(n SpecNote) bool { return n.Source == "UpdateRouter" }) ||
		!slices.ContainsFunc(r.Notes, func(n SpecNote) bool { return n.Source == "Target.RoutingStatus" }) {
		t.Errorf("Router notes = %+v, want the UpdateRouter replacement and the ignored RoutingStatus", r.Notes)
	}

	p := byNoun["Policy"]
	if !slices.Equal(p.Operations.Read, []string{"DescribePolicies"}) || len(p.Operations.List) != 0 {
		t.Errorf("Policy operations = %+v; the plural Describe reads", p.Operations)
	}
	if d := p.Delete; d.Operation != "DeletePolicies" || !d.Plural || d.Member != "PolicyIds" || d.Max != 50 {
		t.Errorf("Policy delete = %+v", p.Delete)
	}
	if tg := p.Tagging; tg.Kind != "tag_specifications" || !slices.Equal(tg.ResourceTypes, []string{"fca-policy"}) {
		t.Errorf("Policy tagging = %+v", p.Tagging)
	}
	var pattrs []string
	for _, f := range p.Attributes {
		pattrs = append(pattrs, f.Member)
	}
	if !slices.Equal(pattrs, []string{"PolicyId"}) {
		t.Errorf("Policy attributes = %v, want PolicyId (Tags come from TagSpecifications)", pattrs)
	}
	for _, f := range p.Arguments {
		if f.Member == "ResourceType" && !f.CreateOnly || f.Member == "Description" && f.CreateOnly {
			t.Errorf("Policy argument %s create-only = %t", f.Member, f.CreateOnly)
		}
	}
}

func TestSentences(t *testing.T) {
	t.Parallel()

	got := sentences("Use v1.2 here, e.g. the default. The name can't change. 3 targets max.")
	want := []string{"Use v1.2 here, e.g. the default.", "The name can't change.", "3 targets max."}
	if !slices.Equal(got, want) {
		t.Errorf("sentences = %q, want %q", got, want)
	}
}

// fieldModel has a recursive union, a list of at most one, and two Puts on a service with TagResource: one returns an
// ARN, one doesn't.
const fieldModel = `{
  "smithy": "2.0",
  "shapes": {
    "com.amazonaws.x#Svc": {"type": "service", "traits": {"aws.api#service": {"sdkId": "X"}}},
    "com.amazonaws.x#TagResource": {"type": "operation"},
    "com.amazonaws.x#Export": {"type": "structure", "members": {
      "Values": {"target": "com.amazonaws.x#ValueMap"},
      "Configs": {"target": "com.amazonaws.x#ConfigList"},
      "Streams": {"target": "com.amazonaws.x#StreamList", "traits": {"smithy.api#documentation": "<p>The list must contain exactly one entry.</p>"}},
      "Choices": {"target": "com.amazonaws.x#StreamList", "traits": {"smithy.api#documentation": "<p>Contains exactly one of the streams.</p>"}}}},
    "com.amazonaws.x#StreamList": {"type": "list", "member": {"target": "smithy.api#String"}, "traits": {"smithy.api#length": {"min": 1, "max": 10000}}},
    "com.amazonaws.x#ValueMap": {"type": "map", "key": {"target": "smithy.api#String"}, "value": {"target": "com.amazonaws.x#AttributeValue"}},
    "com.amazonaws.x#AttributeValue": {"type": "union", "members": {
      "S": {"target": "smithy.api#String"},
      "L": {"target": "com.amazonaws.x#ValueList"}}},
    "com.amazonaws.x#ValueList": {"type": "list", "member": {"target": "com.amazonaws.x#AttributeValue"}},
    "com.amazonaws.x#ConfigList": {"type": "list", "member": {"target": "com.amazonaws.x#Config"}, "traits": {"smithy.api#length": {"min": 1, "max": 1}}},
    "com.amazonaws.x#Config": {"type": "union", "members": {"A": {"target": "smithy.api#String"}}},

    "com.amazonaws.x#PutAccountSettings": {"type": "operation", "input": {"target": "com.amazonaws.x#PutAccountSettingsRequest"},
      "output": {"target": "com.amazonaws.x#PutAccountSettingsResponse"}},
    "com.amazonaws.x#PutAccountSettingsRequest": {"type": "structure", "members": {"Mode": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#PutAccountSettingsResponse": {"type": "structure", "members": {}},
    "com.amazonaws.x#CreateWidget": {"type": "operation", "input": {"target": "com.amazonaws.x#CreateWidgetRequest"},
      "output": {"target": "com.amazonaws.x#CreateWidgetResponse"}},
    "com.amazonaws.x#DeleteWidget": {"type": "operation", "input": {"target": "com.amazonaws.x#PutAccountSettingsRequest"}},
    "com.amazonaws.x#CreateWidgetRequest": {"type": "structure", "members": {"Name": {"target": "smithy.api#String"}}},
    "com.amazonaws.x#CreateWidgetResponse": {"type": "structure", "members": {"Widget": {"target": "com.amazonaws.x#Widget"}}},
    "com.amazonaws.x#Widget": {"type": "structure", "members": {"WidgetArn": {"target": "smithy.api#String"}}}
  }
}`

func TestFieldShapes(t *testing.T) {
	t.Parallel()

	m, err := smithy.Parse([]byte(fieldModel))
	if err != nil {
		t.Fatal(err)
	}
	sh := m.Shapes["Export"]
	values := fieldOf(m, sh, "Values", 1)
	if values.Recursive != "AttributeValue" || len(values.Fields) != 0 {
		t.Errorf("Values = %+v; want AttributeValue, recursive, without its members", values)
	}
	if got := FieldText(values, false); got != "map of union `AttributeValue`, recursive" {
		t.Errorf("Values text = %q", got)
	}
	configs := fieldOf(m, sh, "Configs", 1)
	if got := FieldText(configs, true); got != "optional; single union" {
		t.Errorf("Configs text = %q; want a single block, without the 1 to 1 items constraint", got)
	}

	// A list documented as having exactly one entry is a single block; "exactly one of" isn't.
	if got := FieldText(fieldOf(m, sh, "Streams", 1), true); got != "optional; single string" {
		t.Errorf("Streams text = %q", got)
	}
	if got := FieldText(fieldOf(m, sh, "Choices", 1), true); got != "optional; list of string, 1 to 10000 items" {
		t.Errorf("Choices text = %q", got)
	}

	// Tags are set with TagResource only for a resource whose output has an ARN.
	for op, want := range map[string]string{"PutAccountSettings": "none", "CreateWidget": "tag_resource"} {
		s := buildSpec(m, Work{Kind: WorkNewResource, Subject: strings.TrimPrefix(strings.TrimPrefix(op, "Put"), "Create"), Operations: []string{op}}, nil)
		if s.Tagging == nil || s.Tagging.Kind != want {
			t.Errorf("%s tagging = %+v, want %s", op, s.Tagging, want)
		}
	}
}
