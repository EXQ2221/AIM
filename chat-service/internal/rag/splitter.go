package rag

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type SplitterConfig struct {
	ChunkSize    int
	ChunkOverlap int
}

type Chunk struct {
	Index   int
	Content string
}

func LoadSplitterConfigFromEnv() (SplitterConfig, error) {
	cfg := SplitterConfig{
		ChunkSize:    1000,
		ChunkOverlap: 150,
	}

	if value := strings.TrimSpace(os.Getenv("RAG_CHUNK_SIZE")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return SplitterConfig{}, errors.New("RAG_CHUNK_SIZE must be a positive integer")
		}
		cfg.ChunkSize = parsed
	}

	if value := strings.TrimSpace(os.Getenv("RAG_CHUNK_OVERLAP")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return SplitterConfig{}, errors.New("RAG_CHUNK_OVERLAP must be a non-negative integer")
		}
		cfg.ChunkOverlap = parsed
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		return SplitterConfig{}, errors.New("RAG_CHUNK_OVERLAP must be smaller than RAG_CHUNK_SIZE")
	}
	return cfg, nil
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
