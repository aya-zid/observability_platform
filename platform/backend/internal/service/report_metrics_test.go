package service

import (
	"regexp"
	"testing"
	"time"
)

func TestRegionWindowsFixedUTC(t *testing.T) {
	day := time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)
	windows := regionWindows(day)

	if len(windows) != 4 {
		t.Fatalf("regionWindows() returned %d regions, want 4", len(windows))
	}

	type expect struct {
		start, end int64
	}
	expected := map[string]expect{
		"night":     {day.Add(0 * time.Hour).UnixNano(), day.Add(6 * time.Hour).UnixNano()},
		"morning":   {day.Add(6 * time.Hour).UnixNano(), day.Add(12 * time.Hour).UnixNano()},
		"afternoon": {day.Add(12 * time.Hour).UnixNano(), day.Add(18 * time.Hour).UnixNano()},
		"evening":   {day.Add(18 * time.Hour).UnixNano(), day.Add(24 * time.Hour).UnixNano()},
	}

	for region, want := range expected {
		got, ok := windows[region]
		if !ok {
			t.Fatalf("regionWindows() missing region %q", region)
		}
		if got[0] != want.start || got[1] != want.end {
			t.Errorf("region %q = [%d, %d), want [%d, %d)", region, got[0], got[1], want.start, want.end)
		}
	}
}

func TestRegionWindowsCoversFullDayAndIsContiguous(t *testing.T) {
	day := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	windows := regionWindows(day)

	if windows["night"][0] != day.UnixNano() {
		t.Fatalf("first region should start at day midnight, got %d", windows["night"][0])
	}
	if windows["evening"][1] != day.Add(24*time.Hour).UnixNano() {
		t.Fatalf("last region should end at next midnight, got %d", windows["evening"][1])
	}

	order := []string{"night", "morning", "afternoon", "evening"}
	for i := 0; i < len(order)-1; i++ {
		if windows[order[i]][1] != windows[order[i+1]][0] {
			t.Errorf("regions %q and %q are not contiguous", order[i], order[i+1])
		}
	}
}

func TestRegionWindowsIgnoresInputLocation(t *testing.T) {
	// A non-UTC input must still produce UTC windows based on its calendar date.
	local := time.Date(2026, time.August, 2, 3, 0, 0, 0, time.FixedZone("test", 3*3600))
	utc := time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)

	wLocal := regionWindows(local)
	wUTC := regionWindows(utc)

	for region := range wUTC {
		if wLocal[region] != wUTC[region] {
			t.Errorf("region %q differs by input location: %v vs %v", region, wLocal[region], wUTC[region])
		}
	}
}

func TestServiceNameRegex(t *testing.T) {
	cases := map[string]string{
		"nginx":      `nginx.*`,
		"apache":     `apache.*|apache2.*|httpd.*`,
		"phpfpm":     `phpfpm.*`,
		"mariadb":    `mysql.*|mariadb.*|mysqld.*`,
		"postgresql": `postgres.*|postgresql.*`,
		"redis":      `redis.*`,
		"host":       `journald`,
		"unknown":    `unknown.*`,
	}
	for kind, want := range cases {
		if got := serviceNameRegex(kind); got != want {
			t.Errorf("serviceNameRegex(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestReportWebSelector(t *testing.T) {
	got := reportWebSelector("nginx", "access")
	want := `{service_name=~"nginx.*"} | log_kind="access"`
	if got != want {
		t.Errorf("reportWebSelector(nil, access) = %q, want %q", got, want)
	}
	if got := reportWebSelector("apache", ""); got != `{service_name=~"apache.*|apache2.*|httpd.*"}` {
		t.Errorf("reportWebSelector without log kind = %q", got)
	}
}

func TestRatioPct(t *testing.T) {
	a, b := 50.0, 100.0
	if got := ratioPct(&a, &b); got == nil || *got != 50 {
		t.Errorf("ratioPct(50,100) = %v, want 50", got)
	}
	z := 0.0
	if got := ratioPct(&a, &z); got != nil {
		t.Errorf("ratioPct(50,0) = %v, want nil", *got)
	}
	if got := ratioPct(nil, &b); got != nil {
		t.Errorf("ratioPct(nil,100) = %v, want nil", *got)
	}
	if got := ratioPct(&a, nil); got != nil {
		t.Errorf("ratioPct(50,nil) = %v, want nil", *got)
	}
}

func TestJoinRegexPatterns(t *testing.T) {
	if got := joinRegexPatterns(nil); got != "" {
		t.Errorf("joinRegexPatterns(nil) = %q, want empty", got)
	}
	if got := joinRegexPatterns([]*regexp.Regexp{}); got != "" {
		t.Errorf("joinRegexPatterns(empty) = %q, want empty", got)
	}
	p := regexp.MustCompile(`(?i)error`)
	if got := joinRegexPatterns([]*regexp.Regexp{p}); got != "(?i)error" {
		t.Errorf("joinRegexPatterns(single) = %q", got)
	}
	q := regexp.MustCompile(`fatal`)
	if got := joinRegexPatterns([]*regexp.Regexp{p, q}); got != "(?i)error|fatal" {
		t.Errorf("joinRegexPatterns(multi) = %q", got)
	}
}
