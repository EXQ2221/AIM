package knowledgeimport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"example.com/aim/shared/errno"
	"rsc.io/pdf"
)

const MaxDocumentBytes int64 = 20 << 20

type ParsedDocument struct {
	Title      string
	SourceType string
	Content    string
	FileType   string
	ImageCount int
	UsedVision bool
	Chunks     []ParsedChunk
}

type ParsedChunk struct {
	Index        int
	ChunkType    string
	SectionTitle string
	Content      string
	PageStart    int
	PageEnd      int
	CharStart    int
	CharEnd      int
	Sentences     []ParsedSentence
}

type ParsedSentence struct {
	SentenceIndex int
	Text          string
	PageStart     int
	PageEnd       int
	CharStart     int
	CharEnd       int
}

func Parse(filename string, contentType string, data []byte) (*ParsedDocument, error) {
	if len(data) == 0 {
		return nil, errno.BadRequest("file is empty")
	}

	title := strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if title == "" {
		title = "Imported Document"
	}

	fileType := detectFileType(filename, contentType, data)
	switch fileType {
	case "text":
		content, err := parseText(data)
		if err != nil {
			return nil, err
		}
		return &ParsedDocument{
			Title:      title,
			SourceType: "TEXT",
			Content:    content,
			FileType:   fileType,
		}, nil
	case "markdown":
		content, err := parseText(data)
		if err != nil {
			return nil, err
		}
		return &ParsedDocument{
			Title:      title,
			SourceType: "MARKDOWN",
			Content:    content,
			FileType:   fileType,
		}, nil
	case "docx":
		content, err := parseDOCX(data)
		if err != nil {
			return nil, err
		}
		return &ParsedDocument{
			Title:      title,
			SourceType: "TEXT",
			Content:    content,
			FileType:   fileType,
		}, nil
	case "pdf":
		content, err := parsePDF(data)
		if err != nil {
			return nil, err
		}
		return &ParsedDocument{
			Title:      title,
			SourceType: "TEXT",
			Content:    content,
			FileType:   fileType,
		}, nil
	case "doc":
		return nil, errno.BadRequest("legacy .doc is not supported yet, please convert it to .docx")
	default:
		return nil, errno.BadRequest("unsupported document type, only txt/md/pdf/docx are supported")
	}
}

func detectFileType(filename string, contentType string, data []byte) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	switch ext {
	case ".txt":
		return "text"
	case ".md", ".markdown":
		return "markdown"
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".doc":
		return "doc"
	}

	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if ct == "" || ct == "application/octet-stream" {
		sniffLen := len(data)
		if sniffLen > 512 {
			sniffLen = 512
		}
		ct = strings.ToLower(http.DetectContentType(data[:sniffLen]))
	}

	switch ct {
	case "text/plain":
		return "text"
	case "text/markdown":
		return "markdown"
	case "application/pdf":
		return "pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/msword":
		return "doc"
	}

	return ""
}

func parseText(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return "", errno.BadRequest("text document is not valid UTF-8")
	}
	content := normalizeExtractedText(string(data))
	if content == "" {
		return "", errno.BadRequest("document content is empty")
	}
	return content, nil
}

func parseDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("read docx archive failed: %w", err)
	}

	fileMap := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		fileMap[file.Name] = file
	}

	parts := []string{"word/document.xml", "word/footnotes.xml", "word/endnotes.xml"}
	var sections []string
	found := false
	for _, name := range parts {
		file := fileMap[name]
		if file == nil {
			continue
		}
		found = true
		text, err := extractOpenXMLText(file)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(text) != "" {
			sections = append(sections, text)
		}
	}
	if !found {
		return "", errno.BadRequest("docx document.xml not found")
	}

	content := normalizeExtractedText(strings.Join(sections, "\n\n"))
	if content == "" {
		return "", errno.BadRequest("no extractable text found in docx")
	}
	return content, nil
}

func extractOpenXMLText(file *zip.File) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open %s failed: %w", file.Name, err)
	}
	defer src.Close()

	decoder := xml.NewDecoder(src)
	var builder strings.Builder
	lastTokenWasBreak := false

	writeBreak := func(double bool) {
		if builder.Len() == 0 {
			return
		}
		if double {
			if !strings.HasSuffix(builder.String(), "\n\n") {
				if !strings.HasSuffix(builder.String(), "\n") {
					builder.WriteByte('\n')
				}
				builder.WriteByte('\n')
			}
			lastTokenWasBreak = true
			return
		}
		if !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteByte('\n')
		}
		lastTokenWasBreak = true
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("parse %s failed: %w", file.Name, err)
		}

		switch node := token.(type) {
		case xml.StartElement:
			switch node.Name.Local {
			case "t":
				var text string
				if err := decoder.DecodeElement(&text, &node); err != nil {
					return "", fmt.Errorf("decode %s text failed: %w", file.Name, err)
				}
				if text != "" {
					builder.WriteString(text)
					lastTokenWasBreak = false
				}
			case "tab":
				builder.WriteByte('\t')
				lastTokenWasBreak = false
			case "br", "cr":
				writeBreak(false)
			}
		case xml.EndElement:
			if node.Name.Local == "p" && !lastTokenWasBreak {
				writeBreak(true)
			}
		}
	}

	return builder.String(), nil
}

func parsePDF(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "aim-rag-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create pdf temp file failed: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write pdf temp file failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close pdf temp file failed: %w", err)
	}

	reader, err := pdf.Open(tmpPath)
	if err != nil {
		return "", fmt.Errorf("open pdf failed: %w", err)
	}

	var pages []string
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		if len(content.Text) == 0 {
			continue
		}
		pageText := layoutPDFText(content.Text)
		if strings.TrimSpace(pageText) != "" {
			pages = append(pages, pageText)
		}
	}

	content := normalizeExtractedText(strings.Join(pages, "\n\n"))
	if content == "" {
		return "", errno.BadRequest("no extractable text found in pdf")
	}
	return content, nil
}

func layoutPDFText(items []pdf.Text) string {
	texts := append([]pdf.Text(nil), items...)
	sort.Sort(pdf.TextVertical(texts))

	var builder strings.Builder
	var lastY float64
	var lastX float64
	var lastW float64
	for index, item := range texts {
		if strings.TrimSpace(item.S) == "" {
			continue
		}
		if index == 0 {
			builder.WriteString(item.S)
			lastY = item.Y
			lastX = item.X
			lastW = item.W
			continue
		}

		lineBreakThreshold := math.Max(item.FontSize*0.6, 2)
		paragraphBreakThreshold := math.Max(item.FontSize*1.4, 8)
		deltaY := math.Abs(item.Y - lastY)
		if deltaY > paragraphBreakThreshold {
			builder.WriteString("\n\n")
		} else if deltaY > lineBreakThreshold {
			builder.WriteByte('\n')
		} else {
			gap := item.X - (lastX + lastW)
			if gap > math.Max(item.FontSize*0.15, 2) {
				builder.WriteByte(' ')
			}
		}

		builder.WriteString(item.S)
		lastY = item.Y
		lastX = item.X
		lastW = item.W
	}

	return builder.String()
}

func normalizeExtractedText(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = sanitizeText(content)

	lines := strings.Split(content, "\n")
	cleaned := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount <= 1 {
				cleaned = append(cleaned, "")
			}
			continue
		}
		blankCount = 0
		cleaned = append(cleaned, line)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func sanitizeText(s string) string {
	// PostgreSQL UTF8 text cannot contain NUL bytes.
	// Also remove most C0 control chars except tab/newline.
	return strings.Map(func(r rune) rune {
		switch {
		case r == 0:
			return -1
		case r == '\t' || r == '\n':
			return r
		case r < 0x20:
			return -1
		default:
			return r
		}
	}, s)
}
