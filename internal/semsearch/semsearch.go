// Package semsearch indexes a project's text files into overlapping
// chunks, embeds them via a local embedding model (Ollama's native
// /api/embed — nomic-embed-text), and answers queries by cosine similarity. Complements grep
// (exact string match) rather than replacing it: this finds "the
// function that validates a session token" by meaning, not literal
// text.
//
// The index is cached at <cwd>/.offcode/semindex.json and kept in sync
// incrementally — re-embedding only files that are new or changed since
// the last index (by mtime+size), dropping entries for files that no
// longer exist — so only the first search in a project pays the full
// embedding cost.
package semsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
)

const (
	// EmbedModel is the embedding model used by this package.
	// A different embedding model would need a full re-index (vectors
	// from different models aren't comparable), so this isn't a
	// per-call option — keeping it fixed avoids ever silently mixing
	// incompatible vectors in one index.
	EmbedModel = "nomic-embed-text"

	chunkLines      = 60
	chunkOverlap    = 15
	maxFileBytes    = 300_000
	maxIndexedFiles = 3000
	embedBatchSize  = 64
)

var skipDirs = map[string]bool{
	".git": true, ".offcode": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".next": true, "__pycache__": true,
	".venv": true, "venv": true, "target": true,
}

// textExtensions is an allowlist, not a blacklist — deliberately, since
// guessing "is this binary" from content is unreliable and a false
// positive (indexing binary garbage) is worse than a false negative
// (skipping an unusual but real text file).
var textExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".java": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cc": true,
	".rs": true, ".rb": true, ".php": true, ".cs": true, ".swift": true, ".kt": true,
	".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".sh": true, ".bash": true, ".sql": true, ".html": true, ".css": true, ".scss": true,
	".vue": true, ".svelte": true, ".lua": true, ".ex": true, ".exs": true,
}

type Chunk struct {
	File      string    `json:"file"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Text      string    `json:"text"`
	Vector    []float32 `json:"vector"`
}

type fileMeta struct {
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
}

type index struct {
	Model  string              `json:"model"`
	Files  map[string]fileMeta `json:"files"`
	Chunks []Chunk             `json:"chunks"`
}

func indexPath(cwd string) string {
	return filepath.Join(cwd, ".offcode", "semindex.json")
}

func loadIndex(cwd string) *index {
	data, err := os.ReadFile(indexPath(cwd))
	if err != nil {
		return &index{Model: EmbedModel, Files: map[string]fileMeta{}}
	}
	var idx index
	if err := json.Unmarshal(data, &idx); err != nil || idx.Model != EmbedModel {
		return &index{Model: EmbedModel, Files: map[string]fileMeta{}}
	}
	if idx.Files == nil {
		idx.Files = map[string]fileMeta{}
	}
	return &idx
}

func saveIndex(cwd string, idx *index) error {
	dir := filepath.Join(cwd, ".offcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "semindex-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), indexPath(cwd))
}

// walkTextFiles returns every indexable file under cwd, relative paths.
func walkTextFiles(cwd string) ([]string, error) {
	info, err := os.Stat(cwd)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root is not a directory: %s", cwd)
	}
	var files []string
	err = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than failing the whole walk
		}
		if d.IsDir() {
			if path != cwd && (skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !textExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
			return nil
		}
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}
		files = append(files, rel)
		if len(files) >= maxIndexedFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return files, err
}

func chunkFile(path string) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil, nil
	}

	var chunks []Chunk
	step := chunkLines - chunkOverlap
	for start := 0; start < len(lines); start += step {
		end := start + chunkLines
		if end > len(lines) {
			end = len(lines)
		}
		text := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
		if text != "" {
			chunks = append(chunks, Chunk{StartLine: start + 1, EndLine: end, Text: text})
		}
		if end >= len(lines) {
			break
		}
	}
	return chunks, nil
}

// Sync brings the cached index for cwd up to date: re-embeds any file
// that's new or changed since the last sync, drops entries for files
// that no longer exist, and leaves everything else untouched. Returns
// how many files were (re-)embedded, for a progress-ish note to the
// user on the first, slow run.
func Sync(ctx context.Context, client *backend.Client, cwd string) (reembedded int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	idx := loadIndex(cwd)
	currentFiles, err := walkTextFiles(cwd)
	if err != nil {
		return 0, err
	}
	currentSet := make(map[string]bool, len(currentFiles))

	var toEmbed []string
	for _, rel := range currentFiles {
		currentSet[rel] = true
		info, err := os.Stat(filepath.Join(cwd, rel))
		if err != nil {
			continue
		}
		meta, known := idx.Files[rel]
		if known && meta.ModTime.Equal(info.ModTime()) && meta.Size == info.Size() {
			continue // unchanged since last sync
		}
		toEmbed = append(toEmbed, rel)
	}

	// Drop chunks for files that changed (will be re-added below) or
	// vanished entirely.
	dirty := len(toEmbed) > 0 || len(idx.Files) != len(currentSet)
	if dirty {
		changedSet := make(map[string]bool, len(toEmbed))
		for _, f := range toEmbed {
			changedSet[f] = true
		}
		kept := make([]Chunk, 0, len(idx.Chunks))
		for _, c := range idx.Chunks {
			if changedSet[c.File] || !currentSet[c.File] {
				continue
			}
			kept = append(kept, c)
		}
		idx.Chunks = kept
	}
	for rel := range idx.Files {
		if !currentSet[rel] {
			delete(idx.Files, rel)
		}
	}

	for _, rel := range toEmbed {
		if err := ctx.Err(); err != nil {
			return reembedded, err
		}
		// Capture metadata before reading: an edit during embedding must be
		// picked up on the next sync, not marked as already indexed.
		delete(idx.Files, rel)
		info, err := os.Stat(filepath.Join(cwd, rel))
		if err != nil {
			continue
		}
		chunks, err := chunkFile(filepath.Join(cwd, rel))
		if err != nil {
			continue
		}
		for i := range chunks {
			chunks[i].File = rel
		}
		if err := embedChunks(ctx, client, chunks); err != nil {
			return reembedded, fmt.Errorf("embedding %s: %w", rel, err)
		}
		idx.Chunks = append(idx.Chunks, chunks...)

		idx.Files[rel] = fileMeta{ModTime: info.ModTime(), Size: info.Size()}
		reembedded++
	}

	if dirty {
		if err := saveIndex(cwd, idx); err != nil {
			return reembedded, fmt.Errorf("save index: %w", err)
		}
	}
	return reembedded, nil
}

func embedChunks(ctx context.Context, client *backend.Client, chunks []Chunk) error {
	for start := 0; start < len(chunks); start += embedBatchSize {
		end := min(start+embedBatchSize, len(chunks))
		texts := make([]string, end-start)
		for i := range texts {
			texts[i] = chunks[start+i].Text
		}
		vecs, err := client.Embed(ctx, EmbedModel, texts)
		if err != nil {
			return err
		}
		for i, v := range vecs {
			chunks[start+i].Vector = v
		}
	}
	return nil
}

type Match struct {
	File      string  `json:"file"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
}

// Search syncs the index (embedding anything new/changed first — see
// Sync) then returns the topK chunks most similar to query by cosine
// similarity.
func Search(ctx context.Context, client *backend.Client, cwd, query string, topK int) ([]Match, int, error) {
	if strings.TrimSpace(query) == "" {
		return nil, 0, fmt.Errorf("query must not be empty")
	}
	if topK <= 0 {
		topK = 8
	}
	topK = min(topK, 50)
	reembedded, err := Sync(ctx, client, cwd)
	if err != nil {
		return nil, reembedded, err
	}

	idx := loadIndex(cwd)
	if len(idx.Chunks) == 0 {
		return nil, reembedded, nil
	}

	qvecs, err := client.Embed(ctx, EmbedModel, []string{query})
	if err != nil {
		return nil, reembedded, err
	}
	qvec := qvecs[0]

	matches := make([]Match, len(idx.Chunks))
	for i, c := range idx.Chunks {
		matches[i] = Match{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine, Text: c.Text, Score: cosineSimilarity(qvec, c.Vector)}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })

	if topK > 0 && topK < len(matches) {
		matches = matches[:topK]
	}
	return matches, reembedded, nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
