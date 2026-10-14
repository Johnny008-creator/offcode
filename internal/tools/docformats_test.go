package tools

import (
	"archive/zip"
	"os"
	"path/filepath"
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

func TestExtractDOCXIncludesTablesLinksAndBreaks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
 <w:p><w:r><w:t>Before </w:t></w:r><w:hyperlink><w:r><w:t>link</w:t></w:r></w:hyperlink></w:p>
 <w:tbl><w:tr><w:tc><w:p><w:r><w:t>Export</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>42</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
 <w:p><w:r><w:t>After</w:t><w:tab/><w:t>tab</w:t><w:br/><w:t>line</w:t></w:r></w:p>
 </w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	text, err := extractDOCX(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Before link\nExport\n42\nAfter\ttab\nline" {
		t.Fatalf("unexpected text: %q", text)
	}
}
