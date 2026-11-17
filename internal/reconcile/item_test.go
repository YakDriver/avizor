package reconcile

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakDriver/avizor/internal/index"
	"github.com/YakDriver/avizor/internal/services"
)

func TestSnakeCase(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"Route53ResolvedIPValidation": "route53_resolved_ip_validation",
		"skipHealthValidation":        "skip_health_validation",
		"ELBLoadBalancerLogging":      "elb_load_balancer_logging",
		"Name":                        "name",
	} {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMemberMatches(t *testing.T) {
	t.Parallel()

	released := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	it := &Item{NewMembers: []string{"Route53ResolvedIPValidation", "AlarmTagsPropagation", "ActionConfiguration", "Name"}, Published: released}
	for _, tc := range []struct {
		name        string
		title, body string
		created     time.Time
		want        []Match
	}{
		{
			name:    "title in words",
			title:   "Support Route 53 Resolved IP Validation",
			created: released.Add(24 * time.Hour),
			want:    []Match{{Type: MatchMember, Value: "Route53ResolvedIPValidation", Where: index.WhereTitle, Strength: StrengthStrong}},
		},
		{
			name:    "title with a singular word",
			title:   "Support alarm tag propagation in alarm actions",
			created: released.Add(24 * time.Hour),
			want:    []Match{{Type: MatchMember, Value: "AlarmTagsPropagation", Where: index.WhereTitle, Strength: StrengthStrong}},
		},
		{
			name:    "body only is weak",
			title:   "Next-generation firewall policy",
			body:    "route53_resolved_ip_validation {\n  action = \"DROP\"\n}",
			created: released.Add(24 * time.Hour),
			want:    []Match{{Type: MatchMember, Value: "Route53ResolvedIPValidation", Where: index.WhereBody, Strength: StrengthWeak}},
		},
		{
			name:    "a day before the release counts",
			title:   "route53_resolved_ip_validation",
			created: released.Add(-24 * time.Hour),
			want:    []Match{{Type: MatchMember, Value: "Route53ResolvedIPValidation", Where: index.WhereTitle, Strength: StrengthStrong}},
		},
		{
			name:    "snake_case in the title",
			title:   "r/aws_cloudwatch_metric_alarm: Add `action_configuration` block",
			created: released,
			want:    []Match{{Type: MatchMember, Value: "ActionConfiguration", Where: index.WhereTitle, Strength: StrengthStrong}},
		},
		{name: "part of another word", title: "Fix redaction configuration", created: released},
		{name: "before the release", title: "route53_resolved_ip_validation", created: released.Add(-7 * 24 * time.Hour)},
		{name: "long after the release", title: "route53_resolved_ip_validation", created: released.Add(memberWindow + time.Hour)},
		{name: "single word", title: "Name is wrong", created: released},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := it.memberMatches(&index.Issue{Title: tc.title, Body: tc.body, CreatedAt: tc.created})
			if len(got) != len(tc.want) {
				t.Fatalf("memberMatches = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("match %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRunItemAnnouncement checks that a candidate citing the item's announcement in its body, with the service
// label, is a strong match, and that one citing it only in a comment isn't.
func TestRunItemAnnouncement(t *testing.T) {
	t.Parallel()

	r := reconciler(t, seed(t), "")
	res, err := r.RunItem(context.Background(), &Item{
		ID: "sdk/fsx/release-2", Title: "FSx: something", Text: "something", Package: "fsx",
		AnnouncementURL: "https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-fsx-netapp-ontap-s3-access/",
		Request:         RequestChange, RequestBasis: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 45362, ""); c.Category != CategoryTracked {
		t.Errorf("#45362 = %s %+v, want already_tracked by the announcement", c.Category, c.Signals)
	}
	for _, c := range res.Candidates {
		if c.Number == 43195 && c.Category != CategoryRelated {
			t.Errorf("#43195 = %s %+v, want related: the announcement is only in a comment, and the label is s3control", c.Category, c.Signals)
		}
	}
}

// TestRunItemSubServiceLabel checks that a candidate with a sub-service label of the item's package keeps its strong
// signals, and that one with another service's label doesn't.
func TestRunItemSubServiceLabel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := index.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ts := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	tx, err := st.Begin(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range []*index.Issue{
		{Number: 50613, Kind: index.KindIssue, State: "open", Labels: []string{"enhancement", "service/transitgateway"},
			Title: "Transit Gateway 1:1 NAT", Body: "Uses ModifyTransitGatewayAttachmentNatConfiguration."},
		{Number: 50614, Kind: index.KindIssue, State: "open", Labels: []string{"service/networkmanager"},
			Title: "Cloud WAN 1:1 NAT", Body: "Uses ModifyTransitGatewayAttachmentNatConfiguration."},
	} {
		is.CreatedAt, is.UpdatedAt, is.URL = ts, ts, "https://github.com/"+repo+"/issues/"
		if err := tx.UpsertIssue(ctx, is, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMeta(ctx, "last_sync", ts.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	r := reconciler(t, st, "")
	r.Services = services.New(&services.Service{Package: "ec2", SubServices: []string{"transitgateway", "vpc"}})
	res, err := r.RunItem(ctx, &Item{
		ID: "sdk/ec2/release-1", Title: "EC2: 1:1 NAT", Text: "Adds 1:1 NAT.", Package: "ec2",
		NewOperations: []string{"ModifyTransitGatewayAttachmentNatConfiguration"},
		Request:       RequestChange, RequestBasis: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c := find(t, res.Candidates, 50613, ""); c.Category != CategoryTracked || !hasSignal(c, "label", "service/transitgateway") {
		t.Errorf("#50613 = %s %+v, want already_tracked with a label signal: service/transitgateway is an ec2 sub-service", c.Category, c.Signals)
	}
	if c := find(t, res.Candidates, 50614, ""); c.Category != CategoryRelated {
		t.Errorf("#50614 = %s %+v, want related: service/networkmanager isn't ec2", c.Category, c.Signals)
	}
}
