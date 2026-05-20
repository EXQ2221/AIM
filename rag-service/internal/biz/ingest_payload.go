package rag

import (
	"encoding/json"
	"regexp"
	"strings"
)

type ingestPayload struct {
	Version int                `json:"version"`
	Content string             `json:"content"`
	Chunks  []IngestChunkInput `json:"chunks"`
}

func parseIngestPayload(raw string) (string, []IngestChunkInput) {
	content := strings.TrimSpace(raw)
	if content == "" {
		return "", nil
	}

	var payload ingestPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return content, nil
	}
	normalizedContent := strings.TrimSpace(payload.Content)
	if normalizedContent == "" {
		normalizedContent = content
	}

	chunks := make([]IngestChunkInput, 0, len(payload.Chunks))
	for idx, item := range payload.Chunks {
		chunkContent := strings.TrimSpace(item.Content)
		if chunkContent == "" {
			continue
		}
		chunkType := strings.ToUpper(strings.TrimSpace(item.ChunkType))
		if chunkType == "" {
			chunkType = "PLAIN_TEXT"
		}
		sectionTitle := strings.TrimSpace(item.SectionTitle)
		if sectionTitle == "" {
			sectionTitle = "Chunk"
		}
		chunkIndex := item.Index
		if chunkIndex < 0 {
			chunkIndex = idx
		}
		chunks = append(chunks, IngestChunkInput{
			Index:        chunkIndex,
			ChunkType:    chunkType,
			SectionTitle: sectionTitle,
			Content:      chunkContent,
		})
	}
	return normalizedContent, chunks
}

var questionNoPattern = regexp.MustCompile(`(?:第\s*)?(\d{1,6})\s*题?`)

func questionNoFromSection(section string) int {
	section = strings.TrimSpace(section)
	if section == "" {
		return 0
	}
	matches := questionNoPattern.FindStringSubmatch(section)
	if len(matches) != 2 {
		return 0
	}
	value := strings.TrimSpace(matches[1])
	if value == "" {
		return 0
	}
	number := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		number = number*10 + int(ch-'0')
	}
	if number <= 0 {
		return 0
	}
	return number
}
