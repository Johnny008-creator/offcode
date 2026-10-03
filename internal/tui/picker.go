package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/permission"
)

type pickerKind int

const (
	pickerNone pickerKind = iota
	pickerSettings
	pickerEffort
	pickerMode
	pickerModel
	pickerAskUser
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
		return contextRefreshedMsg{limit: c.ContextLength(ctx, model)}
	}
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.picker == pickerAskUser {
			return m.cancelAskUser()
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
	}
	return m, nil
}
