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
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateClusterInput.metricGroups", Operations: []string{"CreateCluster"}, UsedBy: []string{cluster}, Member: "metric_groups", Field: field},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateClusterInput.metricGroups", Operations: []string{"UpdateCluster"}, UsedBy: []string{cluster, peering}, Member: "metric_groups", Field: field},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "GetClusterOutput.metricGroups", Operations: []string{"GetCluster"}, UsedBy: []string{cluster, peering}, Field: field},
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
	if d.Title != "r/`aws_widget_cluster`: Add `metric_groups` and `vpc.egress_mode`" {
		t.Errorf("title = %q", d.Title)
	}
	for _, want := range []string{
		"- New argument `metric_groups` (optional; list of structure), from `CreateClusterInput.metricGroups` and " +
			"`UpdateClusterInput.metricGroups`; returned by `GetClusterOutput.metricGroups`. <!-- decide: set of strings or nested block --> " +
			"<!-- decide: optional, or optional and computed -->\n  - `metric_group` (required; enum, STATS)\n",
		"- New argument `vpc.egress_mode` (optional; string), from `Vpc.EgressMode`.\n",
		"- `RoutingStrategy`: new value `KV_AWARE`. The type is validated with the SDK type in `internal/service/widget/cluster.go:9`.\n",
		"\n- `vpc` is a ListNestedBlock in `internal/service/widget/cluster.go:40` (plugin-framework); `vpc` forces replacement.\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if !slices.Equal(d.Decide, []string{"metric_groups: set of strings or nested block", "metric_groups: optional, or optional and computed"}) {
		t.Errorf("decide = %q", d.Decide)
	}

	// Release notes naming the operations replace the API fields; a nested structure's field stays.
	if d, err = New(item("widget", "Adds metricGroups to CreateCluster, UpdateCluster, and GetCluster."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- New argument `metric_groups` (optional; list of structure). <!-- decide: set of strings or nested block -->",
		"- New argument `vpc.egress_mode` (optional; string), from `Vpc.EgressMode`.\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body with named operations has no %q:\n%s", want, d.Body)
		}
	}

	// Without the checkout, the resource is a judgment call.
	if d, err = New(item("widget", "Adds metric groups."), en, nil, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Body, "- `metric_groups`: new argument `metric_groups` (optional; list of structure)") || !slices.Contains(d.Decide, "metric_groups: which resource") {
		t.Errorf("body without a checkout:\n%s\ndecide %q", d.Body, d.Decide)
	}
}

// Fields are grouped per noun and path; the create input's field describes the argument; an output reaching the
// argument's own shape counts as read returning it; a member in several blocks is in each; new operations are named.
func TestScopeChangeGroups(t *testing.T) {
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
	reqString := &enrich.Field{Name: "color", Member: "Color", Type: "string", Required: true}
	optList := &enrich.Field{Name: "color", Member: "Color", Type: "list", Elem: "string"}
	en := implemented("widget",
		// The output comes first, required; the create input is optional.
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "GetClusterOutput.Size", UsedBy: []string{cluster}, Field: &enrich.Field{Name: "size", Type: "list", Elem: "string", Required: true}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateClusterInput.Size", UsedBy: []string{cluster}, Member: "size", Field: &enrich.Field{Name: "size", Type: "list", Elem: "string", Required: true}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateClusterInput.Size", UsedBy: []string{cluster}, Member: "size", Field: &enrich.Field{Name: "size", Type: "list", Elem: "string"}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateClusterInput.Color", UsedBy: []string{cluster}, Member: "color", Field: reqString},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreatePeeringInput.Color", UsedBy: []string{peering}, Member: "color", Field: optList},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "Settings.Mode", UsedBy: []string{cluster}, Member: "mode", Returns: []string{"GetCluster"},
			Field:  &enrich.Field{Name: "mode", Type: "list", Elem: "string"},
			Schema: []enrich.SchemaPlace{{File: cluster, Line: 10, Path: "primary.settings"}, {File: cluster, Line: 20, Path: "secondary.settings"}}},
		enrich.Work{Kind: enrich.WorkNewOperation, Subject: "Job", Operations: []string{"StartJob", "StopJob"}},
	)
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds things."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `aws_widget_cluster`: new argument `size` (optional; list of string), from `UpdateClusterInput.Size` and `CreateClusterInput.Size`; returned by `GetClusterOutput.Size`. <!-- decide: optional, or optional and computed -->\n",
		"- `aws_widget_cluster`: new argument `color` (required; string), from `CreateClusterInput.Color`.\n",
		"- `aws_widget_peering`: new argument `color` (optional; list of string), from `CreatePeeringInput.Color`.\n",
		// A shape in two blocks of one resource is in both.
		"- `aws_widget_cluster`: new arguments `primary.settings.mode` and `secondary.settings.mode` (optional; list of string), from `Settings.Mode`; " +
			"returned by `GetCluster`. <!-- decide: optional, or optional and computed -->\n",
		"- New operations `StartJob` and `StopJob`. <!-- decide: resource, argument, or none -->\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
}

func TestShapeNoun(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{"CreateClusterInput": "Cluster", "GetClusterOutput": "Cluster", "UpdateClusterRequest": "Cluster",
		"VpcEgressConfiguration": "VpcEgressConfiguration", "Output": "Output", "ClusterOutput": "Cluster"} {
		if got := shapeNoun(in); got != want {
			t.Errorf("shapeNoun(%s) = %s, want %s", in, got, want)
		}
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

func TestNamedOperations(t *testing.T) {
	t.Parallel()

	notes := []string{"Adds metricGroups to CreateCluster and GetCluster, and Router to Widgets.", "UpdateClusters is new."}
	work := func(subject string, ops ...string) enrich.Work { return enrich.Work{Subject: subject, Operations: ops} }
	for _, tc := range []struct {
		name  string
		works []enrich.Work
		want  bool
	}{
		{"named", []enrich.Work{work("CreateClusterInput.metricGroups", "CreateCluster"), work("GetClusterOutput.metricGroups", "GetCluster")}, true},
		{"request and response", []enrich.Work{work("CreateClusterRequest.x", "CreateCluster"), work("GetClusterResponse.x", "GetCluster")}, true},
		{"not a whole word", []enrich.Work{work("UpdateClusterInput.metricGroups", "UpdateCluster")}, false},
		{"a structure named like an output", []enrich.Work{work("RouterOutput.Processors", "GetRouter")}, false},
		{"nested structure", []enrich.Work{work("Cluster.metricGroups", "CreateCluster")}, false},
		{"returns not named", []enrich.Work{{Subject: "CreateClusterInput.x", Operations: []string{"CreateCluster"}, Returns: []string{"DescribeCluster"}}}, false},
		{"none", nil, false},
	} {
		if got := namedOperations(notes, tc.works); got != tc.want {
			t.Errorf("%s: namedOperations = %t", tc.name, got)
		}
	}
}

// writeFiles writes provider files with annotations into a checkout, by path under internal/service/widget.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	co := t.TempDir()
	src := filepath.Join(co, "internal", "service", "widget")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(src, f), []byte("package widget\n\n"+body+"func f() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return co
}

func TestResourceNames(t *testing.T) {
	t.Parallel()

	co := writeFiles(t, map[string]string{
		// Consecutive annotations are one implementation under two names; the last is the current one.
		"listener.go": "// @SDKResource(\"aws_alb_listener\", name=\"Listener\")\n// @SDKResource(\"aws_lb_listener\", name=\"Listener\")\n// @Tags(identifierAttribute=\"arn\")\n",
		"voices.go":   "// @FrameworkDataSource(\"aws_widget_voices\", name=\"Voices\")\n",
	})
	got := resourceNames(co, []string{"internal/service/widget/listener.go", "internal/service/widget/voices.go"})
	want := []typeName{{Name: "aws_lb_listener"}, {Name: "aws_widget_voices", DataSource: true}}
	if !slices.Equal(got, want) {
		t.Errorf("resourceNames = %+v, want %+v", got, want)
	}
}

func TestBestNamed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		names, ops, want []string
	}{
		{[]string{"aws_rds_cluster", "aws_rds_global_cluster"}, []string{"CreateDBCluster", "ModifyDBCluster"}, []string{"aws_rds_cluster"}},
		{[]string{"aws_iam_role", "aws_iam_service_linked_role"}, []string{"UpdateRole"}, []string{"aws_iam_role"}},
		// Generic words don't count against a name.
		{[]string{"aws_s3_bucket", "aws_s3_bucket_replication_configuration"}, []string{"PutBucketReplication"}, []string{"aws_s3_bucket_replication_configuration"}},
		// Each name matches one noun exactly; the association has words of both, but more than either.
		{[]string{"aws_wafv2_rule_group", "aws_wafv2_web_acl", "aws_wafv2_web_acl_rule_group_association"}, []string{"CreateRuleGroup", "CreateWebACL"},
			[]string{"aws_wafv2_rule_group", "aws_wafv2_web_acl"}},
		// Each operation keeps its own best match.
		{[]string{"aws_widget_cluster", "aws_widget_cluster_endpoint"}, []string{"CreateCluster", "CreateClusterEndpoint"},
			[]string{"aws_widget_cluster", "aws_widget_cluster_endpoint"}},
		// No word matched: all are kept.
		{[]string{"aws_sesv2_account_vdm_attributes", "aws_sesv2_account_suppression_attributes"}, []string{"GetDomain"},
			[]string{"aws_sesv2_account_vdm_attributes", "aws_sesv2_account_suppression_attributes"}},
	} {
		if got := bestNamed(tc.names, tc.ops, "aws_"); !slices.Equal(got, tc.want) {
			t.Errorf("bestNamed(%q, %q) = %q, want %q", tc.names, tc.ops, got, tc.want)
		}
	}
}

// Members that aren't arguments are left out, with why; an argument of operations no file calls is in a note.
func TestScopeLeftOut(t *testing.T) {
	t.Parallel()

	co := writeFiles(t, map[string]string{
		"queue.go":   "// @FrameworkResource(\"aws_widget_queue\", name=\"Queue\")\n",
		"queues.go":  "// @FrameworkDataSource(\"aws_widget_queues\", name=\"Queues\")\n",
		"cluster.go": "// @FrameworkResource(\"aws_widget_cluster\", name=\"Cluster\")\n",
	})
	queue, queues := "internal/service/widget/queue.go", "internal/service/widget/queues.go"
	boolean := &enrich.Field{Type: "boolean"}
	list := &enrich.Field{Type: "list", Elem: "structure"}
	en := implemented("widget",
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateQueueInput.LogConfiguration", Operations: []string{"CreateQueue"}, UsedBy: []string{queue}, Field: list},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateQueueInput.AddLogDestinations", Operations: []string{"UpdateQueue"}, UsedBy: []string{queue}, Field: list},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateQueueInput.RemoveLogDestinations", Operations: []string{"UpdateQueue"}, UsedBy: []string{queue}, Field: list},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateQueueInput.ResetRetention", Operations: []string{"UpdateQueue"}, UsedBy: []string{queue}, Field: boolean},
		// A List request member is a data source's argument, if a data source calls the List.
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "ListQueuesInput.QueueType", Operations: []string{"ListQueues"}, UsedBy: []string{queues}, Field: &enrich.Field{Type: "string"}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "SearchQueuesInput.Owner", Operations: []string{"SearchQueues"}, UsedBy: []string{queue}, Field: &enrich.Field{Type: "string"}},
		// Operations no file calls: the argument waits for a resource, and the attribute is left out.
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateRecordInput.Metadata", Operations: []string{"CreateRecord", "UpdateRecord"}, Field: &enrich.Field{Type: "document"}},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "Session.GatewayStatus", Operations: []string{"DescribeSessions"}, Field: &enrich.Field{Type: "enum"}},
	)
	en.CodeSearched = true
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds logging."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `aws_widget_queue`: new argument `log_configuration` (optional; list of structure), from `CreateQueueInput.LogConfiguration`.",
		"- `aws_widget_queues` data source: new argument `queue_type` (optional; string), from `ListQueuesInput.QueueType`.",
		"- Update (`UpdateQueue`) takes `AddLogDestinations` and `RemoveLogDestinations`, so it applies the difference between the configured and current lists.",
		"- `metadata` is in `CreateRecord` and `UpdateRecord`, which no resource calls yet.",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	for _, unwanted := range []string{"reset_retention", "add_log_destinations", "owner", "gateway_status", "decide: which resource"} {
		if strings.Contains(d.Body, unwanted) {
			t.Errorf("body has %q:\n%s", unwanted, d.Body)
		}
	}
	if got := inferred(d, "left_out"); !slices.Equal(got, []string{"add_log_destinations", "remove_log_destinations", "reset_retention", "owner", "gateway_status"}) {
		t.Errorf("left out = %q", got)
	}

	// Without the code search, nothing is left out.
	en.CodeSearched = false
	if d, err = New(item("widget", "Adds logging."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	if got := inferred(d, "left_out"); len(got) != 0 || !strings.Contains(d.Body, "reset_retention") {
		t.Errorf("left out without a code search = %q:\n%s", got, d.Body)
	}
}

// An attribute joins the argument read returns it as, and a summary shape's attribute joins the full shape's.
func TestGroupChanges(t *testing.T) {
	t.Parallel()

	export, cert := "internal/service/widget/export.go", "internal/service/widget/certificate.go"
	spec := &enrich.Field{Type: "structure", Fields: []enrich.Field{{Name: "filter_expression", Type: "string"}}}
	usage := &enrich.Field{Type: "enum", Values: []string{"CLIENT", "SERVER"}}
	cs := groupChanges([]enrich.Work{
		{Kind: enrich.WorkNewArgument, Subject: "ExportTableInput.FilterSpecification", Operations: []string{"ExportTable"}, UsedBy: []string{export}, Field: spec},
		{Kind: enrich.WorkNewAttribute, Subject: "ExportDescription.FilterSpecification", Operations: []string{"DescribeExport"}, UsedBy: []string{export}, Field: spec},
		{Kind: enrich.WorkNewArgument, Subject: "RequestCertificateRequest.Usage", Operations: []string{"RequestCertificate"}, UsedBy: []string{cert}, Field: usage},
		{Kind: enrich.WorkNewAttribute, Subject: "CertificateDetail.Usage", Operations: []string{"DescribeCertificate"}, UsedBy: []string{cert}, Field: usage},
		{Kind: enrich.WorkNewAttribute, Subject: "CertificateSummary.Usage", Operations: []string{"ListCertificates"}, Field: usage},
		// The same name in another resource's shapes stays apart, the summary too.
		{Kind: enrich.WorkNewAttribute, Subject: "Peering.Usage", Operations: []string{"DescribePeering"}, UsedBy: []string{"internal/service/widget/peering.go"}, Field: usage},
		{Kind: enrich.WorkNewAttribute, Subject: "PeeringSummary.Usage", Operations: []string{"ListPeerings"}, UsedBy: []string{"internal/service/widget/peering.go"}, Field: usage},
	})
	var got []string
	for _, c := range cs {
		got = append(got, c.name+": "+strings.Join(c.subjects, ", ")+" returned by "+strings.Join(c.returned, ", "))
	}
	want := []string{
		"filter_specification: `ExportTableInput.FilterSpecification` returned by `ExportDescription.FilterSpecification`",
		"usage: `RequestCertificateRequest.Usage` returned by `CertificateDetail.Usage`, `CertificateSummary.Usage`",
		"usage: `Peering.Usage`, `PeeringSummary.Usage` returned by ",
	}
	if !slices.Equal(got, want) {
		t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Members the launch text names are listed first.
func TestMentionedFirst(t *testing.T) {
	t.Parallel()

	en := implemented("widget",
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "DataSourceParameters.AuthenticationType", Member: "authentication_type", Field: &enrich.Field{Type: "string"}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "Filter.HierarchyFilter", Member: "hierarchy_filter", Field: &enrich.Field{Type: "structure"}},
	)
	d, err := New(item("widget", "This release adds HierarchyFilter support."), en, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if i, j := strings.Index(d.Body, "`hierarchy_filter`"), strings.Index(d.Body, "`authentication_type`"); i < 0 || j < 0 || i > j {
		t.Errorf("hierarchy_filter isn't first:\n%s", d.Body)
	}
}

// Review cases: whole Add and Remove verbs, schema helper files, notes-only scopes, a data source's lines, and a data
// source among a grouped query's callers.
func TestScopeOwnership(t *testing.T) {
	t.Parallel()

	co := writeFiles(t, map[string]string{
		"queue.go":        "// @FrameworkResource(\"aws_widget_queue\", name=\"Queue\")\n",
		"queue_schema.go": "",
		"other.go":        "// @FrameworkResource(\"aws_widget_other\", name=\"Other\")\n",
		"queues.go":       "// @FrameworkDataSource(\"aws_widget_queues\", name=\"Queues\")\n",
	})
	queue, schema, other, queues := "internal/service/widget/queue.go", "internal/service/widget/queue_schema.go", "internal/service/widget/other.go", "internal/service/widget/queues.go"
	str := &enrich.Field{Type: "string"}
	en := implemented("widget",
		// Address starts with Add but isn't an add list.
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "UpdateQueueInput.Address", Operations: []string{"UpdateQueue"}, UsedBy: []string{queue}, Field: str},
		// The parent block is in the queue's schema helper and in another resource's file.
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "Retention.Days", Operations: []string{"CreateQueue"}, UsedBy: []string{queue}, Field: str,
			Schema: []enrich.SchemaPlace{{File: schema, Line: 5, Path: "retention", Kind: "ListNestedBlock"}, {File: other, Line: 9, Path: "other_retention", Kind: "ListNestedBlock"}}},
	)
	en.CodeSearched = true
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds things."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- New argument `address` (optional; string)", "- New argument `retention.days` (optional; string)", "`retention` is a ListNestedBlock in `" + schema + ":5`"} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if strings.Contains(d.Body, "other_retention") {
		t.Errorf("body has another resource's block:\n%s", d.Body)
	}

	// Only a note: the scope still has it.
	en = implemented("widget", enrich.Work{Kind: enrich.WorkNewArgument, Subject: "CreateRecordInput.Metadata", Operations: []string{"CreateRecord"}, Field: str})
	en.CodeSearched = true
	if d, err = New(item("widget", "Adds records."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Body, "## Scope\n\n- `metadata` is in `CreateRecord`, which no resource calls yet.\n") {
		t.Errorf("notes-only body:\n%s", d.Body)
	}

	// A data source's lines lose its name, and the title names it. A query member is a data source's though the
	// operations' callers have a resource in common.
	en = implemented("widget",
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "ListQueuesInput.QueueType", Operations: []string{"ListQueues"}, UsedBy: []string{queue, queues}, Field: str},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "DescribeQueuesInput.QueueType", Operations: []string{"DescribeQueues"}, UsedBy: []string{queue}, Field: str},
	)
	en.CodeSearched = true
	if d, err = New(item("widget", "Adds queue types."), en, nil, o); err != nil {
		t.Fatal(err)
	}
	if d.Title != "d/`aws_widget_queues`: Add `queue_type`" || !strings.Contains(d.Body, "- New argument `queue_type` (optional; string), from `ListQueuesInput.QueueType` and `DescribeQueuesInput.QueueType`.") {
		t.Errorf("title = %q, body:\n%s", d.Title, d.Body)
	}
}

// A Put input's member joins the block the input's noun names, and an attribute named like a new resource is that
// resource's read.
func TestScopeBlockInputAndNewTypeRead(t *testing.T) {
	t.Parallel()

	set := "internal/service/widget/configuration_set.go"
	co := writeFiles(t, map[string]string{"configuration_set.go": "// @SDKResource(\"aws_widget_configuration_set\", name=\"Configuration Set\")\n"})
	mode := &enrich.Field{Type: "enum", Values: []string{"GLOBAL", "REGIONAL"}}
	en := implemented("widget",
		enrich.Work{Kind: enrich.WorkNewResource, Subject: "AccountDeliveryAttributes", Operations: []string{"PutAccountDeliveryAttributes"},
			Spec: &enrich.Spec{Operations: enrich.SpecOperations{Create: []string{"PutAccountDeliveryAttributes"}}}},
		// A new type with its own read doesn't take an existing resource's attribute.
		enrich.Work{Kind: enrich.WorkNewDataSource, Subject: "PlatformVersion", Operations: []string{"DescribePlatformVersions"},
			Spec: &enrich.Spec{Operations: enrich.SpecOperations{Read: []string{"DescribePlatformVersions"}}}},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "ConfigurationSet.PlatformVersion", Operations: []string{"GetConfigurationSet"}, UsedBy: []string{set},
			Field: &enrich.Field{Type: "string"}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "DeliveryOptions.SharedPoolDeliveryMode", Operations: []string{"CreateConfigurationSet"}, UsedBy: []string{set}, Field: mode,
			Schema: []enrich.SchemaPlace{{File: set, Line: 58, Path: "delivery_options", Kind: "TypeList"}}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "PutConfigurationSetDeliveryOptionsRequest.SharedPoolDeliveryMode",
			Operations: []string{"PutConfigurationSetDeliveryOptions"}, UsedBy: []string{set}, Field: mode},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "GetAccountResponse.DeliveryAttributes", Operations: []string{"GetAccount"}, UsedBy: []string{set},
			Field: &enrich.Field{Type: "structure"}},
	)
	en.CodeSearched = true
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds delivery modes."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `aws_widget_configuration_set`: new argument `delivery_options.shared_pool_delivery_mode` (optional; enum, GLOBAL | REGIONAL), " +
			"from `DeliveryOptions.SharedPoolDeliveryMode` and `PutConfigurationSetDeliveryOptionsRequest.SharedPoolDeliveryMode`.\n",
		"- Read `delivery_attributes` from `GetAccountResponse.DeliveryAttributes`.\n",
		"- `aws_widget_configuration_set`: new attribute `platform_version` (string), from `ConfigurationSet.PlatformVersion`.\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if strings.Count(d.Body, "shared_pool_delivery_mode") != 1 || strings.Contains(d.Body, "new attribute `delivery_attributes`") {
		t.Errorf("body repeats a member:\n%s", d.Body)
	}
}

// Review cases: a Put input in another resource's file, a nested shape with no block, an attribute from a mutation's
// response, and an attribute an exactly named resource reads all stay as they are.
func TestScopeBlockInputLimits(t *testing.T) {
	t.Parallel()

	foo, bar, account := "internal/service/widget/foo.go", "internal/service/widget/bar.go", "internal/service/widget/account.go"
	co := writeFiles(t, map[string]string{
		"foo.go":     "// @SDKResource(\"aws_widget_foo\", name=\"Foo\")\n",
		"bar.go":     "// @SDKResource(\"aws_widget_bar\", name=\"Bar\")\n",
		"account.go": "// @SDKResource(\"aws_widget_account\", name=\"Account\")\n",
	})
	mode := &enrich.Field{Type: "string"}
	put := &enrich.Spec{Operations: enrich.SpecOperations{Create: []string{"PutAccountDeliveryAttributes"}}}
	en := implemented("widget",
		enrich.Work{Kind: enrich.WorkNewResource, Subject: "AccountDeliveryAttributes", Operations: []string{"PutAccountDeliveryAttributes"}, Spec: put},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "BarDeliveryOptions.Mode", Operations: []string{"CreateBar"}, UsedBy: []string{bar}, Field: mode,
			Schema: []enrich.SchemaPlace{{File: bar, Line: 5, Path: "delivery_options", Kind: "TypeList"}}},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "PutFooDeliveryOptionsRequest.Mode", Operations: []string{"PutFooDeliveryOptions"}, UsedBy: []string{foo}, Field: mode},
		enrich.Work{Kind: enrich.WorkNewArgument, Subject: "DeliveryOptions.Mode", Operations: []string{"CreateBar"}, UsedBy: []string{bar}, Field: mode},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "CreateBarResponse.DeliveryAttributes", Operations: []string{"CreateBar"}, UsedBy: []string{bar},
			Field: &enrich.Field{Type: "structure"}},
		enrich.Work{Kind: enrich.WorkNewAttribute, Subject: "GetAccountResponse.DeliveryAttributes", Operations: []string{"GetAccount"}, UsedBy: []string{account},
			Field: &enrich.Field{Type: "structure"}},
	)
	en.CodeSearched = true
	o := opts
	o.Checkout = co
	d, err := New(item("widget", "Adds delivery modes."), en, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `aws_widget_bar`: new argument `delivery_options.mode` (optional; string), from `BarDeliveryOptions.Mode`.\n",
		"- `aws_widget_foo`: new argument `mode` (optional; string), from `PutFooDeliveryOptionsRequest.Mode`.\n",
		"- `aws_widget_bar`: new argument `mode` (optional; string), from `DeliveryOptions.Mode`.\n",
		"- `aws_widget_bar`: new attribute `delivery_attributes` (structure), from `CreateBarResponse.DeliveryAttributes`.\n",
		"- `aws_widget_account`: new attribute `delivery_attributes` (structure), from `GetAccountResponse.DeliveryAttributes`.\n",
	} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("body has no %q:\n%s", want, d.Body)
		}
	}
	if strings.Contains(d.Body, "Read `delivery_attributes`") {
		t.Errorf("body reassigns an attribute:\n%s", d.Body)
	}
}
