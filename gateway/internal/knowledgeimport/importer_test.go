package knowledgeimport

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestParseTextDocument(t *testing.T) {
	doc, err := Parse("notes.txt", "text/plain", []byte("\xEF\xBB\xBFhello\nworld\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if doc.SourceType != "TEXT" {
		t.Fatalf("expected TEXT source type, got %q", doc.SourceType)
	}
	if doc.Content != "hello\nworld" {
		t.Fatalf("unexpected text content: %q", doc.Content)
	}
}

func TestParseDOCXDocument(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	file, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>hello</w:t></w:r></w:p>
    <w:p><w:r><w:t>world</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if _, err := file.Write([]byte(xmlData)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	doc, err := Parse("notes.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data.Bytes())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if doc.SourceType != "TEXT" {
		t.Fatalf("expected TEXT source type, got %q", doc.SourceType)
	}
	if doc.Content != "hello\n\nworld" {
		t.Fatalf("unexpected docx content: %q", doc.Content)
	}
}

func TestParseLegacyDocRejected(t *testing.T) {
	_, err := Parse("legacy.doc", "application/msword", []byte("dummy"))
	if err == nil {
		t.Fatal("expected error for legacy .doc")
	}
}

func TestParseTextDocumentRemovesNUL(t *testing.T) {
	doc, err := Parse("nul.txt", "text/plain", []byte("a\x00b\nc"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if doc.Content != "ab\nc" {
		t.Fatalf("unexpected sanitized content: %q", doc.Content)
	}
}
