package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/session"
)

type pickerKind int

const (
	pickerNone pickerKind = iota
	pickerSettings
	pickerEffort
	pickerMode
	pickerModel
	pickerAskUser
	pickerResume
)

var effortLevels = []backend.ThinkLevel{
	backend.ThinkDefault,
	backend.ThinkOff,
	backend.ThinkLow,
	backend.ThinkMedium,
	backend.ThinkHigh,
}

func parseEffortArg(s string) (backend.ThinkLevel, bool) {
	for _, lvl := range effortLevels {
		if string(lvl) == s || (lvl == backend.ThinkDefault && s == "default") {
			return lvl, true
		}
	}
	return backend.ThinkDefault, false
}

var permissionModes = []permission.Mode{
	permission.ModePlan,
	permission.ModeDefault,
	permission.ModeAuto,
}

var effortDescriptions = map[string]string{
	"default": "model's own default reasoning behavior",
	"off":     "no reasoning — fastest, best for simple tasks",
	"low":     "light reasoning",
	"medium":  "balanced reasoning depth",
	"high":    "deepest reasoning — slowest, best for hard problems",
}

func effortLabel(t backend.ThinkLevel) string {
	if t == backend.ThinkDefault {
		return "default"
	}
	return string(t)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m Model) settingsItems() []string {
	return []string{
		"Thinking display: " + onOff(m.showThinking),
		"Reasoning effort: " + effortLabel(m.think),
		"Permission mode: " + string(m.cfg.Mode),
		"Model: " + modelLabel(m.cfg.Model),
	}
}

func (m *Model) openSettingsPicker() {
	m.picker = pickerSettings
	m.pickerReturnTo = pickerNone
	m.pickerItems = m.settingsItems()
	m.pickerCursor = 0
	m.resizeViewport()
	m.refreshViewport()
}

func (m *Model) openEffortPicker(returnTo pickerKind) {
	m.picker = pickerEffort
	m.pickerReturnTo = returnTo
	m.pickerItems = make([]string, len(effortLevels))
	for i, lvl := range effortLevels {
		m.pickerItems[i] = effortLabel(lvl)
		if lvl == m.think {
			m.pickerCursor = i
		}
	}
	m.resizeViewport()
}

func (m *Model) openModePicker(returnTo pickerKind) {
	m.picker = pickerMode
	m.pickerReturnTo = returnTo
	m.pickerItems = make([]string, len(permissionModes))
	for i, mode := range permissionModes {
		m.pickerItems[i] = string(mode)
		if mode == m.cfg.Mode {
			m.pickerCursor = i
		}
	}
	m.resizeViewport()
}

func (m Model) openModelPicker(returnTo pickerKind) (Model, tea.Cmd) {
	m.picker = pickerModel
	m.pickerReturnTo = returnTo
	m.pickerItems = nil
	m.pickerCursor = 0
	m.pickerLoading = true
	m.resizeViewport()
	return m, fetchModelsCmd(m.client, m.cfg.Backend)
}

// openResumePicker lists saved sessions for the current directory, newest
// first, with a "start a new session" entry pinned at the top. Used both
// at startup (-resume) and mid-run (/sessions).
func (m Model) openResumePicker(sessions []session.Meta) Model {
	m.picker = pickerResume
	m.pickerReturnTo = pickerNone
	m.pickerSessions = sessions
	items := make([]string, 0, len(sessions)+1)
	items = append(items, "▸ start a new session")
	for _, s := range sessions {
		items = append(items, fmt.Sprintf("%s · %d msgs · %s", s.SavedAt.Format("2006-01-02 15:04"), s.Messages, s.Summary))
	}
	m.pickerItems = items
	m.pickerCursor = 0
	m.resizeViewport()
	return m
}

func fetchModelsCmd(c *backend.Client, backendKind string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		models, err := c.ListModels(ctx, backendKind)
		return modelsLoadedMsg{models: models, err: err}
	}
}

func refreshContextCmd(c *backend.Client, model string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return contextRefreshedMsg{limit: c.ContextLength(ctx, model), visionCapable: c.SupportsVision(ctx, model)}
	}
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.picker == pickerAskUser {
			return m.cancelAskUser()
		}
		if m.picker == pickerResume {
			m.picker = pickerNone
			m.resizeViewport()
			return m.afterSessionDecided()
		}
		return m.closePicker(""), nil

	case tea.KeyUp:
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
		return m, nil

	case tea.KeyDown:
		if m.pickerCursor < len(m.pickerItems)-1 {
			m.pickerCursor++
		}
		return m, nil

	case tea.KeyEnter:
		return m.applyPickerSelection()
	}
	return m, nil
}

func (m Model) closePicker(note string) Model {
	if note != "" {
		m.pushBlock(block{kind: blockSystem, text: note})
	}
	if m.pickerReturnTo != pickerNone {
		target := m.pickerReturnTo
		m.pickerReturnTo = pickerNone
		m.picker = target
		m.pickerItems = m.settingsItems()
		m.pickerCursor = 0
	} else {
		m.picker = pickerNone
	}
	m.resizeViewport()
	m.refreshViewport()
	return m
}

func (m Model) applyPickerSelection() (tea.Model, tea.Cmd) {
	switch m.picker {
	case pickerSettings:
		switch m.pickerCursor {
		case 0:
			m.showThinking = !m.showThinking
			m.refreshViewport()
			m.pickerItems = m.settingsItems()
		case 1:
			m.openEffortPicker(pickerSettings)
		case 2:
			m.openModePicker(pickerSettings)
		case 3:
			return m.openModelPicker(pickerSettings)
		}
		return m, nil

	case pickerEffort:
		if m.pickerCursor < len(effortLevels) {
			m.think = effortLevels[m.pickerCursor]
		}
		return m.closePicker(fmt.Sprintf("reasoning effort set to %s", effortLabel(m.think))), nil

	case pickerMode:
		if m.pickerCursor < len(permissionModes) {
			m.cfg.Mode = permissionModes[m.pickerCursor]
		}
		return m.closePicker(fmt.Sprintf("permission mode set to %s", m.cfg.Mode)), nil

	case pickerModel:
		if m.pickerCursor < len(m.pickerItems) {
			newModel := m.pickerItems[m.pickerCursor]
			m.cfg.Model = newModel
			m.lastUsage = backend.Usage{}
			if len(m.messages) > 0 && m.messages[0].Role == "system" {
				m.messages[0].Content = buildSystemPrompt(newModel)
			}
			m2 := m.closePicker("model set to " + newModel)
			return m2, refreshContextCmd(m.client, newModel)
		}
		return m.closePicker(""), nil

	case pickerAskUser:
		return m.resolveAskUser()

	case pickerResume:
		return m.applyResumeSelection()
	}
	return m, nil
}

// applyResumeSelection handles the resume picker's choice: either a
// brand-new session, or a specific saved one. Either way it rebuilds the
// visible transcript from scratch (just the banner, then whatever's being
// loaded) rather than appending — this runs both at startup and mid-run
// via /sessions, and mid-run the old conversation's blocks should
// disappear from view on a switch, not pile up alongside the new one.
func (m Model) applyResumeSelection() (tea.Model, tea.Cmd) {
	m.picker = pickerNone

	if m.pickerCursor == 0 || m.pickerCursor-1 >= len(m.pickerSessions) {
		m.sessionID = session.NewID()
		m.messages = []backend.Message{{Role: "system", Content: buildSystemPrompt(m.cfg.Model)}}
		m.resumedCount = 0
		m.blocks = []block{{kind: blockBanner, text: renderBanner(m.cfg)}}
		m.resizeViewport()
		m.pushBlock(block{kind: blockSystem, text: "started a new session"})
		return m.afterSessionDecided()
	}

	chosen := m.pickerSessions[m.pickerCursor-1]
	saved, ok, err := session.Load(m.cwd, chosen.ID)
	if err != nil || !ok {
		m.blocks = []block{{kind: blockBanner, text: renderBanner(m.cfg)}}
		m.resizeViewport()
		m.pushBlock(block{kind: blockError, text: "couldn't load session " + chosen.ID})
		return m.afterSessionDecided()
	}

	m.sessionID = chosen.ID
	if m.cfg.Model == "" && saved.Model != "" {
		m.cfg.Model = saved.Model
	}
	m.messages = append([]backend.Message{{Role: "system", Content: buildSystemPrompt(m.cfg.Model)}}, saved.Messages...)
	m.resumedCount = len(saved.Messages)
	// Rendered only now, after cfg.Model may have just been restored from
	// the saved session — rendering it earlier would bake in a stale
	// "none selected" even though the header (which re-renders live) shows
	// the right model a moment later.
	m.blocks = []block{{kind: blockBanner, text: renderBanner(m.cfg)}}
	m.resizeViewport()
	m.pushBlock(block{kind: blockSystem, text: fmt.Sprintf("resumed session %s (%d messages)", chosen.ID, len(saved.Messages))})
	for _, msg := range saved.Messages {
		switch {
		case msg.Role == "user":
			m.pushBlock(block{kind: blockUser, text: msg.Content})
		case msg.Role == "assistant" && msg.Content != "":
			m.pushBlock(block{kind: blockAssistant, text: msg.Content})
		}
	}
	return m.afterSessionDecided()
}
