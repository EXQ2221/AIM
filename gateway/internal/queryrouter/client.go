package queryrouter

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Planner interface {
	Plan(ctx context.Context, input PlanningInput) (*Plan, error)
}

type Config struct {
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

type HTTPPlanner struct {
	config     Config
	httpClient *http.Client
}

func NewHTTPPlanner(config Config) (*HTTPPlanner, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.Model = strings.TrimSpace(config.Model)
	if config.BaseURL == "" {
		return nil, fmt.Errorf("query router base url is empty")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("query router api key is empty")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("query router model is empty")
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}

	return &HTTPPlanner{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}, //nolint:gosec
			},
		},
	}, nil
}

func (p *HTTPPlanner) Plan(ctx context.Context, input PlanningInput) (*Plan, error) {
	if p == nil {
		return nil, fmt.Errorf("query router planner is nil")
	}
	messages, err := BuildMessages(input)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"model":       p.config.Model,
		"temperature": 0.1,
		"messages":    messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query router request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("read query router response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("query router request failed: status=%d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	content, err := extractCompletionContent(respBody)
	if err != nil {
		return nil, err
	}
	jsonBody, err := extractJSONObject(content)
	if err != nil {
		return nil, err
	}

	var plan Plan
	if err := json.Unmarshal([]byte(jsonBody), &plan); err != nil {
		return nil, fmt.Errorf("parse query router plan failed: %w", err)
	}
	normalized := plan.Normalized(input)
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("query router plan validation failed: %w", err)
	}
	return &normalized, nil
}

func extractCompletionContent(raw []byte) (string, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse query router response failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("query router response is empty")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("query router content is empty")
	}
	return content, nil
}

func extractJSONObject(content string) (string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```JSON")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("query router did not return a json object")
	}
	return content[start : end+1], nil
}
