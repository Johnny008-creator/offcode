package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
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
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/tools"
)

const systemPromptTemplate = `You are offcode, a local coding assistant running against an on-device model.
You have tools to read, write, and edit files, run shell commands, search the
codebase, search the web, and fetch a URL's content. Use tools when you need
real information instead of guessing. Keep responses concise.

For an EXISTING file: prefer the edit tool over write. If edit fails (e.g.
old_string not found or not unique), read the file first to see its real
current content before trying again — never call write on a file you
haven't just read, since write replaces the entire file and a guessed
rewrite will silently discard content you never saw. Only use write
directly for brand-new files or when you've just read the file you're
updating.

Planning: for any task with more than one or two obvious steps, call
plan_set first with a short ordered list of concrete steps before doing
anything else — the first step is marked in_progress for you automatically.
As soon as you finish a step, call plan_update(index, "done") right away,
even if the next action feels like a natural continuation — the next
pending step gets marked in_progress for you automatically, so that's the
only call you need per finished step. Add or adjust remaining steps with
plan_set again if the task's scope changes mid-way. When you finish, give
a substantive closing summary of what you actually did or built, not just
a one-line acknowledgment — say what changed, where, and how the user can
see or use it. Skip planning entirely for genuinely trivial one-step
requests.

Asking the user: if there's a real fork in approach and you have a
recommendation but no clearly-forced answer (e.g. which of several valid
designs, which library, which UI layout), call ask_user with the options
and your recommended_index instead of silently guessing. Don't ask about
things you can reasonably decide yourself — that's just friction.

Building UI/web output (HTML/CSS/frontend code): always give it real,
modern styling — sensible typography, spacing, and color, not bare
unstyled markup. A finished page or app should look deliberately designed,
not like a first draft. If you're not confident it looks good, iterate on
the CSS before calling the task done.

Web search and fetch_url: your training data can be stale or wrong about
anything time-sensitive (current versions, prices, recent events, current
best practices). When that matters, use web_search, and if the result
snippets don't directly answer the question, use fetch_url on the most
relevant result instead of falling back to a training-data guess.

Environment you are actually running in — use these, don't guess or assume
a different OS or layout (e.g. never invent macOS-style paths like
/Users/<name> unless the operating system below is darwin):
- Operating system: %s
- Shell for the bash tool: %s
- Home directory: %s
- Current working directory: %s`

func buildSystemPrompt() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "unknown"
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}
	shell := "sh -c"
	if runtime.GOOS == "windows" {
		shell = "cmd /C"
	}
	return fmt.Sprintf(systemPromptTemplate, runtime.GOOS, shell, home, cwd)
}

const maxToolIterationsPerTurn = 25

const maxPlanNudges = 2

const appVersion = "0.1.0"

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

	pendingToolCalls []backend.ToolCall
	pendingConfirm   *backend.ToolCall
	toolIterations   int
	readPaths        map[string]bool

	streamEvents <-chan backend.StreamEvent
	cancelGen    context.CancelFunc

	quitting bool
}

func New(cfg config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "type your request..."
	ti.Prompt = "> "
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		cfg:          cfg,
		client:       backend.NewClient(cfg.BaseURL),
		messages:     []backend.Message{{Role: "system", Content: buildSystemPrompt()}},
		phase:        phaseSplash,
		state:        stateIdle,
		input:        ti,
		spinner:      sp,
		sessionStart: time.Now(),
		showThinking: true,
		readPaths:    map[string]bool{},
	}
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
	return tea.Batch(splashCmd(m.client, m.cfg), m.spinner.Tick)
}

func splashCmd(c *backend.Client, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctxLen := c.ContextLength(context.Background(), cfg.Model)
		if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
			time.Sleep(500*time.Millisecond - elapsed)
		}
		return splashDoneMsg{contextLimit: ctxLen}
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
		m.phase = phaseChat
		m.pushBlock(block{kind: blockBanner, text: renderBanner(m.cfg)})
		return m, tickCmd()

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
		return m, nil

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

func (m *Model) resizeViewport() {
	if !m.ready {
		return
	}
	headerH := 3
	footerH := 1
	bottomH := 2
	if m.picker != pickerNone {
		bottomH = pickerReservedHeight
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
		if strings.HasPrefix(text, "/") {
			return m.handleCommand(text)
		}
		return m.submitUserMessage(text)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingConfirm == nil {
		m.state = stateIdle
		return m, nil
	}

	switch strings.ToLower(msg.String()) {
	case "y":
		tc := *m.pendingConfirm
		m.pendingConfirm = nil
		m.state = stateGenerating
		m.activity = "running tool: " + tc.Function.Name
		return m, execToolCmd(tc)
	case "n", "esc", "ctrl+c":
		tc := *m.pendingConfirm
		m.pendingConfirm = nil
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
		m.openEffortPicker(pickerNone)

	default:
		m.pushBlock(block{kind: blockSystem, text: "unknown command: " + fields[0] + " (try /settings, /model, /effort, or /thinking)"})
	}
	return m, nil
}

func (m Model) submitUserMessage(text string) (tea.Model, tea.Cmd) {
	m.pushBlock(block{kind: blockUser, text: text})
	m.viewport.GotoBottom()
	m.messages = append(m.messages, backend.Message{Role: "user", Content: text})
	m.toolIterations = 0
	m.planNudges = 0
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
		Tools:    append(tools.Definitions(), metaToolDefinitions()...),
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
		m.state = stateIdle
		m.activity = ""
		if !m.promptStart.IsZero() {
			m.lastTurnDuration = time.Since(m.promptStart)
		}
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
	elapsed := time.Since(m.turnStart).Seconds()
	if elapsed > 0 && ev.Usage.CompletionTokens > 0 {
		m.tokensPerSec = float64(ev.Usage.CompletionTokens) / elapsed
	}
	m.lastUsage = ev.Usage

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
		return m, nil
	}

	m.activity = "running tool: " + tc.Function.Name
	return m, execToolCmd(tc)
}

func extractPath(argsJSON string) string {
	var a struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return a.Path
}

func execToolCmd(tc backend.ToolCall) tea.Cmd {
	return func() tea.Msg {
		out, err := tools.Dispatch(tc.Function.Name, tc.Function.Arguments)
		return toolExecutedMsg{callID: tc.ID, name: tc.Function.Name, path: extractPath(tc.Function.Arguments), output: out, err: err}
	}
}

func (m Model) handleToolExecuted(ev toolExecutedMsg) (tea.Model, tea.Cmd) {
	m.actionsSincePlanUpdate++
	tc := backend.ToolCall{ID: ev.callID, Function: backend.FunctionCall{Name: ev.name}}
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
	return m.recordToolResult(tc, ev.output, false)
}

func (m Model) recordToolResult(tc backend.ToolCall, content string, isError bool) (tea.Model, tea.Cmd) {
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
