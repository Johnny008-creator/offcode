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

// extractDOCX returns body text in XML reading order, including table cells and
// hyperlinks. Formatting, images, headers and footers are not extracted.
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

	const maxDocumentXML = 32 << 20
	if docXML.UncompressedSize64 > maxDocumentXML {
		return "", fmt.Errorf("document XML exceeds 32 MiB extraction limit")
	}
	decoder := xml.NewDecoder(io.LimitReader(rc, maxDocumentXML+1))
	var text strings.Builder
	inBody := false
	const wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse word/document.xml: %w", err)
		}
		switch node := token.(type) {
		case xml.StartElement:
			if node.Name.Space != wordNS {
				continue
			}
			if node.Name.Local == "body" {
				inBody = true
				continue
			}
			if !inBody {
				continue
			}
			switch node.Name.Local {
			case "t":
				var value string
				if err := decoder.DecodeElement(&value, &node); err != nil {
					return "", fmt.Errorf("parse document text: %w", err)
				}
				text.WriteString(value)
			case "tab":
				text.WriteByte('\t')
			case "br", "cr":
				text.WriteByte('\n')
			}
		case xml.EndElement:
			if node.Name.Space != wordNS {
				continue
			}
			if node.Name.Local == "body" {
				inBody = false
			}
			if inBody && node.Name.Local == "p" {
				text.WriteByte('\n')
			}
		}
	}
	result := strings.TrimSpace(text.String())
	if result == "" {
		return "", fmt.Errorf("no extractable text found in document body")
	}
	return result, nil
}
