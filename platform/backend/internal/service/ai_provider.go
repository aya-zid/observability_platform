// internal/service/ai_provider.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// AIProvider turns the numeric telemetry payload into the structured AI report
// JSON text. Implementations are stateless wrappers around a single LLM REST
// API (Gemini, Groq/OpenAI-compatible, ...). The API key lives only server-side.
type AIProvider interface {
	// Generate sends the system prompt plus the numeric payload JSON and returns
	// the raw JSON text produced by the model. The returned text must be valid
	// JSON; implementations validate and retry once on malformed output.
	Generate(ctx context.Context, systemPrompt, payloadJSON string, schema map[string]any) (string, error)
}

// fallbackAIProvider tries primary first and falls back to secondary on any
// error, so the reports feature keeps working when one vendor is down or the
// model is unavailable.
type fallbackAIProvider struct {
	primary   AIProvider
	secondary AIProvider
}

func (f *fallbackAIProvider) Generate(ctx context.Context, systemPrompt, payloadJSON string, schema map[string]any) (string, error) {
	text, err := f.primary.Generate(ctx, systemPrompt, payloadJSON, schema)
	if err != nil {
		log.Printf("[ai] primary provider failed: %v", err)
		if f.secondary == nil {
			return "", err
		}
		alt, altErr := f.secondary.Generate(ctx, systemPrompt, payloadJSON, schema)
		if altErr == nil {
			return alt, nil
		}
		log.Printf("[ai] secondary provider failed: %v", altErr)
		return "", altErr
	}
	return text, nil
}

// chunkedAIProvider splits a large numbers payload into per-region analysis
// calls followed by one assembly call. This keeps every request small enough to
// fit provider rate/token limits (e.g. Groq's free-tier 12k TPM) while still
// producing the full report schema.
type chunkedAIProvider struct {
	inner    AIProvider
	maxBytes int
}

func (c *chunkedAIProvider) Generate(ctx context.Context, systemPrompt, payloadJSON string, schema map[string]any) (string, error) {
	if len(payloadJSON) <= c.maxBytes {
		return c.inner.Generate(ctx, systemPrompt, payloadJSON, schema)
	}

	var payload reportNumbersPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[ai] chunk parse failed (%v), falling back to single call", err)
		return c.inner.Generate(ctx, systemPrompt, payloadJSON, schema)
	}
	if len(payload.Regions) == 0 {
		return c.inner.Generate(ctx, systemPrompt, payloadJSON, schema)
	}

	regionSchema := todayRegionItemSchema()
	regionPrompt := todayRegionPrompt()
	if payload.Type == "compare" {
		regionSchema = compareRegionItemSchema()
		regionPrompt = compareRegionPrompt()
	}

	// Phase 1: analyze each UTC region independently.
	analyses := make([]map[string]any, 0, len(payload.Regions))
	for _, r := range payload.Regions {
		regionJSON, err := json.Marshal(map[string]any{
			"region": r.Region,
			"day_a":  r.DayA,
			"day_b":  r.DayB,
		})
		if err != nil {
			return "", fmt.Errorf("marshal region payload: %w", err)
		}
		text, err := c.inner.Generate(ctx, regionPrompt, string(regionJSON), regionSchema)
		if err != nil {
			return "", fmt.Errorf("region %q analysis: %w", r.Region, err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(text), &obj); err != nil {
			return "", fmt.Errorf("region %q analysis invalid JSON: %w", r.Region, err)
		}
		obj["region"] = r.Region
		analyses = append(analyses, obj)
	}

	// Phase 2: assemble the full report from whole-day data + region analyses.
	assembly := map[string]any{
		"type":            payload.Type,
		"day_a":           payload.DayA,
		"day_b":           payload.DayB,
		"generated_at":    payload.GeneratedAt,
		"alerts_a":        payload.AlertsA,
		"alerts_b":        payload.AlertsB,
		"incidents":       payload.Incidents,
		"service_health":  payload.ServiceHealth,
		"hosts":           payload.Hosts,
		"fleet":           payload.Fleet,
		"fleet_day_b":     payload.FleetDayB,
		"deltas":          payload.Deltas,
		"region_analyses": analyses,
	}
	assemblyJSON, err := json.Marshal(assembly)
	if err != nil {
		return "", fmt.Errorf("marshal assembly payload: %w", err)
	}

	assemblyPrompt := systemPrompt + `
ASSEMBLY INSTRUCTION:
The payload includes a "region_analyses" array with one pre-computed analysis per UTC region
(the exact objects your output schema expects under "regions").
Your "regions" output array MUST contain each of these region objects verbatim —
copy them as-is, do not rewrite, drop, or invent regions. Everything else
(overall_status, executive_summary, key_findings, alerts insight, recommendations, outlook / deltas)
must be derived only from the numbers in this payload.`

	return c.inner.Generate(ctx, assemblyPrompt, string(assemblyJSON), schema)
}

// NewAIProvider builds the provider chain from config. The configured provider
// is primary; if the other vendor's key is also set it becomes the fallback.
// Large payloads are automatically chunked per UTC region to stay within the
// primary provider's token/rate limits.
func NewAIProvider(provider, geminiKey, geminiModel, groqKey, groqModel string, chunkBytes int) AIProvider {
	gemini := NewGeminiClient(geminiKey, geminiModel)
	groq := NewGroqClient(groqKey, groqModel)

	var inner AIProvider
	if provider == "groq" {
		inner = &fallbackAIProvider{primary: groq, secondary: gemini}
	} else {
		inner = &fallbackAIProvider{primary: gemini, secondary: groq}
	}
	if chunkBytes <= 0 {
		chunkBytes = 35000
	}
	return &chunkedAIProvider{inner: inner, maxBytes: chunkBytes}
}

func todayRegionPrompt() string {
	return `You are a senior SRE / DevOps analyst for a multi-tenant observability platform.
You analyze telemetry for ONE fixed UTC time region of a daily report and produce that region's section.
The region input contains "day_a" with per-service-kind metrics (nginx/apache/postgresql/mariadb/redis/phpfpm),
fleet aggregates, log_signals (error/warning log counts) and alerts_fired.

Return ONLY valid JSON for this region:
- region: the exact region name given
- summary: 1-2 sentences describing the region's health and any anomalies
- highlights: short strings of notable positive/stable observations
- concerns: short strings of actual problems (spikes, saturation, errors) seen in the data
- notable_metrics: array of {service, metric, value} for the most telling numbers

GROUND-TRUTH RULES:
- ONLY reference numbers that literally appear in the input. Quote values exactly WITH units.
- If a service/region has no data (null/empty), say "no data" — never guess.
- Distinguish noise (single spike) from patterns (sustained across hosts/services).
- No markdown, no prose outside the JSON object.`
}

func compareRegionPrompt() string {
	return `You are a senior SRE / DevOps analyst for a multi-tenant observability platform.
You analyze telemetry for ONE fixed UTC time region of a day-comparison report.
The region input contains "day_a" and "day_b" windows (each with per-service-kind metrics
nginx/apache/postgresql/mariadb/redis/phpfpm, fleet, log_signals, alerts_fired).

Return ONLY valid JSON for this region:
- region: the exact region name given
- summary: 1-2 sentences describing what changed between day_a and day_b in this region
- biggest_differences: array of {metric, day_a, day_b, delta_pct, direction: up|down|flat} for the largest changes

GROUND-TRUTH RULES:
- ONLY reference numbers that literally appear in the input, including any delta_pct already provided.
  Never recompute or invent percentages.
- Quote values exactly WITH units. delta_pct null means "not measurable".
- Distinguish real change from noise (small delta on low traffic = noise).
- No markdown, no prose outside the JSON object.`
}
