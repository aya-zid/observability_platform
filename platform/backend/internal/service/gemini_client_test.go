package service

import "testing"

func TestTodayReportSchemaKeyFindings(t *testing.T) {
	schema := TodayReportSchema()
	props := schema["properties"].(map[string]any)
	kf := props["key_findings"].(map[string]any)
	items := kf["items"].(map[string]any)
	iprops := items["properties"].(map[string]any)

	for _, f := range []string{"severity", "title", "detail"} {
		if _, ok := iprops[f]; !ok {
			t.Errorf("today key_findings item missing %q", f)
		}
	}
	// Every item must carry severity/title/detail as required.
	required := toStringSet(items["required"])
	for _, f := range []string{"severity", "title", "detail"} {
		if !required[f] {
			t.Errorf("today key_findings item %q should be required", f)
		}
	}
	sev := iprops["severity"].(map[string]any)
	if _, ok := sev["enum"]; !ok {
		t.Error("severity should be an enum")
	}
}

func TestCompareReportSchemaKeyFindings(t *testing.T) {
	schema := CompareReportSchema()
	props := schema["properties"].(map[string]any)
	kf := props["key_findings"].(map[string]any)
	items := kf["items"].(map[string]any)
	iprops := items["properties"].(map[string]any)

	for _, f := range []string{"severity", "title", "detail", "day_a_value", "day_b_value", "delta_pct"} {
		if _, ok := iprops[f]; !ok {
			t.Errorf("compare key_findings item missing %q", f)
		}
	}
	required := toStringSet(items["required"])
	for _, f := range []string{"severity", "title", "detail", "day_a_value", "day_b_value"} {
		if !required[f] {
			t.Errorf("compare key_findings item %q should be required", f)
		}
	}
	dp := iprops["delta_pct"].(map[string]any)
	if dp["type"] != "NUMBER" {
		t.Errorf("delta_pct type = %v, want NUMBER", dp["type"])
	}
	if required["delta_pct"] {
		t.Error("delta_pct should not be required")
	}
}

func TestCompareReportSchemaSections(t *testing.T) {
	schema := CompareReportSchema()
	props := schema["properties"].(map[string]any)

	// biggest_differences.direction must be an up|down|flat enum.
	regions := props["regions"].(map[string]any)
	ritems := regions["items"].(map[string]any)
	rprops := ritems["properties"].(map[string]any)
	bd := rprops["biggest_differences"].(map[string]any)
	bdItems := bd["items"].(map[string]any)
	bdProps := bdItems["properties"].(map[string]any)
	dir := bdProps["direction"].(map[string]any)
	for _, v := range []string{"up", "down", "flat"} {
		if !contains(dir["enum"], v) {
			t.Errorf("direction enum missing %q: %v", v, dir["enum"])
		}
	}

	for _, f := range []string{"service_comparisons", "alerts_diff", "recommendations"} {
		if _, ok := props[f]; !ok {
			t.Errorf("CompareReportSchema missing %q", f)
		}
	}

	required := toStringSet(schema["required"])
	for _, f := range []string{"overall_status", "executive_summary", "key_findings", "regions", "service_comparisons", "alerts_diff", "recommendations"} {
		if !required[f] {
			t.Errorf("compare schema missing required %q", f)
		}
	}
}

func TestTodayReportSchemaSections(t *testing.T) {
	schema := TodayReportSchema()
	required := toStringSet(schema["required"])
	for _, f := range []string{"overall_status", "executive_summary", "key_findings", "regions", "alerts_insight", "recommendations", "outlook"} {
		if !required[f] {
			t.Errorf("today schema missing required %q", f)
		}
	}
}

func TestRecommendationSchema(t *testing.T) {
	rec := recommendationSchema()
	props := rec["properties"].(map[string]any)
	for _, f := range []string{"priority", "title", "action", "rationale", "expected_impact", "effort"} {
		if _, ok := props[f]; !ok {
			t.Errorf("recommendation missing %q", f)
		}
	}
	effort := props["effort"].(map[string]any)
	for _, v := range []string{"low", "medium", "high"} {
		if !contains(effort["enum"], v) {
			t.Errorf("effort enum missing %q: %v", v, effort["enum"])
		}
	}
	required := toStringSet(rec["required"])
	for _, f := range []string{"priority", "title", "action", "rationale", "expected_impact", "effort"} {
		if !required[f] {
			t.Errorf("recommendation missing required %q", f)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toStringSet(v any) map[string]bool {
	out := map[string]bool{}
	arr, ok := v.([]string)
	if !ok {
		return out
	}
	for _, s := range arr {
		out[s] = true
	}
	return out
}

func contains(v any, want string) bool {
	arr, ok := v.([]string)
	if !ok {
		return false
	}
	for _, s := range arr {
		if s == want {
			return true
		}
	}
	return false
}
