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
	"strconv"
	"strings"
	"time"

	"example.com/aim/shared/errno"
)

type parserServiceResponse struct {
	Title                 string `json:"title"`
	SourceType            string `json:"sourceType"`
	Content               string `json:"content"`
	FileType              string `json:"fileType"`
	ImageCount            int    `json:"imageCount"`
	UsedVisionDescription bool   `json:"usedVisionDescription"`
	Chunks                []struct {
		Index        int    `json:"index"`
		ChunkType    string `json:"chunkType"`
		SectionTitle string `json:"sectionTitle"`
		Content      string `json:"content"`
		PageStart    int    `json:"pageStart"`
		PageEnd      int    `json:"pageEnd"`
		CharStart    int    `json:"charStart"`
		CharEnd      int    `json:"charEnd"`
		Sentences    []struct {
			SentenceIndex int    `json:"sentenceIndex"`
			Text          string `json:"text"`
			PageStart     int    `json:"pageStart"`
			PageEnd       int    `json:"pageEnd"`
			CharStart     int    `json:"charStart"`
			CharEnd       int    `json:"charEnd"`
		} `json:"sentences"`
	} `json:"chunks"`
}

func ParseViaService(ctx context.Context, filename string, contentType string, data []byte, title string) (*ParsedDocument, error) {
	return ParseViaServiceFromReader(ctx, filename, contentType, bytes.NewReader(data), title)
}

func ParseViaServiceFromReader(ctx context.Context, filename string, contentType string, reader io.Reader, title string) (*ParsedDocument, error) {
	baseURL := strings.TrimSpace(os.Getenv("PARSER_SERVICE_URL"))
	if baseURL == "" {
		baseURL = "http://parser-service:8000"
	}
	url := strings.TrimRight(baseURL, "/") + "/v1/parse"

	timeout := parseDurationEnv("KNOWLEDGE_IMPORT_PARSE_HTTP_TIMEOUT", 30*time.Minute)
	client := &http.Client{Timeout: timeout}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	if reader == nil {
		err := errors.New("source reader is nil")
		_ = pw.CloseWithError(err)
		return nil, err
	}
	go func() {
		if err := writeParserMultipart(writer, reader, filename, title); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		_ = pw.CloseWithError(err)
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("X-Original-Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		_ = pw.CloseWithError(err)
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
		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, errno.Internal(message)
		}
		return nil, errno.BadRequest(message)
	}

	var parsed parserServiceResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid parser response: %w", err)
	}

	content := strings.TrimSpace(parsed.Content)
	if content == "" {
		return nil, errno.BadRequest("document content is empty")
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
		Chunks:     normalizeParsedChunks(parsed.Chunks),
	}, nil
}

func writeParserMultipart(writer *multipart.Writer, reader io.Reader, filename string, title string) error {
	if writer == nil {
		return errors.New("multipart writer is nil")
	}
	if strings.TrimSpace(title) != "" {
		if err := writer.WriteField("title", strings.TrimSpace(title)); err != nil {
			return err
		}
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return err
	}
	return writer.Close()
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return fallback
}

func normalizeParsedChunks(raw []struct {
	Index        int    `json:"index"`
	ChunkType    string `json:"chunkType"`
	SectionTitle string `json:"sectionTitle"`
	Content      string `json:"content"`
	PageStart    int    `json:"pageStart"`
	PageEnd      int    `json:"pageEnd"`
	CharStart    int    `json:"charStart"`
	CharEnd      int    `json:"charEnd"`
	Sentences    []struct {
		SentenceIndex int    `json:"sentenceIndex"`
		Text          string `json:"text"`
		PageStart     int    `json:"pageStart"`
		PageEnd       int    `json:"pageEnd"`
		CharStart     int    `json:"charStart"`
		CharEnd       int    `json:"charEnd"`
	} `json:"sentences"`
}) []ParsedChunk {
	if len(raw) == 0 {
		return nil
	}
	result := make([]ParsedChunk, 0, len(raw))
	for idx, item := range raw {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		sectionTitle := strings.TrimSpace(item.SectionTitle)
		if sectionTitle == "" {
			sectionTitle = fmt.Sprintf("Chunk %d", idx+1)
		}
		sentences := make([]ParsedSentence, 0, len(item.Sentences))
		for _, sentence := range item.Sentences {
			text := strings.TrimSpace(sentence.Text)
			if text == "" {
				continue
			}
			sentences = append(sentences, ParsedSentence{
				SentenceIndex: sentence.SentenceIndex,
				Text:          text,
				PageStart:     sentence.PageStart,
				PageEnd:       sentence.PageEnd,
				CharStart:     sentence.CharStart,
				CharEnd:       sentence.CharEnd,
			})
		}
		result = append(result, ParsedChunk{
			Index:        item.Index,
			ChunkType:    strings.ToUpper(strings.TrimSpace(item.ChunkType)),
			SectionTitle: sectionTitle,
			Content:      content,
			PageStart:    item.PageStart,
			PageEnd:      item.PageEnd,
			CharStart:    item.CharStart,
			CharEnd:      item.CharEnd,
			Sentences:    sentences,
		})
	}
	return result
}
