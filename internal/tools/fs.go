package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read returns a file's content as text. PDF and DOCX get their text
// extracted rather than returned as raw bytes (which would just be
// binary garbage to the model) — detected by extension, not content
// sniffing, since both formats are themselves zip/binary containers
// that don't have a simple magic-byte check worth the complexity here.
func Read(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return extractPDF(path)
	case ".docx":
		return extractDOCX(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Write(path string, content string) (string, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

func Edit(path, oldString, newString string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s does not exist: use write with path and content to create it, then use edit only for existing files", path)
		}
		return "", err
	}
	content := string(data)

	count := strings.Count(content, oldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	if count > 1 {
		return "", fmt.Errorf("old_string is not unique in %s (found %d occurrences)", path, count)
	}

	updated := strings.Replace(content, oldString, newString, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", path), nil
}
