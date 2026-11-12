package items

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	st, err := Open(filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	day := func(d int) time.Time { return time.Date(2026, 1, d, 20, 0, 0, 0, time.UTC) }
	rec := func(svc string, d int, status string) Record {
		return Record{ID: "s/" + svc + "/" + day(d).Format(time.DateOnly), Source: "s", Service: svc, Release: day(d).Format(time.DateOnly),
			PublishedAt: day(d), Status: status, PulledAt: day(d), Data: json.RawMessage(`{"x":1}`)}
	}
	if err := st.Put(ctx, []Record{rec("fsx", 1, StatusNew), rec("ec2", 2, StatusSkipped), rec("fsx", 3, StatusNew)}); err != nil {
		t.Fatal(err)
	}
	// Put replaces.
	r := rec("fsx", 1, StatusSkipped)
	if err := st.Put(ctx, []Record{r}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, r.ID)
	if err != nil || got == nil || got.Status != StatusSkipped || string(got.Data) != `{"x":1}` || !got.PublishedAt.Equal(day(1)) {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if missing, err := st.Get(ctx, "nope"); missing != nil || err != nil {
		t.Errorf("Get(nope) = %v, %v", missing, err)
	}

	for name, tc := range map[string]struct {
		f    Filter
		want []string
	}{
		"all":     {Filter{}, []string{"fsx/2026-01-03", "ec2/2026-01-02", "fsx/2026-01-01"}},
		"service": {Filter{Service: "fsx"}, []string{"fsx/2026-01-03", "fsx/2026-01-01"}},
		"status":  {Filter{Status: StatusNew}, []string{"fsx/2026-01-03"}},
		"since":   {Filter{Since: day(2)}, []string{"fsx/2026-01-03", "ec2/2026-01-02"}},
		"limit":   {Filter{Limit: 1}, []string{"fsx/2026-01-03"}},
	} {
		recs, err := st.List(ctx, tc.f)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, r := range recs {
			ids = append(ids, r.Service+"/"+r.Release)
		}
		if len(ids) != len(tc.want) {
			t.Errorf("%s: %v, want %v", name, ids, tc.want)
			continue
		}
		for i := range ids {
			if ids[i] != tc.want[i] {
				t.Errorf("%s: %v, want %v", name, ids, tc.want)
				break
			}
		}
	}
}
