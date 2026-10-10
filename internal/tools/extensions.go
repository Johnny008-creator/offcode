package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/media"
)

func extensionDefinitions() []backend.Tool {
	var defs []backend.Tool
	for _, kind := range []string{"image", "video", "music"} {
		defs = append(defs, backend.Tool{Type: "function", Function: backend.FunctionDef{Name: "generate_" + kind, Description: "Generate " + kind + " using a configured local ComfyUI workflow. Requires ~/.offcode/media.json. Output path must be new and have the workflow's output extension.", Parameters: schema(`{"prompt":{"type":"string"},"path":{"type":"string"}}`, "prompt", "path")}})
	}
	for _, d := range []struct {
		name, description, props string
		required                 []string
	}{
		{"export_document", "Convert a Markdown file to PDF or DOCX using local Pandoc (PDF also requires a PDF engine). Output must be new.", `{"source":{"type":"string"},"path":{"type":"string"}}`, []string{"source", "path"}},
		{"transcribe_audio", "Transcribe an audio file locally using whisper-cli and OFFCODE_WHISPER_MODEL; ffmpeg converts to mono WAV first.", `{"path":{"type":"string"},"language":{"type":"string","description":"Language code; defaults to auto"}}`, []string{"path"}},
		{"synthesize_speech", "Convert text to a WAV file using local espeak-ng. Output must be new. Voice defaults to en; cs supports Czech.", `{"text":{"type":"string"},"path":{"type":"string"},"language":{"type":"string"}}`, []string{"text", "path"}},
		{"repo_map", "Summarize project structure and Go symbols without an embedding model.", `{"path":{"type":"string"}}`, nil},
		{"git_summary", "Show the current branch, status and staged/unstaged diff summary for preparing a review or PR.", `{"path":{"type":"string"}}`, nil},
	} {
		defs = append(defs, backend.Tool{Type: "function", Function: backend.FunctionDef{Name: d.name, Description: d.description, Parameters: schema(d.props, d.required...)}})
	}
	return defs
}

func dispatchExtension(parent context.Context, name, args string) (string, error) {
	var a struct{ Prompt, Path, Source, Text, Language string }
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Minute)
	defer cancel()
	switch name {
	case "generate_image", "generate_video", "generate_music":
		path, err := media.Generate(ctx, strings.TrimPrefix(name, "generate_"), a.Prompt, a.Path)
		if err != nil {
			return "", err
		}
		return artifactResult(path)
	case "export_document":
		if err := ExportDocument(ctx, a.Source, a.Path); err != nil {
			return "", err
		}
		return artifactResult(a.Path)
	case "synthesize_speech":
		if err := Synthesize(ctx, a.Text, a.Path, a.Language); err != nil {
			return "", err
		}
		return artifactResult(a.Path)
	case "transcribe_audio":
		return Transcribe(ctx, a.Path, a.Language)
	case "repo_map":
		return RepoMap(a.Path)
	case "git_summary":
		return GitSummary(ctx, a.Path)
	}
	return "", fmt.Errorf("unknown tool: %q", name)
}

func artifactResult(path string) (string, error) {
	data, err := json.Marshal(map[string]string{"artifact": path})
	return string(data), err
}

// produceFile preserves any existing output and only publishes successful output.
func produceFile(path string, produce func(string) error) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("output must be a new file: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".offcode-output-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "output"+filepath.Ext(path))
	if err := produce(tmp); err != nil {
		return err
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("producer returned an empty file")
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	closeErr := out.Close()
	if err != nil {
		os.Remove(path)
		return err
	}
	if closeErr != nil {
		os.Remove(path)
	}
	return closeErr
}

func runLocal(ctx context.Context, name string, args []string, input string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, out)
	}
	return nil
}

func ExportDocument(ctx context.Context, source, path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".pdf" && ext != ".docx" {
		return fmt.Errorf("output must end in .pdf or .docx")
	}
	if source == "" {
		return fmt.Errorf("source Markdown file is required")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	return produceFile(path, func(tmp string) error {
		args := []string{"--from=markdown", "--standalone", "--output", tmp}
		if ext == ".pdf" {
			engine := os.Getenv("OFFCODE_PDF_ENGINE")
			if engine == "" {
				engine = "xelatex"
			}
			args = append(args, "--pdf-engine", engine)
		}
		args = append(args, "--", source)
		return runLocal(ctx, "pandoc", args, "")
	})
}

func Synthesize(ctx context.Context, text, path, voice string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text is required")
	}
	if strings.ToLower(filepath.Ext(path)) != ".wav" {
		return fmt.Errorf("speech output must end in .wav")
	}
	if voice == "" {
		voice = "en"
	}
	return produceFile(path, func(tmp string) error {
		return runLocal(ctx, "espeak-ng", []string{"--stdin", "-v", voice, "-w", tmp}, text)
	})
}

func Transcribe(ctx context.Context, path, language string) (string, error) {
	model := os.Getenv("OFFCODE_WHISPER_MODEL")
	if model == "" {
		return "", fmt.Errorf("set OFFCODE_WHISPER_MODEL to a local whisper.cpp model file")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if language == "" {
		language = "auto"
	}
	dir, err := os.MkdirTemp("", "offcode-transcribe-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "input.wav")
	prefix := filepath.Join(dir, "transcript")
	if err := runLocal(ctx, "ffmpeg", []string{"-nostdin", "-v", "error", "-i", path, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav}, ""); err != nil {
		return "", err
	}
	if err := runLocal(ctx, "whisper-cli", []string{"-m", model, "-f", wav, "-l", language, "-otxt", "-of", prefix}, ""); err != nil {
		return "", err
	}
	text, err := os.ReadFile(prefix + ".txt")
	return strings.TrimSpace(string(text)), err
}
