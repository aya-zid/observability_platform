package service

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

func TestDeltaPct(t *testing.T) {
	cases := []struct {
		name string
		a, b *float64
		want *float64
	}{
		{"rise", f64(100), f64(150), f64(50)},
		{"fall", f64(200), f64(150), f64(-25)},
		{"flat", f64(80), f64(80), f64(0)},
		{"zero_denominator_is_null", f64(0), f64(10), nil},
		{"nil_a", nil, f64(10), nil},
		{"nil_b", f64(10), nil, nil},
		{"both_nil", nil, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deltaPct(c.a, c.b)
			if c.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a value, got nil")
			}
			if *got != *c.want {
				t.Fatalf("expected %.2f, got %.2f", *c.want, *got)
			}
		})
	}
}

func TestComputeDeltasRegionAndDay(t *testing.T) {
	mkService := func(requests float64, fiveXX float64) *ReportServiceMetrics {
		return &ReportServiceMetrics{TotalRequests: &requests, Total5xx: &fiveXX}
	}

	mkExtraction := func(requests, fiveXX float64) dayExtraction {
		return dayExtraction{
			windows: map[string]reportWindowResult{
				"morning": {
					services: map[string]*ReportServiceMetrics{"nginx": mkService(requests, fiveXX)},
				},
				"afternoon": {},
			},
			fullDay: reportWindowResult{
				services: map[string]*ReportServiceMetrics{"nginx": mkService(requests, fiveXX)},
			},
			alerts: nil,
		}
	}

	deltas := computeDeltas(mkExtraction(100, 10), mkExtraction(150, 20))

	// Expect per-region deltas for "morning" and whole-day deltas for "day".
	var foundMorning, foundDay bool
	for _, d := range deltas {
		if d.Region == "morning" && d.Service == "nginx" && d.Metric == "total_requests" {
			foundMorning = true
			if d.DayAValue == nil || *d.DayAValue != 100 || d.DayBValue == nil || *d.DayBValue != 150 {
				t.Fatalf("morning total_requests values wrong: %+v", d)
			}
			if d.DeltaPct == nil || *d.DeltaPct != 50 {
				t.Fatalf("morning total_requests delta wrong: %+v", d)
			}
		}
		if d.Region == "day" && d.Service == "nginx" && d.Metric == "total_5xx" {
			foundDay = true
			if d.DeltaPct == nil || *d.DeltaPct != 100 {
				t.Fatalf("day total_5xx delta wrong: %+v", d)
			}
		}
	}
	if !foundMorning {
		t.Fatal("expected per-region 'morning' delta for nginx total_requests")
	}
	if !foundDay {
		t.Fatal("expected whole-day 'day' delta for nginx total_5xx")
	}
}

func TestMidnightUTC(t *testing.T) {
	in := time.Date(2026, 8, 2, 14, 32, 5, 0, time.FixedZone("x", 3600))
	got := midnightUTC(in)
	want := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
}

func f64(v float64) *float64 { return &v }

func TestCountAlertsInRange(t *testing.T) {
	day := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	start := day.UnixNano()
	end := day.Add(24 * time.Hour).UnixNano()
	mk := func(h, m int) domain.AlertEvent {
		return domain.AlertEvent{TriggeredAt: day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)}
	}
	alerts := []domain.AlertEvent{
		mk(0, 0),   // exactly at start -> included
		mk(5, 59),  // night
		mk(23, 59), // just before end -> included
		mk(24, 0),  // exactly at end -> excluded
		mk(25, 0),  // after -> excluded
	}
	if got := countAlertsInRange(alerts, start, end); got != 3 {
		t.Errorf("countAlertsInRange = %d, want 3", got)
	}
	if got := countAlertsInRange(nil, start, end); got != 0 {
		t.Errorf("countAlertsInRange(nil) = %d, want 0", got)
	}
}

func TestMapAlerts(t *testing.T) {
	val := 42.0
	triggered := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	resolved := triggered.Add(30 * time.Minute)
	ev := domain.AlertEvent{
		Rule:          &domain.AlertRule{DefaultSeverity: "warning", TemplateName: "nginx_5xx"},
		Threshold:     &domain.AlertThreshold{Severity: "critical"},
		HostMachineID: "machine-1",
		MetricString:  "nginx",
		Status:        "firing",
		TriggeredAt:   triggered,
		ResolvedAt:    &resolved,
		ActualValue:   &val,
		Domain:        "phpapp.local",
		Pool:          "www",
		ContainerName: "",
	}

	out := mapAlerts([]domain.AlertEvent{ev})
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	p := out[0]
	if p.Severity != "critical" {
		t.Errorf("Severity = %q, want critical (threshold override)", p.Severity)
	}
	if p.RuleName != "nginx_5xx" {
		t.Errorf("RuleName = %q, want nginx_5xx", p.RuleName)
	}
	if p.HostMachineID != "machine-1" || p.Service != "nginx" || p.Status != "firing" {
		t.Errorf("mapping of base fields wrong: %+v", p)
	}
	if p.ResolvedAt == "" {
		t.Error("ResolvedAt should be set when the event is resolved")
	}
	if p.ActualValue == nil || *p.ActualValue != 42 {
		t.Errorf("ActualValue = %v, want 42", p.ActualValue)
	}

	// No threshold -> falls back to rule default severity.
	noThr := ev
	noThr.Threshold = nil
	if out := mapAlerts([]domain.AlertEvent{noThr}); out[0].Severity != "warning" {
		t.Errorf("Severity without threshold = %q, want warning", out[0].Severity)
	}
	// No rule -> empty rule name.
	noRule := ev
	noRule.Rule = nil
	if out := mapAlerts([]domain.AlertEvent{noRule}); out[0].RuleName != "" {
		t.Errorf("RuleName without rule = %q, want empty", out[0].RuleName)
	}
	// Unresolved -> ResolvedAt stays empty.
	unresolved := ev
	unresolved.ResolvedAt = nil
	if out := mapAlerts([]domain.AlertEvent{unresolved}); out[0].ResolvedAt != "" {
		t.Errorf("ResolvedAt for unresolved event = %q, want empty", out[0].ResolvedAt)
	}
}

func TestMapIncidentsFiltering(t *testing.T) {
	dayStart := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	mk := func(firing time.Time, resolved *time.Time) IncidentCard {
		return IncidentCard{
			ID:          "c1",
			Title:       "title",
			Severity:    "warning",
			Category:    CategoryPerformance,
			Status:      "firing",
			Hostname:    "kali.kali",
			FiringSince: firing,
			ResolvedAt:  resolved,
			Members:     []IncidentMember{{}, {}},
		}
	}

	inside := dayStart.Add(6 * time.Hour)
	c1 := mk(inside, nil) // firing inside day, unresolved -> included
	before := dayStart.Add(-2 * time.Hour)
	c2 := mk(before, &before) // resolved before day start -> excluded
	after := dayEnd.Add(1 * time.Hour)
	c3 := mk(after, nil) // firing after day end -> excluded
	fireBefore := dayStart.Add(-4 * time.Hour)
	resInside := dayStart.Add(3 * time.Hour)
	c4 := mk(fireBefore, &resInside) // overlaps the day -> included

	out := mapIncidents([]IncidentCard{c1, c2, c3, c4}, dayStart, dayEnd)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	for _, p := range out {
		if p.Members != 2 {
			t.Errorf("Members = %d, want 2", p.Members)
		}
		if p.Category != string(CategoryPerformance) {
			t.Errorf("Category = %q, want %q", p.Category, string(CategoryPerformance))
		}
	}
}

func TestServiceUpQuery(t *testing.T) {
	cases := map[string]string{
		"nginx":      `count(nginx_requests_total)`,
		"apache":     `count(apache_requests_total)`,
		"postgresql": `count(pg_up)`,
		"mariadb":    `count(mysql_up)`,
		"redis":      `count(redis_up)`,
		"phpfpm":     `count(phpfpm_up)`,
		"container":  `count({__name__=~"container_.*_cpu_percent"})`,
		"unknown":    "",
	}
	for kind, want := range cases {
		if got := serviceUpQuery(kind); got != want {
			t.Errorf("serviceUpQuery(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestHeadlineMetricsMatchStructTags(t *testing.T) {
	// Every metric key used in the delta headlines must map to a real JSON tag
	// on the payload structs, so deltas never reference a field that is renamed
	// or dropped. top_error_path is intentionally absent (string, not numeric).
	serviceTags := jsonTagSet(reflect.TypeOf(ReportServiceMetrics{}))
	for kind, refs := range reportHeadlines {
		for _, ref := range refs {
			if !serviceTags[ref.metric] {
				t.Errorf("reportHeadlines[%s] references %q with no JSON tag on ReportServiceMetrics", kind, ref.metric)
			}
		}
	}

	fleetTags := jsonTagSet(reflect.TypeOf(ReportFleetMetrics{}))
	for _, ref := range reportFleetHeadlines {
		if !fleetTags[ref.metric] {
			t.Errorf("reportFleetHeadlines references %q with no JSON tag on ReportFleetMetrics", ref.metric)
		}
	}
}

func jsonTagSet(rt reflect.Type) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < rt.NumField(); i++ {
		if tag := rt.Field(i).Tag.Get("json"); tag != "" && tag != "-" {
			name := strings.Split(tag, ",")[0]
			if name != "" {
				out[name] = true
			}
		}
	}
	return out
}
