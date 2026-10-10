package webserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Johnny008-creator/offcode/internal/tools"
)

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, 405, "POST only")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	dir, err := os.MkdirTemp("", "offcode-voice-")
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "recording.webm")
	f, err := os.Create(path)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	_, err = io.Copy(f, r.Body)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		httpError(w, 400, "invalid or oversized recording (limit 25 MiB)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	text, err := tools.Transcribe(ctx, path, "auto")
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]string{"text": text})
}

func (s *Server) handleSpeak(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, 405, "POST only")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64000)
	var a struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if json.NewDecoder(r.Body).Decode(&a) != nil {
		httpError(w, 400, "invalid speech request")
		return
	}
	dir, err := os.MkdirTemp("", "offcode-speech-")
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "speech.wav")
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	if err := tools.Synthesize(ctx, a.Text, path, a.Language); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}
