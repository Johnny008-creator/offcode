package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Johnny008-creator/offcode/internal/agent"
	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/hooks"
	"github.com/Johnny008-creator/offcode/internal/mcpclient"
	"github.com/Johnny008-creator/offcode/internal/memory"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/session"
	"github.com/Johnny008-creator/offcode/internal/tools"
	"github.com/Johnny008-creator/offcode/internal/webserver"
)

const maxToolIterationsPerTurn = 25

const maxPlanNudges = 2

const AppVersion = "0.1.0"

// maxToolCallParseRetries bounds how many extra attempts a turn gets after
// Ollama fails to parse a model's own tool-call output against its chat
// template — see backend.IsMalformedToolCallResponse and the mirrored
// retry in internal/webengine/loop.go.
const maxToolCallParseRetries = 2

type phase int

const (
	phaseSplash phase = iota
	phaseChat
)

type state int

const (
	stateIdle state = iota
	stateGenerating
	stateConfirming
)

type Model struct {
	cfg    config.Config
	client *backend.Client

	messages []backend.Message

	phase phase
	state state

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	blocks           []block
	streamingContent string
	streamingReason  string

	width, height int
	ready         bool

	sessionStart     time.Time
	promptStart      time.Time
	turnStart        time.Time
	lastTurnDuration time.Duration
	tokensPerSec     float64
	lastUsage        backend.Usage
	contextLimit     int
	visionCapable    bool

	activity     string
	showThinking bool
	think        backend.ThinkLevel

	picker           pickerKind
	pickerItems      []string
	pickerCursor     int
	pickerReturnTo   pickerKind
	pickerLoading    bool
	pickerQuestion   string
	pickerAskOptions []string
	pendingAskUser   *backend.ToolCall

	planSteps              []planStep
	planNudges             int
	actionsSincePlanUpdate int

	pendingToolCalls     []backend.ToolCall
	pendingConfirm       *backend.ToolCall
	toolIterations       int
	readPaths            map[string]bool
	lastFailedCall       string
	repeatFailCount      int
	toolCallParseRetries int

	streamEvents <-chan backend.StreamEvent
	cancelGen    context.CancelFunc

	cwd            string
	sessionID      string
	resumedCount   int
	pickerSessions []session.Meta

	webAddr  string
	hooksCfg hooks.Config

	mcpMgr   *mcpclient.Manager
	mcpTools []backend.Tool

	quitting bool
}

func New(cfg config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "type your request..."
	ti.Prompt = "> "
	ti.PromptStyle = inputPromptStyle
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	cwd, _ := os.Getwd()
	hooksCfg, err := hooks.Load(cwd)
	if err != nil {
		hooksCfg = hooks.Config{}
	}

	messages := []backend.Message{{Role: "system", Content: buildSystemPrompt(cfg.Model)}}

	return Model{
		cfg:          cfg,
		client:       backend.NewClient(cfg.BaseURL),
		messages:     messages,
		phase:        phaseSplash,
		state:        stateIdle,
		input:        ti,
		spinner:      sp,
		sessionStart: time.Now(),
		showThinking: true,
		readPaths:    map[string]bool{},
		cwd:          cwd,
		sessionID:    session.NewID(),
		hooksCfg:     hooksCfg,
	}
}

// saveSession persists the conversation so far (everything but the system
// prompt, which gets rebuilt fresh on load) under this session's id.
// Best-effort: a failed save is silently ignored, same stance as usagelog
// and the write-safety checks elsewhere — persistence is a convenience,
// never something that should interrupt the chat loop.
func (m Model) saveSession() {
	if m.cwd == "" || m.sessionID == "" || len(m.messages) <= 1 {
		return
	}
	_ = session.Save(m.cwd, m.sessionID, session.Saved{
		Model:    m.cfg.Model,
		Messages: append([]backend.Message(nil), m.messages[1:]...),
	})
}

// afterSessionDecided continues startup (or a mid-run session switch) once
// it's settled whether a saved session was resumed — the one thing both
// paths still need is picking a model if none is set yet.
func (m Model) afterSessionDecided() (tea.Model, tea.Cmd) {
	if m.cfg.Model == "" {
		m.pushBlock(block{kind: blockSystem, text: "no model given — pick one to get started"})
		mm, cmd := m.openModelPicker(pickerNone)
		return mm, tea.Batch(tickCmd(), cmd)
	}
	return m, tickCmd()
}

func Run(cfg config.Config) error {
	// Deliberately no mouse motion capture: it would let the wheel scroll
	// the viewport, but it also disables the terminal's native click-drag
	// text selection, which matters more (copying output is a core use
	// case; keyboard scrolling already covers the rest).
	p := tea.NewProgram(New(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(splashCmd(m.client, m.cfg), m.spinner.Tick, connectMCPCmd(m.cwd))
}

// connectMCPCmd loads ~/.offcode/mcp.json + <cwd>/.offcode/mcp.json and
// connects to every configured MCP server in the background — this can
// involve starting subprocesses (and, for npx-based servers, fetching a
// package), so it must never block TUI startup. Absent config is the
// common case and resolves instantly with zero tools.
func connectMCPCmd(cwd string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := mcpclient.Load(cwd)
		if err != nil {
			return mcpConnectedMsg{warnings: []string{"couldn't read MCP config: " + err.Error()}}
		}
		if len(cfg.Servers) == 0 {
			return mcpConnectedMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		mgr, warnings := mcpclient.Connect(ctx, cfg)
		return mcpConnectedMsg{mgr: mgr, tools: mgr.Tools(), warnings: warnings}
	}
}

func splashCmd(c *backend.Client, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctxLen := c.ContextLength(context.Background(), cfg.Model)
		vision := c.SupportsVision(context.Background(), cfg.Model)
		if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
			time.Sleep(500*time.Millisecond - elapsed)
		}
		return splashDoneMsg{contextLimit: ctxLen, visionCapable: vision}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForStream(ch <-chan backend.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return backend.StreamEvent{Kind: backend.EventDone}
		}
		return ev
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if !m.ready {
			m.viewport = viewport.New(m.width, 3)
			m.ready = true
		}
		m.resizeViewport()
		m.input.Width = m.width - 4
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tickCmd()

	case splashDoneMsg:
		m.contextLimit = msg.contextLimit
		m.visionCapable = msg.visionCapable
		m.phase = phaseChat
		m.pushBlock(block{kind: blockBanner, text: renderBanner(m.cfg)})

		if m.cfg.Resume {
			if sessions, err := session.List(m.cwd); err == nil && len(sessions) > 0 {
				mm := m.openResumePicker(sessions)
				return mm, tickCmd()
			}
			m.pushBlock(block{kind: blockSystem, text: "no saved sessions for this directory yet — starting a new one"})
		}
		return m.afterSessionDecided()

	case backend.StreamEvent:
		return m.handleStreamEvent(msg)

	case toolExecutedMsg:
		return m.handleToolExecuted(msg)

	case modelsLoadedMsg:
		if m.picker != pickerModel {
			return m, nil
		}
		m.pickerLoading = false
		if msg.err != nil {
			m.picker = pickerNone
			m.pushBlock(block{kind: blockError, text: "failed to list models: " + msg.err.Error()})
			return m, nil
		}
		m.pickerItems = msg.models
		m.pickerCursor = 0
		for i, name := range msg.models {
			if name == m.cfg.Model {
				m.pickerCursor = i
			}
		}
		return m, nil

	case contextRefreshedMsg:
		m.contextLimit = msg.limit
		m.visionCapable = msg.visionCapable
		return m, nil

	case mcpConnectedMsg:
		m.mcpMgr = msg.mgr
		m.mcpTools = msg.tools
		for _, w := range msg.warnings {
			m.pushBlock(block{kind: blockSystem, text: "mcp: " + w})
		}
		if len(msg.tools) > 0 {
			m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("connected %d MCP tool(s)", len(msg.tools))})
			m.refreshViewport()
		}
		return m, nil

	case subagentDoneMsg:
		tc := backend.ToolCall{ID: msg.callID, Function: backend.FunctionCall{Name: "spawn_subagent"}}
		if msg.err != nil {
			return m.recordToolResult(tc, "error: subagent failed: "+msg.err.Error(), true)
		}
		var trace string
		if len(msg.result.Trace) > 0 {
			trace = fmt.Sprintf(" (used: %s)", strings.Join(msg.result.Trace, ", "))
		}
		return m.recordToolResult(tc, fmt.Sprintf("subagent report%s:\n\n%s", trace, msg.result.Summary), false)

	case semanticSearchDoneMsg:
		if errors.Is(msg.err, context.Canceled) {
			return m.handleToolExecuted(toolExecutedMsg{callID: msg.callID, name: "semantic_search", err: msg.err})
		}
		tc := backend.ToolCall{ID: msg.callID, Function: backend.FunctionCall{Name: "semantic_search"}}
		if msg.err != nil {
			return m.recordToolResult(tc, "error: semantic_search failed: "+msg.err.Error(), true)
		}
		return m.recordToolResult(tc, renderSemanticMatches(msg.matches, msg.reembedded), false)

	case clipboardCopiedMsg:
		if msg.err != nil {
			m.pushBlock(block{kind: blockError, text: "clipboard copy failed: " + msg.err.Error()})
		} else if msg.chars > 0 {
			m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("copied transcript to clipboard (%d chars)", msg.chars)})
		}
		return m, nil
	}

	return m, nil
}

const pickerReservedHeight = 10
const confirmReservedHeight = 16

func (m *Model) resizeViewport() {
	if !m.ready {
		return
	}
	headerH := 3
	footerH := 1
	bottomH := 2
	switch {
	case m.picker != pickerNone:
		bottomH = pickerReservedHeight
	case m.state == stateConfirming:
		bottomH = confirmReservedHeight
	}
	planH := 0
	if n := len(m.planSteps); n > 0 {
		shown := n
		if shown > 8 {
			shown = 8
		}
		planH = shown + 3
	}
	vh := m.height - headerH - bottomH - footerH - planH
	if vh < 3 {
		vh = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vh
}

func (m *Model) pushBlock(b block) {
	m.blocks = append(m.blocks, b)
	m.refreshViewport()
}

func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	wasAtBottom := m.viewport.AtBottom()

	var parts []string
	for _, b := range m.blocks {
		if b.kind == blockReasoning && !m.showThinking {
			continue
		}
		r := renderBlock(b)
		if r != "" {
			parts = append(parts, r)
		}
	}
	content := strings.Join(parts, "\n\n")
	if m.viewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.viewport.Width).Render(content)
	}
	m.viewport.SetContent(content)

	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlD:
		m.quitting = true
		if m.cancelGen != nil {
			m.cancelGen()
		}
		return m, tea.Quit

	case tea.KeyCtrlC:
		if m.state == stateGenerating {
			if m.cancelGen != nil {
				m.cancelGen()
			}
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlT:
		m.showThinking = !m.showThinking
		m.refreshViewport()
		return m, nil

	case tea.KeyCtrlY:
		return m, copyToClipboardCmd(m.transcriptPlainText())
	}

	if m.picker != pickerNone {
		return m.handlePickerKey(msg)
	}

	switch msg.Type {
	case tea.KeyPgUp, tea.KeyPgDown, tea.KeyUp, tea.KeyDown:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	if m.state == stateConfirming {
		return m.handleConfirmKey(msg)
	}

	if m.state == stateGenerating {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}
		m.input.SetValue("")
		m.restyleInput()
		if strings.HasPrefix(text, "/") {
			return m.handleCommand(text)
		}
		return m.submitUserMessage(text)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.restyleInput()
	return m, cmd
}

func (m *Model) restyleInput() {
	if strings.HasPrefix(m.input.Value(), "/") {
		m.input.PromptStyle = commandPromptStyle
		m.input.TextStyle = commandTextStyle
	} else {
		m.input.PromptStyle = inputPromptStyle
		m.input.TextStyle = lipgloss.NewStyle()
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingConfirm == nil {
		m.state = stateIdle
		m.resizeViewport()
		return m, nil
	}

	switch strings.ToLower(msg.String()) {
	case "y":
		tc := *m.pendingConfirm
		m.pendingConfirm = nil
		m.state = stateGenerating
		m.activity = "running tool: " + tc.Function.Name
		m.resizeViewport()
		return m.startTool(tc)
	case "n", "esc", "ctrl+c":
		tc := *m.pendingConfirm
		m.pendingConfirm = nil
		m.resizeViewport()
		return m.recordToolResult(tc, "DENIED: user declined confirmation", true)
	}
	return m, nil
}

func (m Model) handleCommand(text string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(text)
	switch strings.ToLower(fields[0]) {
	case "/thinking":
		switch {
		case len(fields) > 1 && strings.EqualFold(fields[1], "on"):
			m.showThinking = true
		case len(fields) > 1 && strings.EqualFold(fields[1], "off"):
			m.showThinking = false
		default:
			m.showThinking = !m.showThinking
		}
		state := "hidden"
		if m.showThinking {
			state = "shown"
		}
		m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("thinking is now %s (ctrl+t to toggle anytime)", state)})
		m.refreshViewport()

	case "/settings":
		m.openSettingsPicker()

	case "/model":
		return m.openModelPicker(pickerNone)

	case "/effort":
		if len(fields) > 1 {
			if lvl, ok := parseEffortArg(fields[1]); ok {
				m.think = lvl
				m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("reasoning effort set to %s", effortLabel(lvl))})
				return m, nil
			}
			m.pushBlock(block{kind: blockSystem, text: "unknown effort level: " + fields[1] + " (try default, off, low, medium, or high)"})
			return m, nil
		}
		m.openEffortPicker(pickerNone)

	case "/usage":
		m.pushBlock(block{kind: blockSystem, text: m.renderUsageReport()})

	case "/sessions":
		m.saveSession()
		sessions, err := session.List(m.cwd)
		if err != nil {
			m.pushBlock(block{kind: blockError, text: "couldn't list sessions: " + err.Error()})
			return m, nil
		}
		if len(sessions) == 0 {
			m.pushBlock(block{kind: blockSystem, text: "no saved sessions for this directory yet"})
			return m, nil
		}
		return m.openResumePicker(sessions), nil

	case "/web":
		if m.webAddr != "" {
			m.pushBlock(block{kind: blockSystem, text: "web interface already running at http://" + m.webAddr})
			return m, nil
		}
		port := 7878
		if len(fields) > 1 {
			if p, err := strconv.Atoi(fields[1]); err == nil {
				port = p
			}
		}
		addr, err := webserver.Start(fmt.Sprintf("127.0.0.1:%d", port), m.cfg)
		if err != nil {
			m.pushBlock(block{kind: blockError, text: "couldn't start web interface: " + err.Error()})
			return m, nil
		}
		m.webAddr = addr
		m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf(
			"web interface running at http://%s — open it in your browser. "+
				"It's a separate session sharing this directory's saved conversation, not live-synced with this terminal.", addr)})

	case "/research":
		if len(fields) < 2 {
			m.pushBlock(block{kind: blockSystem, text: "usage: /research <question> [--depth=lite|normal|deep] (default: normal)"})
			return m, nil
		}
		if m.state != stateIdle {
			m.pushBlock(block{kind: blockSystem, text: "can't start research while a generation is already in progress"})
			return m, nil
		}
		question, depth := parseResearchCommand(text)
		if question == "" {
			m.pushBlock(block{kind: blockSystem, text: "usage: /research <question> [--depth=lite|normal|deep] (default: normal)"})
			return m, nil
		}
		prompt, filename := researchPrompt(question, depth)
		depthName := []string{"lite", "normal", "deep"}[depth]
		m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("starting %s research — will save to %s", depthName, filename)})
		display := fmt.Sprintf("/research %s (depth=%s)", question, depthName)
		return m.submitUserMessageDisplay(display, prompt)

	case "/remember":
		note := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
		if note == "" {
			m.pushBlock(block{kind: blockSystem, text: "usage: /remember <note to keep across sessions>"})
			return m, nil
		}
		if err := memory.Append(note); err != nil {
			m.pushBlock(block{kind: blockSystem, text: "couldn't save note: " + err.Error()})
			return m, nil
		}
		m.pushBlock(block{kind: blockSystem, text: "remembered: " + note})

	default:
		m.pushBlock(block{kind: blockSystem, text: "unknown command: " + fields[0] + " (try /settings, /model, /effort, /usage, /remember, /research, /web, /sessions, or /thinking)"})
	}
	return m, nil
}

func (m Model) submitUserMessage(text string) (tea.Model, tea.Cmd) {
	return m.submitUserMessageDisplay(text, text)
}

// submitUserMessageDisplay is submitUserMessage but with a separate, shorter
// string shown in the transcript's "you" block — used by /research, whose
// actual message to the model is a long crafted instruction the user never
// typed and shouldn't have to read back.
func (m Model) submitUserMessageDisplay(display, actual string) (tea.Model, tea.Cmd) {
	m.maybeCompact()
	m.pushBlock(block{kind: blockUser, text: display})
	m.viewport.GotoBottom()
	m.messages = append(m.messages, backend.Message{Role: "user", Content: actual})
	m.toolIterations = 0
	m.planNudges = 0
	m.toolCallParseRetries = 0
	m.promptStart = time.Now()
	return m.startGeneration()
}

func (m Model) incompletePlanSteps() []string {
	var out []string
	for i, s := range m.planSteps {
		if s.status != stepDone && s.status != stepSkipped {
			out = append(out, fmt.Sprintf("%d: %s", i, s.text))
		}
	}
	return out
}

func (m Model) startGeneration() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelGen = cancel
	m.state = stateGenerating
	m.activity = "thinking"
	m.turnStart = time.Now()
	m.streamingContent = ""
	m.streamingReason = ""

	req := backend.ChatRequest{
		Model:    m.cfg.Model,
		Messages: m.messages,
		Tools:    toolsForModel(m.cfg.Model, m.visionCapable, m.mcpTools),
	}

	var ch <-chan backend.StreamEvent
	if m.cfg.Backend == "ollama" {
		ch = m.client.StreamChatNative(ctx, req, m.think)
	} else {
		ch = m.client.StreamChatCompletion(ctx, req)
	}
	m.streamEvents = ch

	return m, waitForStream(ch)
}

func (m Model) handleStreamEvent(ev backend.StreamEvent) (tea.Model, tea.Cmd) {
	switch ev.Kind {
	case backend.EventContent:
		m.streamingContent += ev.ContentDelta
		m.activity = "generating"
		m.setStreamingAssistantBlock()
		return m, waitForStream(m.streamEvents)

	case backend.EventReasoning:
		m.streamingReason += ev.ContentDelta
		m.activity = "thinking"
		m.setStreamingReasoningBlock()
		return m, waitForStream(m.streamEvents)

	case backend.EventToolCallStart:
		m.activity = "using tool: " + ev.ToolCallName
		return m, waitForStream(m.streamEvents)

	case backend.EventError:
		// Ollama failed to parse the model's own tool-call tags against its
		// chat template (observed live with qwen3.5:4b) — the model did
		// produce a response, only the server-side extraction of it
		// failed, so a fresh attempt has a real chance of succeeding. See
		// backend.IsMalformedToolCallResponse and the mirrored retry in
		// internal/webengine/loop.go.
		if backend.IsMalformedToolCallResponse(ev.Err) && m.toolCallParseRetries < maxToolCallParseRetries {
			m.toolCallParseRetries++
			return m.startGeneration()
		}
		m.toolCallParseRetries = 0
		m.state = stateIdle
		m.activity = ""
		if !m.promptStart.IsZero() {
			m.lastTurnDuration = time.Since(m.promptStart)
		}
		m.saveSession()
		if errors.Is(ev.Err, context.Canceled) {
			m.pushBlock(block{kind: blockSystem, text: "( interrupted )"})
		} else {
			m.pushBlock(block{kind: blockError, text: ev.Err.Error()})
		}
		return m, nil

	case backend.EventDone:
		return m.finishTurn(ev)
	}

	return m, waitForStream(m.streamEvents)
}

func (m *Model) setStreamingAssistantBlock() {
	text := m.streamingContent
	if n := len(m.blocks); n > 0 && m.blocks[n-1].kind == blockAssistant {
		m.blocks[n-1].text = text
	} else {
		m.blocks = append(m.blocks, block{kind: blockAssistant, text: text})
	}
	m.refreshViewport()
}

func (m *Model) setStreamingReasoningBlock() {
	text := m.streamingReason
	n := len(m.blocks)
	if n > 0 && m.blocks[n-1].kind == blockReasoning {
		m.blocks[n-1].text = text
	} else {
		m.blocks = append(m.blocks, block{kind: blockReasoning, text: text})
	}
	m.refreshViewport()
}

func (m Model) finishTurn(ev backend.StreamEvent) (tea.Model, tea.Cmd) {
	m.toolCallParseRetries = 0
	if ev.Usage.CompletionTokens > 0 {
		switch {
		case ev.Usage.EvalDurationNs > 0:
			// Pure generation speed (Ollama's eval_duration), excludes
			// prompt processing — more honest than wall-clock, which
			// includes the time spent re-processing our ~2K-token tool
			// schema on every request and understates true tok/s,
			// especially on short replies (found via live model testing).
			m.tokensPerSec = float64(ev.Usage.CompletionTokens) / (float64(ev.Usage.EvalDurationNs) / 1e9)
		case time.Since(m.turnStart).Seconds() > 0:
			m.tokensPerSec = float64(ev.Usage.CompletionTokens) / time.Since(m.turnStart).Seconds()
		}
	}
	m.lastUsage = ev.Usage
	recordUsage(m.cfg.Model, ev.Usage, time.Since(m.turnStart))

	msg := ev.Message

	if len(msg.ToolCalls) == 0 {
		if name, argsJSON, ok := agent.ExtractPseudoToolCall(msg.Content); ok {
			msg.ToolCalls = []backend.ToolCall{{
				ID:       fmt.Sprintf("pseudo-%d", m.toolIterations),
				Type:     "function",
				Function: backend.FunctionCall{Name: name, Arguments: argsJSON},
			}}
			msg.Content = ""
		}
	}

	m.messages = append(m.messages, msg)

	if len(msg.ToolCalls) == 0 {
		if incomplete := m.incompletePlanSteps(); len(incomplete) > 0 && m.planNudges < maxPlanNudges {
			m.planNudges++
			m.pushBlock(block{kind: blockSystem, text: "( plan has unfinished steps — asking the model to confirm or continue )"})
			m.messages = append(m.messages, backend.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"Your plan still shows these steps as not done: %s. If you actually finished them, call plan_update for each one now. If not, finish them before answering.",
					strings.Join(incomplete, "; "),
				),
			})
			return m.startGeneration()
		}
		m.state = stateIdle
		m.activity = ""
		if !m.promptStart.IsZero() {
			m.lastTurnDuration = time.Since(m.promptStart)
		}
		m.saveSession()
		return m, nil
	}

	m.toolIterations++
	if m.toolIterations > maxToolIterationsPerTurn {
		m.pushBlock(block{kind: blockError, text: fmt.Sprintf("stopped after %d tool iterations without a final answer", maxToolIterationsPerTurn)})
		m.state = stateIdle
		m.activity = ""
		if !m.promptStart.IsZero() {
			m.lastTurnDuration = time.Since(m.promptStart)
		}
		m.saveSession()
		return m, nil
	}

	m.pendingToolCalls = msg.ToolCalls
	return m.processNextToolCall()
}

func (m Model) processNextToolCall() (tea.Model, tea.Cmd) {
	if len(m.pendingToolCalls) == 0 {
		return m.startGeneration()
	}

	tc := m.pendingToolCalls[0]
	m.pendingToolCalls = m.pendingToolCalls[1:]

	m.pushBlock(block{kind: blockToolCall, toolName: tc.Function.Name, toolArgs: tc.Function.Arguments})

	switch tc.Function.Name {
	case "plan_set":
		return m.handlePlanSet(tc)
	case "plan_update":
		return m.handlePlanUpdate(tc)
	case "ask_user":
		return m.handleAskUser(tc)
	case "remember":
		return m.handleRemember(tc)
	case "load_skill":
		return m.handleLoadSkill(tc)
	case "view_image":
		return m.handleViewImage(tc)
	case "preview_page":
		return m.handlePreviewPage(tc)
	case "spawn_subagent":
		return m.handleSpawnSubagent(tc)
	case "semantic_search":
		return m.handleSemanticSearch(tc)
	}

	// Loop breaker: a model that gets an error back sometimes just retries
	// the exact same call verbatim instead of adapting — observed live
	// looping for 7+ minutes on an identical malformed write call. After
	// two identical failures in a row, refuse the third outright rather
	// than executing (and likely failing) it again.
	callKey := tc.Function.Name + "\x00" + tc.Function.Arguments
	if callKey == m.lastFailedCall && m.repeatFailCount >= 2 {
		return m.recordToolResult(tc, fmt.Sprintf(
			"DENIED: this exact %s call with identical arguments has already failed %d times in a row — refusing to run it again verbatim, since it will just fail the same way. Change the arguments, try a different tool, or explain the problem to the user instead of repeating this call.",
			tc.Function.Name, m.repeatFailCount), true)
	}

	if hookRes := hooks.RunPre(m.hooksCfg, tc.Function.Name, tc.Function.Arguments); hookRes.Denied {
		return m.recordToolResult(tc, "DENIED by pre_tool_use hook: "+hookRes.Reason, true)
	}

	decision := permission.Check(m.cfg.Mode, tc.Function.Name, tc.Function.Arguments)

	if tc.Function.Name == "write" && !decision.Denied {
		path := extractPath(tc.Function.Arguments)
		if path != "" && !m.readPaths[path] {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				decision.NeedsConfirm = true
				m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf(
					"warning: %s exists and hasn't been read in this session — forcing confirmation even in auto mode, since overwriting it would discard content the model hasn't seen", path)})
			}
		}
	}

	if decision.Denied {
		return m.recordToolResult(tc, "DENIED: "+decision.DenyReason, true)
	}
	if decision.NeedsConfirm {
		m.state = stateConfirming
		m.pendingConfirm = &tc
		m.activity = "awaiting confirmation"
		m.resizeViewport()
		return m, nil
	}

	m.activity = "running tool: " + tc.Function.Name
	return m.startTool(tc)
}

func extractPath(argsJSON string) string {
	var a struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return a.Path
}

func (m Model) startTool(tc backend.ToolCall) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	m.cancelGen = cancel
	cmd := execToolCmd(ctx, tc, m.mcpMgr)
	return m, func() tea.Msg { defer cancel(); return cmd() }
}
func execToolCmd(parent context.Context, tc backend.ToolCall, mcpMgr *mcpclient.Manager) tea.Cmd {
	return func() tea.Msg {
		var out string
		var err error
		if mcpclient.IsMCPTool(tc.Function.Name) {
			ctx, cancel := context.WithTimeout(parent, 60*time.Second)
			defer cancel()
			out, err = mcpMgr.Dispatch(ctx, tc.Function.Name, tc.Function.Arguments)
		} else {
			out, err = tools.DispatchContext(parent, tc.Function.Name, tc.Function.Arguments)
		}
		if parent.Err() != nil {
			err = parent.Err()
		}
		return toolExecutedMsg{callID: tc.ID, name: tc.Function.Name, args: tc.Function.Arguments, path: extractPath(tc.Function.Arguments), output: out, err: err}
	}
}

func (m Model) handleToolExecuted(ev toolExecutedMsg) (tea.Model, tea.Cmd) {
	m.actionsSincePlanUpdate++
	tc := backend.ToolCall{ID: ev.callID, Function: backend.FunctionCall{Name: ev.name, Arguments: ev.args}}
	if errors.Is(ev.err, context.Canceled) {
		calls := append([]backend.ToolCall{tc}, m.pendingToolCalls...)
		for _, call := range calls {
			m.messages = append(m.messages, backend.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: "interrupted by user"})
		}
		m.pendingToolCalls = nil
		m.state = stateIdle
		m.activity = ""
		m.pushBlock(block{kind: blockSystem, text: "( interrupted )"})
		m.saveSession()
		return m, nil
	}
	if ev.err != nil {
		content := "error: " + ev.err.Error()
		if ev.output != "" {
			content = fmt.Sprintf("error: %v\noutput:\n%s", ev.err, ev.output)
		}
		return m.recordToolResult(tc, content, true)
	}
	if (ev.name == "read" || ev.name == "write") && ev.path != "" {
		m.readPaths[ev.path] = true
	}
	for _, warning := range hooks.RunPost(m.hooksCfg, ev.name, ev.args, ev.output) {
		m.pushBlock(block{kind: blockSystem, text: warning})
	}
	return m.recordToolResult(tc, ev.output, false)
}

func (m Model) recordToolResult(tc backend.ToolCall, content string, isError bool) (tea.Model, tea.Cmd) {
	callKey := tc.Function.Name + "\x00" + tc.Function.Arguments
	switch {
	case !isError:
		m.lastFailedCall = ""
		m.repeatFailCount = 0
	case callKey == m.lastFailedCall:
		m.repeatFailCount++
	default:
		m.lastFailedCall = callKey
		m.repeatFailCount = 1
	}

	m.messages = append(m.messages, backend.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Function.Name,
		Content:    content,
	})
	m.pushBlock(block{kind: blockToolResult, text: content, toolIsError: isError})
	m.state = stateGenerating
	return m.processNextToolCall()
}

// recordToolResultImage is recordToolResult but attaches image data to
// the tool-result message (view_image) instead of plain text — same
// bookkeeping (repeat-failure tracking, display block, resuming the
// loop), just a different message shape.
func (m Model) recordToolResultImage(tc backend.ToolCall, content string, images []string) (tea.Model, tea.Cmd) {
	m.lastFailedCall = ""
	m.repeatFailCount = 0

	m.messages = append(m.messages, backend.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Function.Name,
		Content:    content,
		Images:     images,
	})
	m.pushBlock(block{kind: blockToolResult, text: content})
	m.state = stateGenerating
	return m.processNextToolCall()
}
