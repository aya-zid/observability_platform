package handler

import (
	"testing"
	"time"
)

func TestNormalizeReportDates(t *testing.T) {
	now := time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC)

	cases := []struct {
		name    string
		typ     string
		dateA   string
		dateB   string
		wantA   string
		wantB   string
		wantErr bool
	}{
		{"today_default", "today", "", "", "2026-08-03", "", false},
		{"today_explicit", "today", "2026-08-02", "", "2026-08-02", "", false},
		{"today_ignores_date_b", "today", "2026-08-02", "2026-08-01", "2026-08-02", "", false},
		{"today_bad_format", "today", "02/08/2026", "", "", "", true},
		{"compare_requires_a", "compare", "", "", "", "", true},
		{"compare_defaults_b_to_day_before", "compare", "2026-08-03", "", "2026-08-03", "2026-08-02", false},
		{"compare_explicit", "compare", "2026-08-03", "2026-08-01", "2026-08-03", "2026-08-01", false},
		{"compare_same_dates", "compare", "2026-08-03", "2026-08-03", "", "", true},
		{"compare_bad_a", "compare", "not-a-date", "2026-08-01", "", "", true},
		{"compare_bad_b", "compare", "2026-08-03", "nope", "", "", true},
		{"unknown_type", "weekly", "2026-08-03", "", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotA, gotB, err := normalizeReportDates(c.typ, c.dateA, c.dateB, now)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got a=%q b=%q", gotA, gotB)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotA != c.wantA || gotB != c.wantB {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotA, gotB, c.wantA, c.wantB)
			}
		})
	}
}

func TestNormalizeReportDatesUsesUTCDate(t *testing.T) {
	// A "today" report must use the UTC calendar date, not the local one.
	local := time.Date(2026, 8, 3, 23, 30, 0, 0, time.FixedZone("x", -3600)) // = 2026-08-04 00:30 UTC
	gotA, gotB, err := normalizeReportDates("today", "", "", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotA != "2026-08-04" {
		t.Errorf("today default = %q, want 2026-08-04 (UTC date)", gotA)
	}
	if gotB != "" {
		t.Errorf("today date_b = %q, want empty", gotB)
	}
}
