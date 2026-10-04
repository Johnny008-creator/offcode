package tools

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDF returns a PDF's plain text content. Formatting/layout is
// lost (this is a text-extraction tool for an LLM to read, not a
// renderer) — tables and multi-column layouts will read out of order.
func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}
	if _, err := buf.ReadFrom(b); err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("no extractable text (the PDF may be scanned images without OCR)")
	}
	return text, nil
}

// docxParagraph/docxRun/docxBody mirror just enough of word/document.xml's
// structure (a .docx is a zip of XML parts) to pull out paragraph text in
// reading order, without a full OOXML dependency.
type docxRun struct {
	Text []string `xml:"t"`
}
type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}
type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}
type docxDocument struct {
	Body docxBody `xml:"body"`
}

// extractDOCX returns a .docx's paragraph text, one paragraph per line.
// Tables, headers/footers, and images are not extracted — plain body
// text covers the overwhelming majority of "read me this document"
// requests, and the real OOXML structure is deep enough that chasing
// full fidelity isn't worth it for an LLM-facing text tool.
func extractDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx (expected a zip-based .docx file): %w", err)
	}
	defer zr.Close()

	var docXML *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("not a valid .docx: missing word/document.xml")
	}

	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("open word/document.xml: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read word/document.xml: %w", err)
	}

	var doc docxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse word/document.xml: %w", err)
	}

	var lines []string
	for _, p := range doc.Body.Paragraphs {
		var sb strings.Builder
		for _, r := range p.Runs {
			for _, t := range r.Text {
				sb.WriteString(t)
			}
		}
		lines = append(lines, sb.String())
	}

	text := strings.TrimSpace(strings.Join(lines, "\n"))
	if text == "" {
		return "", fmt.Errorf("no extractable text found in document body")
	}
	return text, nil
}
