// internal/service/gemini_client.go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GeminiClient is a thin wrapper around the Gemini generateContent REST API that
// enforces structured JSON output via responseSchema. The API key lives only on
// the server.
type GeminiClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewGeminiClient(apiKey, model string) *GeminiClient {
	return &GeminiClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 120 * time.Second},
	}
}

var errGeminiNotConfigured = errors.New("AI is not configured on this platform")

// Generate sends the system prompt plus the numeric payload JSON and returns the
// raw JSON text produced by the model. The returned text is validated to be
// parseable JSON; if the first response is malformed the call is retried once.
func (c *GeminiClient) Generate(ctx context.Context, systemPrompt, payloadJSON string, schema map[string]any) (string, error) {
	if c.apiKey == "" {
		return "", errGeminiNotConfigured
	}

	request := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]any{{"text": systemPrompt}},
		},
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": payloadJSON}}},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema":   schema,
		},
	}

	raw, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	text, err := c.postAndExtract(ctx, raw)
	if err != nil {
		return "", err
	}

	// Retry once on malformed JSON output.
	if !json.Valid([]byte(text)) {
		text, err = c.postAndExtract(ctx, raw)
		if err != nil {
			return "", err
		}
	}
	if !json.Valid([]byte(text)) {
		return "", errors.New("AI returned malformed JSON after retry")
	}
	return text, nil
}

func (c *GeminiClient) postAndExtract(ctx context.Context, rawBody []byte) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gemini read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error: status %d: %s", resp.StatusCode, truncateBytes(body, 300))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("gemini parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("gemini error: %s", parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini returned no candidates")
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

func truncateBytes(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}

// ─── Response schemas (Gemini structured output) ─────────────────────────────

func strSchema() map[string]any {
	return map[string]any{"type": "STRING"}
}

func strEnumSchema(values ...string) map[string]any {
	return map[string]any{"type": "STRING", "enum": values}
}

func strArrSchema() map[string]any {
	return map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}}
}

func recommendationSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"priority":        strEnumSchema("critical", "high", "medium", "low"),
			"title":           strSchema(),
			"action":          strSchema(),
			"rationale":       strSchema(),
			"expected_impact": strSchema(),
			"effort":          strEnumSchema("low", "medium", "high"),
		},
		"required": []string{"priority", "title", "action", "rationale", "expected_impact", "effort"},
	}
}

func keyFindingItemSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"severity": strEnumSchema("critical", "warning", "info"),
			"title":    strSchema(),
			"detail":   strSchema(),
		},
		"required": []string{"severity", "title", "detail"},
	}
}

func todayRegionItemSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"region":  strSchema(),
			"summary": strSchema(),
			"highlights": map[string]any{
				"type":  "ARRAY",
				"items": map[string]any{"type": "STRING"},
			},
			"concerns": map[string]any{
				"type":  "ARRAY",
				"items": map[string]any{"type": "STRING"},
			},
			"notable_metrics": map[string]any{
				"type": "ARRAY",
				"items": map[string]any{
					"type": "OBJECT",
					"properties": map[string]any{
						"service": strSchema(),
						"metric":  strSchema(),
						"value":   strSchema(),
					},
					"required": []string{"service", "metric", "value"},
				},
			},
		},
		"required": []string{"region", "summary"},
	}
}

func TodayReportSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"overall_status":    strEnumSchema("healthy", "degraded", "critical"),
			"executive_summary": strSchema(),
			"key_findings": map[string]any{
				"type":  "ARRAY",
				"items": keyFindingItemSchema(),
			},
			"regions": map[string]any{
				"type":  "ARRAY",
				"items": todayRegionItemSchema(),
			},
			"alerts_insight":  strSchema(),
			"recommendations": map[string]any{"type": "ARRAY", "items": recommendationSchema()},
			"outlook":         strSchema(),
		},
		"required": []string{"overall_status", "executive_summary", "key_findings", "regions", "alerts_insight", "recommendations", "outlook"},
	}
}

// compareKeyFindingItemSchema extends the shared finding shape with the delta
// fields for day-comparison reports.
func compareKeyFindingItemSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"severity":    strEnumSchema("critical", "warning", "info"),
			"title":       strSchema(),
			"detail":      strSchema(),
			"day_a_value": strSchema(),
			"day_b_value": strSchema(),
			"delta_pct":   map[string]any{"type": "NUMBER"},
		},
		"required": []string{"severity", "title", "detail", "day_a_value", "day_b_value"},
	}
}

func compareRegionItemSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"region":  strSchema(),
			"summary": strSchema(),
			"biggest_differences": map[string]any{
				"type": "ARRAY",
				"items": map[string]any{
					"type": "OBJECT",
					"properties": map[string]any{
						"metric":    strSchema(),
						"day_a":     strSchema(),
						"day_b":     strSchema(),
						"delta_pct": map[string]any{"type": "NUMBER"},
						"direction": strEnumSchema("up", "down", "flat"),
					},
					"required": []string{"metric", "day_a", "day_b"},
				},
			},
		},
		"required": []string{"region", "summary"},
	}
}

func CompareReportSchema() map[string]any {
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"overall_status":    strEnumSchema("healthy", "degraded", "critical"),
			"executive_summary": strSchema(),
			"key_findings": map[string]any{
				"type":  "ARRAY",
				"items": compareKeyFindingItemSchema(),
			},
			"regions": map[string]any{
				"type":  "ARRAY",
				"items": compareRegionItemSchema(),
			},
			"service_comparisons": map[string]any{
				"type": "ARRAY",
				"items": map[string]any{
					"type": "OBJECT",
					"properties": map[string]any{
						"service": strSchema(),
						"metrics": map[string]any{
							"type": "ARRAY",
							"items": map[string]any{
								"type": "OBJECT",
								"properties": map[string]any{
									"metric":    strSchema(),
									"day_a":     strSchema(),
									"day_b":     strSchema(),
									"delta_pct": map[string]any{"type": "NUMBER"},
								},
								"required": []string{"metric", "day_a", "day_b"},
							},
						},
					},
					"required": []string{"service", "metrics"},
				},
			},
			"alerts_diff":     strSchema(),
			"recommendations": map[string]any{"type": "ARRAY", "items": recommendationSchema()},
		},
		"required": []string{"overall_status", "executive_summary", "key_findings", "regions", "service_comparisons", "alerts_diff", "recommendations"},
	}
}
