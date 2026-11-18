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

var _ = enum.Validate[awstypes.RoutingStrategy]()
var _ fwtypes.StringEnum[awstypes.Mode]
var _ = awstypes.RoutingStrategy(v)
var _ = output.Mode
`
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Enricher{Checkout: checkout}
	ws := []Work{{Kind: WorkNewEnumValue, Subject: "RoutingStrategy"}, {Kind: WorkNewEnumValue, Subject: "Mode"}, {Kind: WorkNewEnumValue, Subject: "Unused"}}
	if err := e.enumUses(context.Background(), "x", ws); err != nil {
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
	if v, o := lines(ws[0].EnumUses); !slices.Equal(v, []int{3}) || !slices.Equal(o, []int{5}) {
		t.Errorf("RoutingStrategy validated %v, other %v", v, o)
	}
	if v, o := lines(ws[1].EnumUses); !slices.Equal(v, []int{4}) || o != nil {
		t.Errorf("Mode validated %v, other %v; output.Mode is a field, not the type", v, o)
	}
	if !ws[2].EnumSearched || ws[2].EnumUses != nil {
		t.Errorf("Unused = %+v, want searched with no uses", ws[2])
	}
}
