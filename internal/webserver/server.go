// Package webserver exposes an internal/webengine.Session over HTTP, with
// Server-Sent Events for live state updates, backing the `offcode web`
// subcommand. One server instance holds exactly one session, matching how
// the TUI treats a working directory as a single ongoing conversation —
// multiple browser tabs pointed at the same server just view/drive the
// same session, like multiple terminals attached to one tmux pane.
package webserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
	"github.com/Johnny008-creator/offcode/internal/webengine"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	sess *webengine.Session
	cfg  config.Config
	mux  *http.ServeMux
}

func New(cfg config.Config) *Server {
	s := &Server{sess: webengine.NewSession(cfg), cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func listen(addr string, cfg config.Config) (*Server, net.Listener, error) {
	s := New(cfg)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	return s, l, nil
}

// Serve starts the HTTP server on addr (expected to be 127.0.0.1:PORT —
// this is a local dev tool, never meant to be exposed beyond localhost)
// and blocks until it exits. Used by the `offcode web` subcommand.
func Serve(addr string, cfg config.Config) error {
	s, l, err := listen(addr, cfg)
	if err != nil {
		return err
	}
	fmt.Printf("offcode web: listening on http://%s (local only)\n", l.Addr().String())
	return http.Serve(l, s.mux)
}

// Start binds addr immediately (so a bind failure, e.g. "port already in
// use", surfaces synchronously) and serves in a background goroutine,
// returning right away. Used by the TUI's /web command, where starting a
// web server must not block the chat loop.
func Start(addr string, cfg config.Config) (actualAddr string, err error) {
	s, l, err := listen(addr, cfg)
	if err != nil {
		return "", err
	}
	go func() {
		_ = http.Serve(l, s.mux)
	}()
	return l.Addr().String(), nil
}

func (s *Server) routes() {
	static, err := fs.Sub(staticFS, "static")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(static)))
	}

	s.mux.HandleFunc("/api/state", s.handleState)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/models", s.handleModels)
	s.mux.HandleFunc("/api/model", s.handleSetModel)
	s.mux.HandleFunc("/api/mode", s.handleSetMode)
	s.mux.HandleFunc("/api/message", s.handleMessage)
	s.mux.HandleFunc("/api/research", s.handleResearch)
	s.mux.HandleFunc("/api/confirm", s.handleConfirm)
	s.mux.HandleFunc("/api/answer", s.handleAnswer)
	s.mux.HandleFunc("/api/remember", s.handleRemember)
	s.mux.HandleFunc("/api/stop", s.handleStop)
	s.mux.HandleFunc("/api/usage", s.handleUsage)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.sess.Snapshot())
}

// handleEvents is the SSE stream: it sends the full current state on
// connect, then a fresh snapshot every time anything changes. Sending the
// whole state each time (rather than diffing) trades a little bandwidth —
// irrelevant on localhost — for an entire class of client/server drift
// bugs that a diff-based protocol would risk.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsubscribe := s.sess.Subscribe()
	defer unsubscribe()

	send := func() {
		data, _ := json.Marshal(s.sess.Snapshot())
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	send()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			send()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	type modelOut struct {
		Tag         string `json:"tag"`
		DisplayName string `json:"display_name"`
		Tagline     string `json:"tagline"`
		Installed   bool   `json:"installed"`
		Known       bool   `json:"known"`
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	client := backend.NewClient(s.cfg.BaseURL)
	installed, _ := client.ListModels(ctx, s.cfg.Backend)
	installedSet := map[string]bool{}
	for _, tag := range installed {
		installedSet[tag] = true
	}

	var out []modelOut
	seen := map[string]bool{}
	for _, t := range tiers.Known {
		out = append(out, modelOut{Tag: t.Tag, DisplayName: t.DisplayName, Tagline: t.Tagline, Installed: installedSet[t.Tag], Known: true})
		seen[t.Tag] = true
	}
	for _, tag := range installed {
		if !seen[tag] {
			out = append(out, modelOut{Tag: tag, DisplayName: tag, Installed: true})
		}
	}
	writeJSON(w, out)
}

type modelReq struct {
	Model string `json:"model"`
}

func (s *Server) handleSetModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req modelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model == "" {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"model\":\"...\"}")
		return
	}
	s.sess.SetModel(req.Model)
	writeJSON(w, s.sess.Snapshot())
}

type modeReq struct {
	Mode string `json:"mode"`
}

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req modeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !s.sess.SetMode(req.Mode) {
		httpError(w, http.StatusBadRequest, "mode must be one of: plan, default, auto")
		return
	}
	writeJSON(w, s.sess.Snapshot())
}

type messageReq struct {
	Text string `json:"text"`
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req messageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"text\":\"...\"}")
		return
	}
	if !s.sess.Submit(req.Text) {
		httpError(w, http.StatusConflict, "a turn is already in progress")
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type researchReq struct {
	Question string `json:"question"`
	Depth    string `json:"depth"`
}

func (s *Server) handleResearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req researchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"question\":\"...\",\"depth\":\"lite|normal|deep\"}")
		return
	}
	prompt, filename, depthName := webengine.ResearchPrompt(req.Question, req.Depth)
	display := fmt.Sprintf("/research %s (depth=%s) → %s", req.Question, depthName, filename)
	if !s.sess.SubmitDisplay(display, prompt) {
		httpError(w, http.StatusConflict, "a turn is already in progress")
		return
	}
	writeJSON(w, map[string]string{"ok": "true", "filename": filename})
}

type confirmReq struct {
	Allow bool `json:"allow"`
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req confirmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"allow\":true|false}")
		return
	}
	if !s.sess.Confirm(req.Allow) {
		httpError(w, http.StatusConflict, "no pending confirmation")
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type answerReq struct {
	Answer string `json:"answer"`
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req answerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Answer == "" {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"answer\":\"...\"}")
		return
	}
	if !s.sess.AnswerAsk(req.Answer) {
		httpError(w, http.StatusConflict, "no pending question")
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type rememberReq struct {
	Note string `json:"note"`
}

func (s *Server) handleRemember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req rememberReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Note == "" {
		httpError(w, http.StatusBadRequest, "expected JSON body {\"note\":\"...\"}")
		return
	}
	if err := webengine.RememberNote(req.Note); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	s.sess.Stop()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	records, err := usagelog.ReadAll()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summaries := usagelog.Summarize(records, time.Time{})
	report := usagelog.RenderReport(summaries, "Usage (all-time)")
	writeJSON(w, map[string]string{"report": report})
}
