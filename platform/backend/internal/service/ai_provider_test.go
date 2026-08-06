package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fakeAIProvider records the payloads it received and returns canned JSON.
type fakeAIProvider struct {
	calls   []string
	payload string
}

func (f *fakeAIProvider) Generate(_ context.Context, systemPrompt, payloadJSON string, _ map[string]any) (string, error) {
	f.calls = append(f.calls, systemPrompt)
	// Return a valid JSON object echoing the first "region" field if present.
	if strings.Contains(payloadJSON, `"region"`) {
		var m map[string]any
		if json.Unmarshal([]byte(payloadJSON), &m) == nil {
			if r, ok := m["region"].(string); ok {
				return `{"region":"` + r + `","summary":"s","highlights":[],"concerns":[],"notable_metrics":[]}`, nil
			}
		}
	}
	return `{"overall_status":"healthy","regions":[]}`, nil
}

func TestChunkedAIProviderSplitsLargePayload(t *testing.T) {
	inner := &fakeAIProvider{}
	chunked := &chunkedAIProvider{inner: inner, maxBytes: 100}

	payload := `{
		"type":"today",
		"day_a":"2026-08-03",
		"generated_at":"2026-08-03T12:00:00Z",
		"regions":[
			{"region":"night","day_a":{"services":{"nginx":{"total_requests":100}}}},
			{"region":"morning","day_a":{"services":{"nginx":{"total_requests":200}}}}
		],
		"alerts_a":[],
		"service_health":[],
		"hosts":{"total":1,"online":1},
		"fleet":null
	}`

	text, err := chunked.Generate(context.Background(), "SYSTEM", payload, TodayReportSchema())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !json.Valid([]byte(text)) {
		t.Fatalf("output not valid JSON: %q", text)
	}
	// 2 region calls + 1 assembly call.
	if len(inner.calls) != 3 {
		t.Fatalf("calls = %d, want 3 (2 regions + assembly), got %v", len(inner.calls), inner.calls)
	}
	// Region calls must use the region prompt, the final one the assembly prompt.
	if !strings.Contains(inner.calls[0], "ONE fixed UTC time region") {
		t.Errorf("call 0 should be the region prompt, got %q", inner.calls[0])
	}
	if !strings.Contains(inner.calls[2], "ASSEMBLY INSTRUCTION") {
		t.Errorf("call 2 should be the assembly prompt, got %q", inner.calls[2])
	}
}

func TestChunkedAIProviderSmallPayloadPassesThrough(t *testing.T) {
	inner := &fakeAIProvider{}
	chunked := &chunkedAIProvider{inner: inner, maxBytes: 1000000}

	payload := `{"type":"today","regions":[]}`
	if _, err := chunked.Generate(context.Background(), "SYSTEM", payload, TodayReportSchema()); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("calls = %d, want 1 (pass-through)", len(inner.calls))
	}
}
