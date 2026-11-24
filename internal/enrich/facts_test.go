package enrich

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/YakDriver/avizor/internal/smithy"
)

const frameworkSource = `package x

import "github.com/hashicorp/terraform-plugin-framework/resource/schema"

func (r *resource) Schema() {
	_ = schema.Schema{
		Blocks: map[string]schema.Block{
			"configuration": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"vpc_egress_configuration": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{},
						},
					},
				},
			},
			"conditional": schema.ListNestedBlock{
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplaceIf(f, "", "")},
				NestedObject:  schema.NestedBlockObject{},
			},
			"metadata_configuration": schema.ListNestedBlock{
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"text": schema.SingleNestedBlock{},
					},
				},
			},
		},
	}
}
`

const sdkv2Source = `package x

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func resourceX() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"production_variants": {
				Type:     schema.TypeList,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"routing_config": &schema.Schema{
							Type:     schema.TypeList,
							MaxItems: 1,
							Elem:     &schema.Resource{},
						},
						// A set of strings with a block's name isn't a block.
						"allowed_principals": {
							Type: schema.TypeSet,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}
`

func TestPlacesOf(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	fw, err := parser.ParseFile(fset, "fw.go", frameworkSource, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := frameworkOf(fw); got != "plugin-framework" {
		t.Errorf("framework = %q", got)
	}
	ps := placesOf(fw, fset, []string{"vpc_egress_configuration"}, nil)
	if len(ps) != 1 || ps[0].Path != "configuration.vpc_egress_configuration" || ps[0].Kind != "ListNestedBlock" || ps[0].ForceNew != "" {
		t.Errorf("vpc_egress_configuration places = %+v", ps)
	}
	if ps := placesOf(fw, fset, []string{"conditional"}, nil); len(ps) != 1 || ps[0].ForceNew != "" || ps[0].ForceNewIf != "conditional" {
		t.Errorf("conditional places = %+v; RequiresReplaceIf is conditional", ps)
	}
	ps = placesOf(fw, fset, []string{"text"}, nil)
	if len(ps) != 1 || ps[0].Path != "metadata_configuration.text" || ps[0].ForceNew != "metadata_configuration" {
		t.Errorf("text places = %+v; metadata_configuration forces replacement", ps)
	}

	fset = token.NewFileSet()
	sdk, err := parser.ParseFile(fset, "sdk.go", sdkv2Source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := frameworkOf(sdk); got != "sdkv2" {
		t.Errorf("framework = %q", got)
	}
	ps = placesOf(sdk, fset, []string{"routing_config"}, nil)
	if len(ps) != 1 || ps[0].Path != "production_variants.routing_config" || ps[0].Kind != "TypeList (MaxItems 1)" || ps[0].ForceNew != "production_variants" {
		t.Errorf("routing_config places = %+v", ps)
	}
	if ps := placesOf(sdk, fset, []string{"allowed_principals"}, nil); len(ps) != 0 {
		t.Errorf("allowed_principals places = %+v, want none", ps)
	}
	// production_variants is written without &schema.Schema, as SDKv2 map values usually are.
	if ps := placesOf(sdk, fset, []string{"production_variants"}, nil); len(ps) != 1 || ps[0].Kind != "TypeList" || ps[0].ForceNew != "production_variants" {
		t.Errorf("production_variants places = %+v", ps)
	}
}

func TestEnumUses(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}

	checkout := t.TempDir()
	dir := filepath.Join(checkout, "internal", "service", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package x

import (
	awstypes "github.com/aws/aws-sdk-go-v2/service/x/types"
	othertypes "github.com/aws/aws-sdk-go-v2/service/other/types"
)

var _ = enum.Validate[awstypes.RoutingStrategy]()
var _ fwtypes.StringEnum[awstypes.Mode]
var _ = awstypes.RoutingStrategy(v)
var _ = output.Mode
// var _ = enum.Validate[awstypes.Mode]()
var _ = "awstypes.Mode"
var _ = enum.Validate[othertypes.Mode]()
`
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Enricher{Checkout: checkout}
	ws := []Work{{Kind: WorkNewEnumValue, Subject: "RoutingStrategy"}, {Kind: WorkNewEnumValue, Subject: "Mode"}, {Kind: WorkNewEnumValue, Subject: "Unused"}}
	if err := e.enumUses(context.Background(), "x", "github.com/aws/aws-sdk-go-v2/service/x", ws); err != nil {
		t.Fatal(err)
	}
	lines := func(us []EnumUse) (validated, other []int) {
		for _, u := range us {
			if u.Validated {
				validated = append(validated, u.Line)
			} else {
				other = append(other, u.Line)
			}
		}
		return
	}
	if v, o := lines(ws[0].EnumUses); !slices.Equal(v, []int{8}) || !slices.Equal(o, []int{10}) {
		t.Errorf("RoutingStrategy validated %v, other %v", v, o)
	}
	if v, o := lines(ws[1].EnumUses); !slices.Equal(v, []int{9}) || o != nil {
		t.Errorf("Mode validated %v, other %v; a field, a comment, a string, and another service's Mode don't count", v, o)
	}
	if !ws[2].EnumSearched || ws[2].EnumUses != nil {
		t.Errorf("Unused = %+v, want searched with no uses", ws[2])
	}
}

// TestAboveNamesXMLWrapper checks that the block above a shape in an XML-wrapped list is named for the wrapper's member,
// as CloudFront's origin is for DistributionConfig.Origins (Quantity and Items).
func TestAboveNamesXMLWrapper(t *testing.T) {
	t.Parallel()

	m, err := smithy.Parse([]byte(`{
  "smithy": "2.0",
  "shapes": {
    "com.amazonaws.x#Svc": {"type": "service", "traits": {"aws.api#service": {"sdkId": "X"}}},
    "com.amazonaws.x#DistributionConfig": {"type": "structure", "members": {"Origins": {"target": "com.amazonaws.x#Origins"}}},
    "com.amazonaws.x#Origins": {"type": "structure", "members": {"Quantity": {"target": "smithy.api#Integer"}, "Items": {"target": "com.amazonaws.x#OriginList"}}},
    "com.amazonaws.x#OriginList": {"type": "list", "member": {"target": "com.amazonaws.x#Origin"}},
    "com.amazonaws.x#Origin": {"type": "structure", "members": {"CustomOriginConfig": {"target": "com.amazonaws.x#CustomOriginConfig"}}},
    "com.amazonaws.x#CustomOriginConfig": {"type": "structure", "members": {"OriginHttpVersion": {"target": "smithy.api#String"}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := aboveNames(m, []string{"Origin"}); !slices.Contains(got, "origin") || !slices.Contains(got, "items") {
		t.Errorf("aboveNames(Origin) = %v, want items and origin", got)
	}
	if !xmlWrapper(m, "Origins") || xmlWrapper(m, "Origin") {
		t.Error("xmlWrapper: want Origins only")
	}
}
