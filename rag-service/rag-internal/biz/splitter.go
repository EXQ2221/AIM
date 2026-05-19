package rag

import (
	"errors"
	"strings"

	ragconf "example.com/aim/rag-service/rag-internal/conf"
	ragmodel "example.com/aim/rag-service/rag-internal/model"
)

type SplitterConfig = ragmodel.SplitterConfig
type Chunk = ragmodel.Chunk

func LoadSplitterConfigFromEnv() (SplitterConfig, error) {
	return ragconf.LoadSplitterConfigFromEnv()
}

func SplitText(content string, cfg SplitterConfig) ([]Chunk, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, errors.New("document content is empty")
	}
	if cfg.ChunkSize <= 0 {
		return nil, errors.New("chunk size must be positive")
	}
	if cfg.ChunkOverlap < 0 || cfg.ChunkOverlap >= cfg.ChunkSize {
		return nil, errors.New("chunk overlap must be >= 0 and < chunk size")
	}

	runes := []rune(trimmed)
	if len(runes) <= cfg.ChunkSize {
		return []Chunk{{Index: 0, Content: string(runes)}}, nil
	}

	step := cfg.ChunkSize - cfg.ChunkOverlap
	chunks := make([]Chunk, 0, (len(runes)+step-1)/step)
	for start, index := 0, 0; start < len(runes); start, index = start+step, index+1 {
		end := start + cfg.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		part := strings.TrimSpace(string(runes[start:end]))
		if part == "" {
			continue
		}
		chunks = append(chunks, Chunk{
			Index:   index,
			Content: part,
		})
		if end == len(runes) {
			break
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("no chunk generated")
	}
	return chunks, nil
}
