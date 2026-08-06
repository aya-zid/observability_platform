// internal/service/groq_client.go
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

// GroqClient is a thin wrapper around the Groq OpenAI-compatible chat
// completions API. It requests JSON output via response_format json_object and
// embeds the schema into the user message (Groq has no server-side schema
// binding like Gemini's responseSchema). The API key lives only on the server.
type GroqClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewGroqClient(apiKey, model string) *GroqClient {
	return &GroqClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 120 * time.Second},
	}
}

// Generate sends the system prompt plus the numeric payload JSON and returns the
// raw JSON text produced by the model.
func (c *GroqClient) Generate(ctx context.Context, systemPrompt, payloadJSON string, schema map[string]any) (string, error) {
	if c.apiKey == "" {
		return "", errGeminiNotConfigured
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("marshal groq schema: %w", err)
	}

	request := map[string]any{
		"model": c.model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{
				"role": "user",
				"content": fmt.Sprintf(
					"INPUT DATA (JSON):\n%s\n\nRESPONSE SCHEMA — return ONLY a JSON object matching this schema exactly:\n%s",
					payloadJSON, string(schemaJSON),
				),
			},
		},
		"response_format": map[string]any{"type": "json_object"},
	}

	raw, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal groq request: %w", err)
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

func (c *GroqClient) postAndExtract(ctx context.Context, rawBody []byte) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("groq read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq API error: status %d: %s", resp.StatusCode, truncateBytes(body, 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("groq parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("groq error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("groq returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
