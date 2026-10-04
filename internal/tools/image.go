package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
}

// ReadImageBase64 reads an image file and returns its base64-encoded
// bytes (no data: URI prefix — Ollama's native API wants the raw
// base64 string in a message's "images" array). Rejects non-image
// extensions rather than guessing from content, since a wrong guess
// here means silently sending garbage to a vision model.
func ReadImageBase64(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if !imageExtensions[ext] {
		return "", fmt.Errorf("%q doesn't look like an image file (expected .png, .jpg, .jpeg, .gif, .webp, or .bmp)", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("%s is empty", path)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
