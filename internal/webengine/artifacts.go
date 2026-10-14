package webengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

// ArtifactPath exposes only artifacts produced by this session's tools,
// never an arbitrary file path from an HTTP query.
func (s *Session) ArtifactPath(block int, expected ...string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.matchesSession(expected) || block < 0 || block >= len(s.blocks) || s.blocks[block].artifactPath == "" {
		return "", false
	}
	return s.blocks[block].artifactPath, true
}

func artifactBlock(tc backend.ToolCall, content string, isError bool, index int) Block {
	b := Block{Kind: BlockToolResult, ToolName: tc.Function.Name, Text: content, IsError: isError}
	if isError {
		return b
	}
	switch tc.Function.Name {
	case "generate_image", "generate_video", "generate_music", "synthesize_speech", "export_document":
	default:
		return b
	}
	var result struct {
		Artifact string `json:"artifact"`
	}
	if json.Unmarshal([]byte(content), &result) != nil || result.Artifact == "" {
		return b
	}
	path, err := filepath.Abs(result.Artifact)
	if err != nil {
		return b
	}
	b.artifactPath = path
	b.ArtifactURL = fmt.Sprintf("/api/artifact?block=%d", index)
	b.ArtifactName = filepath.Base(path)
	return b
}

func diffPreview(tc backend.ToolCall) string {
	var a struct{ Path, Content, Old_String, New_String string }
	if json.Unmarshal([]byte(tc.Function.Arguments), &a) != nil {
		return ""
	}
	var old, new string
	switch tc.Function.Name {
	case "edit":
		old, new = a.Old_String, a.New_String
	case "write":
		info, err := os.Stat(a.Path)
		if err == nil && info.Size() > 1<<20 {
			return "(existing file too large for preview)"
		}
		data, err := os.ReadFile(a.Path)
		if err != nil && !os.IsNotExist(err) {
			return "(cannot read existing file for preview)"
		}
		old, new = string(data), a.Content
	default:
		return ""
	}
	before, after := strings.Split(old, "\n"), strings.Split(new, "\n")
	start := 0
	for start < len(before) && start < len(after) && before[start] == after[start] {
		start++
	}
	end := 0
	for end < len(before)-start && end < len(after)-start && before[len(before)-1-end] == after[len(after)-1-end] {
		end++
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", a.Path)
	count := 0
	for i, lines := range [][]string{before[start : len(before)-end], after[start : len(after)-end]} {
		prefix := "- "
		if i == 1 {
			prefix = "+ "
		}
		for _, line := range lines {
			if count >= 80 || b.Len() > 12000 {
				b.WriteString("… preview truncated\n")
				return b.String()
			}
			fmt.Fprintln(&b, prefix+line)
			count++
		}
	}
	if count == 0 {
		b.WriteString("(no textual change)")
	}
	return b.String()
}
