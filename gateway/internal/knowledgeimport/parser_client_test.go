package knowledgeimport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type chunkedReader struct {
	data  []byte
	chunk int
	reads int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := r.chunk
	if n <= 0 || n > len(p) {
		n = len(p)
	}
	if n > len(r.data) {
		n = len(r.data)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	r.reads++
	return n, nil
}

func TestParseViaServiceFromReader(t *testing.T) {
	var gotTitle string
	var gotFile string
	var gotContentType string
	var gotMultipartType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("X-Original-Content-Type")
		gotMultipartType = r.Header.Get("Content-Type")

		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("MultipartReader returned error: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for {
			part, err := reader.NextPart()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Errorf("NextPart returned error: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			body, readErr := io.ReadAll(part)
			part.Close()
			if readErr != nil {
				t.Errorf("ReadAll returned error: %v", readErr)
				http.Error(w, readErr.Error(), http.StatusBadRequest)
				return
			}
			switch part.FormName() {
			case "title":
				gotTitle = string(body)
			case "file":
				gotFile = string(body)
			}
		}

		if gotTitle != "streamed title" {
			t.Errorf("unexpected title: %q", gotTitle)
		}
		if gotFile != strings.Repeat("chunk-", 16000) {
			t.Errorf("unexpected file content length: %d", len(gotFile))
		}
		if gotContentType != "text/plain" {
			t.Errorf("unexpected original content type: %q", gotContentType)
		}
		if !strings.HasPrefix(gotMultipartType, "multipart/form-data;") {
			t.Errorf("unexpected multipart content type: %q", gotMultipartType)
		}

		_ = json.NewEncoder(w).Encode(parserServiceResponse{
			Title:      "streamed title",
			SourceType: "TEXT",
			Content:    "parsed content",
			FileType:   "text",
		})
	}))
	defer server.Close()

	t.Setenv("PARSER_SERVICE_URL", server.URL)

	payload := strings.Repeat("chunk-", 16000)
	reader := &chunkedReader{
		data:  []byte(payload),
		chunk: 1024,
	}

	doc, err := ParseViaServiceFromReader(context.Background(), "notes.txt", "text/plain", reader, "streamed title")
	if err != nil {
		t.Fatalf("ParseViaServiceFromReader returned error: %v", err)
	}
	if doc.Title != "streamed title" {
		t.Fatalf("unexpected document title: %q", doc.Title)
	}
	if doc.SourceType != "TEXT" {
		t.Fatalf("unexpected source type: %q", doc.SourceType)
	}
	if doc.Content != "parsed content" {
		t.Fatalf("unexpected content: %q", doc.Content)
	}
	if reader.reads <= 1 {
		t.Fatalf("expected multiple reader reads, got %d", reader.reads)
	}
}
