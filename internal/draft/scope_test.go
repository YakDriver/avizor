package draft

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/YakDriver/avizor/internal/enrich"
	"github.com/YakDriver/avizor/internal/smithy"
)

func TestScopeResource(t *testing.T) {
	t.Parallel()

	spec := &enrich.Spec{
		Operations: enrich.SpecOperations{Create: []string{"CreateFlexPolicy"}, Read: []string{"DescribeFlexPolicies"},
			Update: []string{"ModifyFlexPolicy"}, Delete: []string{"DeleteFlexPolicies"}, Other: []string{"AssociateFlexPolicy"}},
		Arguments: []enrich.Field{
			{Name: "resource_type", Member: "ResourceType", Type: "enum", Required: true, CreateOnly: true, Values: []string{"ec2", "natgw"}},
			{Name: "groups", Member: "Groups", Type: "list", Elem: "structure", Returned: true, Constraints: smithy.Constraints{},
				Fields: []enrich.Field{{Name: "group", Member: "Group", Type: "enum", Required: true, Values: []string{"STATS"}}}},
			{Name: "tag_specifications", Member: "TagSpecifications", Type: "list", Elem: "structure"},
		},
		Attributes: []enrich.Field{{Name: "fp_policy_id", Member: "FpPolicyId", Type: "string"}, {Name: "owner", Member: "Owner", Type: "string"}},
		Status:     &enrich.SpecStatus{Member: "State", Pending: []string{"creating"}, Target: []string{"available"}},
		Delete:     &enrich.SpecDelete{Kind: "delete", Operation: "DeleteFlexPolicies", Plural: true, Member: "FpPolicyIds", Max: 50},
		Update:     &enrich.SpecUpdate{Kind: "fields"},
		Tagging:    &enrich.SpecTags{Kind: "tag_specifications", Member: "TagSpecifications", ResourceTypes: []string{"fp-policy"}},
		Notes:      []enrich.SpecNote{{Source: "ModifyFlexPolicy", Text: "Versions are never removed."}},
	}
	en := implemented("widget", enrich.Work{Kind: enrich.WorkNewResource, Subject: "FlexPolicy", Spec: spec})
	d, err := New(item("widget", "Adds flex policies."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Scope\n\nNew resource `aws_widget_flex_policy`: `CreateFlexPolicy`, `DescribeFlexPolicies`, `ModifyFlexPolicy`, `DeleteFlexPolicies`.\n",
		"  - `resource_type` (required, forces replacement; enum, ec2 | natgw)\n",
		"  - `groups` (optional; list of structure) <!-- decide: set of strings or nested block --> <!-- decide: optional, or optional and computed -->\n    - `group` (required; enum, STATS)\n",
		"  - `tags`\n- Attributes: `fp_policy_id`, `owner`. <!-- decide: Terraform names for `FpPolicyId`, which abbreviate the resource's name -->\n",
		"- `State` has pending values `creating` and target `available`. Wait for the target value.\n",
		"- Delete is `DeleteFlexPolicies`, which takes up to 50 IDs in `FpPolicyIds`.\n",
		"- Tags are set with `TagSpecifications` (new resource type `fp-policy`).\n",
		"- `ModifyFlexPolicy`: \"Versions are never removed.\"\n",
		"- Other operations: `AssociateFlexPolicy`. <!-- decide: one resource per association, an argument, or none -->\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if strings.Contains(d.Body, "tag_specifications") {
		t.Errorf("tag member listed as an argument:\n%s", d.Body)
	}
	if len(d.Decide) != 4 || d.Decide[0] != "aws_widget_flex_policy groups: set of strings or nested block" {
		t.Errorf("decide = %q", d.Decide)
	}
}

func TestScopeChanges(t *testing.T) {
	t.Parallel()

	co := t.TempDir()
	src := filepath.Join(co, "internal", "service", "widget")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, name := range map[string]string{"cluster.go": "aws_widget_cluster", "peering.go": "aws_widget_peering"} {
		if err := os.WriteFile(filepath.Join(src, f), []byte("package widget\n\n// @FrameworkResource(\""+name+"\", name=\"X\")\nfunc f() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cluster, peering := "internal/service/widget/cluster.go", "internal/service/widget/peering.go"
	field := &enrich.Field{Name: "metric_groups", Member: "metricGroups", Type: "list", Elem: "structure",
		Fields: []enrich.Field{{Name: "metric_group", Member: "metricGroup", Type: "enum", Required: true, Values: []string{"STATS"}}}}
	en := implemented("widget",
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateClusterInput.metricGroups", UsedBy: []string{cluster}, Member: "metric_groups", Field: field},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateClusterInput.metricGroups", UsedBy: []string{cluster, peering}, Member: "metric_groups", Field: field},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "GetClusterOutput.metricGroups", UsedBy: []string{cluster, peering}, Field: field},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "Vpc.EgressMode", UsedBy: []string{cluster}, Member: "egress_mode",
			Field:  &enrich.Field{Name: "egress_mode", Member: "EgressMode", Type: "string"},
			Schema: []enrich.SchemaPlace{{File: cluster, Line: 40, Path: "vpc", Kind: "ListNestedBlock", Framework: "plugin-framework", ForceNew: "vpc"}}},
		enrich.Work{Kind: enrich.WorkNewEnumValue, Subject: "RoutingStrategy", Values: []string{"KV_AWARE"}, EnumSearched: true,
			EnumUses: []enrich.EnumUse{{File: cluster, Line: 9, Validated: true}}},
	)
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds metric groups."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "r/aws_widget_cluster: Add `metric_groups` and `egress_mode`" {
		t.Errorf("title = %q", d.Title)
	}
	for _, want := range []string{
		"- `aws_widget_cluster`: new argument `metric_groups` (optional; list of structure), from `CreateClusterInput.metricGroups` and " +
			"`UpdateClusterInput.metricGroups`; returned by `GetClusterOutput.metricGroups`. <!-- decide: set of strings or nested block --> " +
			"<!-- decide: optional, or optional and computed -->\n  - `metric_group` (required; enum, STATS)\n",
		"- `aws_widget_cluster`: new argument `vpc.egress_mode` (optional; string), from `Vpc.EgressMode`.\n",
		"- `RoutingStrategy`: new value `KV_AWARE`. The type is validated with the SDK type in `internal/service/widget/cluster.go:9`.\n",
		"Notes:\n\n- `vpc` is a ListNestedBlock in `internal/service/widget/cluster.go:40` (plugin-framework); `vpc` forces replacement.\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if !slices.Equal(d.Decide, []string{"metric_groups: set of strings or nested block", "metric_groups: optional, or optional and computed"}) {
		t.Errorf("decide = %q", d.Decide)
	}

	// Without the checkout, the resource is a judgment call.
	if d, err = New(item("widget", "Adds metric groups."), en, nil, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Body, "- `metric_groups`: new argument `metric_groups` (optional; list of structure)") || !slices.Contains(d.Decide, "metric_groups: which resource") {
		t.Errorf("body without a checkout:\n%s\ndecide %q", d.Body, d.Decide)
	}
}

func TestAbbreviates(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, subject string
		want          bool
	}{
		{"fca_policy_id", "NetworkingFlexCostAllocationPolicy", true},
		{"fca_policy_arn", "NetworkingFlexCostAllocationPolicy", true},
		{"policy_id", "NetworkingFlexCostAllocationPolicy", false},
		{"vpc_id", "NetworkingFlexCostAllocationPolicy", false},
		{"owner_account", "NetworkingFlexCostAllocationPolicy", false},
		{"ir_arn", "InferenceRouter", true},
	} {
		if got := abbreviates(tc.name, tc.subject); got != tc.want {
			t.Errorf("abbreviates(%s, %s) = %t", tc.name, tc.subject, got)
		}
	}
}
