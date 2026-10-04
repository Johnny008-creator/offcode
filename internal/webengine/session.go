// Package webengine is the backend-agnostic agent engine behind `offcode
// web`. It is a deliberate parallel implementation to internal/tui, not a
// refactor of it: the TUI's loop is written in Bubble Tea's
// continuation-passing style (every step returns control to an event
// loop), which doesn't translate to a plain HTTP/SSE server. Running the
// turn loop as a single blocking goroutine per session instead makes the
// control flow here much more direct — confirmation and ask_user pauses
// are just a blocking channel receive, no state machine needed.
package webengine

import (
	"context"
	"os"
	"sync"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/session"
)

const maxToolIterationsPerTurn = 25
const maxPlanNudges = 2

type BlockKind string

const (
	BlockUser       BlockKind = "user"
	BlockAssistant  BlockKind = "assistant"
	BlockReasoning  BlockKind = "reasoning"
	BlockToolCall   BlockKind = "tool_call"
	BlockToolResult BlockKind = "tool_result"
	BlockSystem     BlockKind = "system"
	BlockError      BlockKind = "error"
)

type Block struct {
	Kind     BlockKind `json:"kind"`
	Text     string    `json:"text,omitempty"`
	ToolName string    `json:"tool_name,omitempty"`
	ToolArgs string    `json:"tool_args,omitempty"`
	IsError  bool      `json:"is_error,omitempty"`
}

type PlanStepView struct {
	Text   string `json:"text"`
	Status string `json:"status"`
}

type ConfirmView struct {
	ToolName string `json:"tool_name"`
	ToolArgs string `json:"tool_args"`
}

type AskView struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Recommended int      `json:"recommended"`
}

type State struct {
	Backend      string         `json:"backend"`
	Model        string         `json:"model"`
	ModelDisplay string         `json:"model_display"`
	Mode         string         `json:"mode"`
	Status       string         `json:"status"` // idle | generating | confirming | asking
	Activity     string         `json:"activity"`
	Blocks       []Block        `json:"blocks"`
	Plan         []PlanStepView `json:"plan"`
	Confirm      *ConfirmView   `json:"confirm,omitempty"`
	Ask          *AskView       `json:"ask,omitempty"`
	TokensPerSec float64        `json:"tokens_per_sec"`
	ResumedCount int            `json:"resumed_count"`
}

type Session struct {
	mu     sync.Mutex
	cfg    config.Config
	client *backend.Client
	think  backend.ThinkLevel
	cwd    string

	messages []backend.Message
	blocks   []Block
	plan     []planStep

	status   string
	activity string

	pendingConfirmTC *backend.ToolCall
	confirmCh        chan bool

	pendingAskTC          *backend.ToolCall
	pendingAskOptions     []string
	pendingAskQuestion    string
	pendingAskRecommended int
	askCh                 chan string

	cancelFn context.CancelFunc

	lastFailedCall  string
	repeatFailCount int
	readPaths       map[string]bool
	toolIterations  int
	planNudges      int

	tokensPerSec float64
	resumedCount int

	subs   map[int]chan struct{}
	nextID int
}

func NewSession(cfg config.Config) *Session {
	cwd, _ := os.Getwd()

	messages := []backend.Message{{Role: "system", Content: buildSystemPrompt(cfg.Model)}}
	resumedCount := 0
	var resumedBlocks []Block
	if cfg.Fresh {
		_ = session.Clear(cwd)
	} else if saved, ok, err := session.Load(cwd); err == nil && ok && len(saved.Messages) > 0 {
		messages = append(messages, saved.Messages...)
		resumedCount = len(saved.Messages)
		for _, msg := range saved.Messages {
			switch msg.Role {
			case "user":
				if msg.Content != "" {
					resumedBlocks = append(resumedBlocks, Block{Kind: BlockUser, Text: msg.Content})
				}
			case "assistant":
				if msg.Content != "" {
					resumedBlocks = append(resumedBlocks, Block{Kind: BlockAssistant, Text: msg.Content})
				}
			}
		}
	}

	blocks := []Block{}
	if resumedCount > 0 {
		blocks = append(blocks, Block{Kind: BlockSystem, Text: "resumed session (" + itoa(resumedCount) + " earlier messages)"})
		blocks = append(blocks, resumedBlocks...)
	}

	return &Session{
		cfg:          cfg,
		client:       backend.NewClient(cfg.BaseURL),
		cwd:          cwd,
		messages:     messages,
		blocks:       blocks,
		status:       "idle",
		readPaths:    map[string]bool{},
		resumedCount: resumedCount,
		subs:         map[int]chan struct{}{},
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Subscribe registers for change notifications. The returned channel
// receives a (non-blocking, coalescing) signal on every state change —
// callers should re-fetch Snapshot() on each signal rather than expecting
// one signal per change. Call the returned function to unsubscribe.
func (s *Session) Subscribe() (<-chan struct{}, func()) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	ch := make(chan struct{}, 1)
	s.subs[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}
}

// notify must be called with s.mu held.
func (s *Session) notify() {
	for _, ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Session) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Session) snapshotLocked() State {
	plan := make([]PlanStepView, len(s.plan))
	for i, p := range s.plan {
		plan[i] = PlanStepView{Text: p.Text, Status: p.Status.String()}
	}

	st := State{
		Backend:      s.cfg.Backend,
		Model:        s.cfg.Model,
		ModelDisplay: identityName(s.cfg.Model),
		Mode:         string(s.cfg.Mode),
		Status:       s.status,
		Activity:     s.activity,
		Blocks:       append([]Block(nil), s.blocks...),
		Plan:         plan,
		TokensPerSec: s.tokensPerSec,
		ResumedCount: s.resumedCount,
	}
	if s.pendingConfirmTC != nil {
		st.Confirm = &ConfirmView{ToolName: s.pendingConfirmTC.Function.Name, ToolArgs: s.pendingConfirmTC.Function.Arguments}
	}
	if s.pendingAskTC != nil {
		st.Ask = &AskView{Question: s.pendingAskQuestion, Options: s.pendingAskOptions, Recommended: s.pendingAskRecommended}
	}
	if st.Model == "" {
		st.ModelDisplay = ""
	}
	return st
}

func (s *Session) pushBlock(b Block) {
	s.mu.Lock()
	s.blocks = append(s.blocks, b)
	s.notify()
	s.mu.Unlock()
}

// SetModel sets the active model (used by the web UI's model picker when
// no model was given on the command line) and rewrites the system prompt
// in place, matching the TUI's /model behavior.
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	s.cfg.Model = model
	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		s.messages[0].Content = buildSystemPrompt(model)
	}
	s.notify()
	s.mu.Unlock()
}

func (s *Session) SetMode(mode string) bool {
	switch mode {
	case "plan", "default", "auto":
	default:
		return false
	}
	s.mu.Lock()
	s.cfg.Mode = permission.Mode(mode)
	s.notify()
	s.mu.Unlock()
	return true
}

func (s *Session) IsBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status != "idle"
}

// Stop cancels an in-flight generation, if any.
func (s *Session) Stop() {
	s.mu.Lock()
	cancel := s.cancelFn
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Session) saveToDisk() {
	if s.cwd == "" || len(s.messages) <= 1 {
		return
	}
	_ = session.Save(s.cwd, session.Saved{
		Model:    s.cfg.Model,
		Messages: append([]backend.Message(nil), s.messages[1:]...),
	})
}

// Submit starts a new turn with the given user text. Returns false (doing
// nothing) if a turn is already in progress.
func (s *Session) Submit(text string) bool {
	s.mu.Lock()
	if s.status != "idle" {
		s.mu.Unlock()
		return false
	}
	s.status = "generating"
	s.messages = append(s.messages, backend.Message{Role: "user", Content: text})
	s.blocks = append(s.blocks, Block{Kind: BlockUser, Text: text})
	s.toolIterations = 0
	s.planNudges = 0
	s.notify()
	s.mu.Unlock()

	go s.runTurn()
	return true
}

// SubmitDisplay is Submit but with a separate, shorter string shown in the
// transcript than the one actually sent to the model (used by /research
// via the HTTP layer, mirroring the TUI's submitUserMessageDisplay).
func (s *Session) SubmitDisplay(display, actual string) bool {
	s.mu.Lock()
	if s.status != "idle" {
		s.mu.Unlock()
		return false
	}
	s.status = "generating"
	s.messages = append(s.messages, backend.Message{Role: "user", Content: actual})
	s.blocks = append(s.blocks, Block{Kind: BlockUser, Text: display})
	s.toolIterations = 0
	s.planNudges = 0
	s.notify()
	s.mu.Unlock()

	go s.runTurn()
	return true
}

// Confirm resolves a pending tool-confirmation request.
func (s *Session) Confirm(allow bool) bool {
	s.mu.Lock()
	ch := s.confirmCh
	s.confirmCh = nil
	s.mu.Unlock()
	if ch == nil {
		return false
	}
	ch <- allow
	return true
}

// AnswerAsk resolves a pending ask_user request.
func (s *Session) AnswerAsk(answer string) bool {
	s.mu.Lock()
	ch := s.askCh
	s.askCh = nil
	s.mu.Unlock()
	if ch == nil {
		return false
	}
	ch <- answer
	return true
}
