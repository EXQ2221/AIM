package knowledgeimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

type parserServiceResponse struct {
	Title                 string `json:"title"`
	SourceType            string `json:"sourceType"`
	Content               string `json:"content"`
	FileType              string `json:"fileType"`
	ImageCount            int    `json:"imageCount"`
	UsedVisionDescription bool   `json:"usedVisionDescription"`
}

func ParseViaService(ctx context.Context, filename string, contentType string, data []byte, title string) (*ParsedDocument, error) {
	baseURL := strings.TrimSpace(os.Getenv("PARSER_SERVICE_URL"))
	if baseURL == "" {
		baseURL = "http://parser-service:8000"
	}
	url := strings.TrimRight(baseURL, "/") + "/v1/parse"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if title = strings.TrimSpace(title); title != "" {
		if err := writer.WriteField("title", title); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	timeout := 10 * time.Minute
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("X-Original-Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("parser service unavailable: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(raw))
		var detail struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(raw, &detail) == nil && strings.TrimSpace(detail.Detail) != "" {
			message = strings.TrimSpace(detail.Detail)
		}
		if message == "" {
			message = "parser service request failed"
		}
		return nil, errors.New(message)
	}

	var parsed parserServiceResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid parser response: %w", err)
	}

	content := strings.TrimSpace(parsed.Content)
	if content == "" {
		return nil, errors.New("document content is empty")
	}
	sourceType := strings.ToUpper(strings.TrimSpace(parsed.SourceType))
	if sourceType != "TEXT" && sourceType != "MARKDOWN" {
		sourceType = "TEXT"
	}

	return &ParsedDocument{
		Title:      strings.TrimSpace(parsed.Title),
		SourceType: sourceType,
		Content:    content,
		FileType:   strings.TrimSpace(parsed.FileType),
		ImageCount: parsed.ImageCount,
		UsedVision: parsed.UsedVisionDescription,
	}, nil
}
