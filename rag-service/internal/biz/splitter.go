package rag

import (
	"errors"
	"strings"

	ragconf "example.com/aim/rag-service/internal/conf"
	ragmodel "example.com/aim/rag-service/internal/dal/model"
)

type SplitterConfig = ragmodel.SplitterConfig
type Chunk = ragmodel.Chunk
type DocumentType = ragmodel.DocumentType
type ChunkMetadata = ragmodel.ChunkMetadata

const (
	DocumentTypePlainText    = ragmodel.DocumentTypePlainText
	DocumentTypeMarkdown     = ragmodel.DocumentTypeMarkdown
	DocumentTypeScript       = ragmodel.DocumentTypeScript
	DocumentTypeQuestionBank = ragmodel.DocumentTypeQuestionBank
)

func LoadSplitterConfigFromEnv() (SplitterConfig, error) {
	return ragconf.LoadSplitterConfigFromEnv()
}

func SplitText(content string, cfg SplitterConfig) ([]Chunk, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, errors.New("document content is empty")
	}
	if cfg.ChunkSize <= 0 {
		return nil, errors.New("chunk size must be positive")
	}

	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		paragraphs = []string{text}
	}

	result := make([]Chunk, 0, len(paragraphs))
	var current strings.Builder
	currentIndex := 0
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if current.Len() > 0 && current.Len()+2+len([]rune(paragraph)) > cfg.ChunkSize {
			result = append(result, Chunk{
				Index:   currentIndex,
				Content: strings.TrimSpace(current.String()),
				Metadata: ChunkMetadata{
					DocumentType: DocumentTypePlainText,
				},
			})
			current.Reset()
			currentIndex++
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}

	if strings.TrimSpace(current.String()) != "" {
		result = append(result, Chunk{
			Index:   currentIndex,
			Content: strings.TrimSpace(current.String()),
			Metadata: ChunkMetadata{
				DocumentType: DocumentTypePlainText,
			},
		})
	}

	if len(result) == 0 {
		return nil, errors.New("document content is empty")
	}
	return result, nil
}

func splitParagraphs(content string) []string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	var current strings.Builder

	flush := func() {
		part := strings.TrimSpace(current.String())
		if part != "" {
			result = append(result, part)
		}
		current.Reset()
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	flush()
	return result
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
