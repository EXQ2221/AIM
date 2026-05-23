package botdal

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

type RerankResult struct {
	Index          int
	RelevanceScore float64
}

type TextReranker interface {
	Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error)
}

type DashScopeCompatibleReranker struct {
	baseURL    string
	apiKey     string
	model      string
	instruct   string
	httpClient *http.Client
}

func NewDashScopeCompatibleReranker(baseURL, apiKey, model string, timeout time.Duration, insecureSkipVerify bool) (*DashScopeCompatibleReranker, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if baseURL == "" {
		return nil, fmt.Errorf("rerank base url is empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("rerank api key is empty")
	}
	if model == "" {
		return nil, fmt.Errorf("rerank model is empty")
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &DashScopeCompatibleReranker{
		baseURL:  baseURL,
		apiKey:   apiKey,
		model:    model,
		instruct: "Given a web search query, retrieve relevant passages that answer the query.",
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
			},
		},
	}, nil
}

func (c *DashScopeCompatibleReranker) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if c == nil {
		return nil, fmt.Errorf("rerank client is nil")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	cleanDocs := make([]string, 0, len(documents))
	for _, item := range documents {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		cleanDocs = append(cleanDocs, value)
	}
	if len(cleanDocs) == 0 {
		return nil, nil
	}

	payload := map[string]interface{}{
		"model":     c.model,
		"query":     query,
		"documents": cleanDocs,
	}
	if topN > 0 {
		payload["top_n"] = topN
	}
	if c.instruct != "" {
		payload["instruct"] = c.instruct
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/reranks", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read rerank response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("rerank request failed: status=%d %s: %s", resp.StatusCode, resp.Status, safeExcerpt(respBody))
	}

	var parsed struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
		Output struct {
			Results []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse rerank response failed: %w", err)
	}

	source := parsed.Results
	if len(source) == 0 {
		source = parsed.Output.Results
	}
	results := make([]RerankResult, 0, len(source))
	for _, item := range source {
		results = append(results, RerankResult{
			Index:          item.Index,
			RelevanceScore: normalizeScore(item.RelevanceScore),
		})
	}
	return results, nil
}

func normalizeScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func safeExcerpt(raw []byte) string {
	const limit = 1024
	content := strings.TrimSpace(string(raw))
	if len(content) <= limit {
		return content
	}
	return content[:limit]
}

var _ TextReranker = (*DashScopeCompatibleReranker)(nil)

