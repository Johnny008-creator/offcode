package tools

import (
	"strings"
	"testing"
)

// sample.pdf/sample.docx (internal/tools/testdata/) were both generated
// from the same source Markdown via `soffice --headless --convert-to`,
// so both should extract the same two paragraphs.

func TestExtractPDF(t *testing.T) {
	text, err := Read("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Read(sample.pdf): %v", err)
	}
	for _, want := range []string{"Test Document", "normal text for extraction testing", "line ordering"} {
		if !strings.Contains(text, want) {
			t.Errorf("extracted PDF text missing %q\ngot: %s", want, text)
		}
	}
}

func TestExtractDOCX(t *testing.T) {
	text, err := Read("testdata/sample.docx")
	if err != nil {
		t.Fatalf("Read(sample.docx): %v", err)
	}
	for _, want := range []string{"Test Document", "normal text for extraction testing", "line ordering"} {
		if !strings.Contains(text, want) {
			t.Errorf("extracted DOCX text missing %q\ngot: %s", want, text)
		}
	}
}

func TestReadPlainTextUnaffected(t *testing.T) {
	// Guard against the extension-based dispatch in Read() accidentally
	// swallowing ordinary text files.
	text, err := Read("docformats.go")
	if err != nil {
		t.Fatalf("Read(docformats.go): %v", err)
	}
	if !strings.Contains(text, "package tools") {
		t.Errorf("plain-text read looks wrong, got %d bytes starting %q", len(text), text[:min(40, len(text))])
	}
}
