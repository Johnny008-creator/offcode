package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.phase == phaseSplash {
		return m.splashView()
	}

	if !m.ready {
		return "starting…"
	}

	header := m.headerView()
	body := m.viewport.View()

	var bottom, footer string
	switch {
	case m.picker != pickerNone:
		bottom = m.pickerView()
		footer = footerStyle.Render(" ↑↓ navigate · enter select · esc back")
	case m.state == stateConfirming && m.pendingConfirm != nil:
		bottom = m.confirmView()
		footer = footerStyle.Render(" y allow · n/esc deny")
	default:
		bottom = lipgloss.JoinVertical(lipgloss.Left, m.statusView(), m.inputView())
		footer = footerStyle.Render(" ctrl+d exit · ctrl+c stop/exit · enter send · ↑↓/pgup/pgdn scroll · ctrl+t thinking · ctrl+y copy · /settings")
	}

	parts := []string{header}
	if plan := m.planView(); plan != "" {
		parts = append(parts, plan)
	}
	parts = append(parts, body, bottom, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) planView() string {
	if len(m.planSteps) == 0 {
		return ""
	}
	steps := m.planSteps
	more := 0
	if len(steps) > 8 {
		more = len(steps) - 8
		steps = steps[:8]
	}

	var lines []string
	lines = append(lines, strings.Repeat("─", max(0, m.width)))
	lines = append(lines, headerStyle.Render("Plan"))
	for _, s := range steps {
		icon := "○"
		style := subHeaderStyle
		switch s.status {
		case stepInProgress:
			icon = "●"
			style = assistantLabelStyle
		case stepDone:
			icon = "✓"
			style = resultLabelStyle
		case stepSkipped:
			icon = "✗"
			style = systemStyle
		}
		lines = append(lines, style.Render(icon+" "+s.text))
	}
	if more > 0 {
		lines = append(lines, systemStyle.Render(fmt.Sprintf("… (%d more)", more)))
	}

	return strings.Join(lines, "\n")
}

func (m Model) pickerView() string {
	title := "Settings"
	switch m.picker {
	case pickerEffort:
		title = "Reasoning effort"
	case pickerMode:
		title = "Permission mode"
	case pickerModel:
		title = "Select model"
	case pickerAskUser:
		title = "offcode is asking"
	case pickerResume:
		title = "Sessions"
	}

	width := m.width - 4
	if width < 10 {
		width = 10
	}

	var lines []string
	lines = append(lines, headerStyle.Render(title))
	if m.picker == pickerAskUser && m.pickerQuestion != "" {
		lines = append(lines, lipgloss.NewStyle().Width(width).Render(m.pickerQuestion))
		lines = append(lines, "")
	}

	switch {
	case m.picker == pickerModel && m.pickerLoading:
		lines = append(lines, m.spinner.View()+" loading models…")
	case len(m.pickerItems) == 0:
		lines = append(lines, systemStyle.Render("(nothing to show)"))
	default:
		for i, item := range m.pickerItems {
			label := item
			switch m.picker {
			case pickerModel:
				label = modelLabel(item)
			case pickerEffort:
				if desc, ok := effortDescriptions[item]; ok {
					label = item + " — " + desc
				}
			}
			if i == m.pickerCursor {
				lines = append(lines, assistantLabelStyle.Render("> "+label))
			} else {
				lines = append(lines, subHeaderStyle.Render("  "+label))
			}
		}
	}

	box := strings.Join(lines, "\n")
	return confirmBoxStyle.Width(width).Render(box)
}

// padLine right-pads s with spaces to width visible columns (ANSI-aware
// via lipgloss.Width), so a box of these lines can be rendered without an
// explicit Width() constraint — see confirmView/pickerView: a Width()-
// constrained Render() on pre-joined multi-line ANSI content reflows it
// instead of just drawing a border (hit this in the monitor's charts and
// the chat plan panel; documented in docs/DECISIONS.md).
func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func (m Model) confirmView() string {
	width := m.width - 4
	if width < 10 {
		width = 10
	}

	header, diffLines := confirmPreview(*m.pendingConfirm)

	var lines []string
	lines = append(lines, padLine(headerStyle.Render(truncate(header, width)), width))
	for _, dl := range diffLines {
		var text string
		switch dl.kind {
		case diffRemoved:
			text = errorStyle.Render(truncate("- "+dl.text, width))
		case diffAdded:
			text = resultLabelStyle.Render(truncate("+ "+dl.text, width))
		default:
			text = subHeaderStyle.Render(truncate(dl.text, width))
		}
		lines = append(lines, padLine(text, width))
	}
	lines = append(lines, padLine("", width))
	lines = append(lines, padLine(assistantLabelStyle.Render(fmt.Sprintf("allow %s? [y/n]", m.pendingConfirm.Function.Name)), width))

	return confirmBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) splashView() string {
	banner := renderBanner(m.cfg)
	sub := splashSubStyle.Render(fmt.Sprintf("connecting to %s…", m.cfg.BaseURL))
	content := lipgloss.JoinVertical(lipgloss.Left, banner, "", sub)

	if m.width == 0 {
		return content
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) headerView() string {
	title := headerStyle.Render("offcode")
	sub := subHeaderStyle.Render(fmt.Sprintf(" model=%s  backend=%s  mode=%s", modelHeaderLabel(m.cfg.Model), m.cfg.Backend, m.cfg.Mode))
	line1 := title + sub
	line2 := strings.Repeat("─", max(0, m.width))
	return line1 + "\n" + line2
}

func (m Model) statusView() string {
	var left string
	switch {
	case m.state == stateGenerating:
		left = m.spinner.View() + " " + m.activity + "…"
	default:
		left = systemStyle.Render("ready")
	}

	ctxStr := "ctx n/a"
	if m.contextLimit > 0 {
		used := m.lastUsage.TotalTokens
		ctxStr = fmt.Sprintf("ctx %s/%s", formatTokens(used), formatTokens(m.contextLimit))
	}

	var rightParts []string
	switch {
	case m.state != stateIdle && !m.promptStart.IsZero():
		rightParts = append(rightParts, time.Since(m.promptStart).Round(time.Second).String())
	case m.lastTurnDuration > 0:
		rightParts = append(rightParts, m.lastTurnDuration.Round(time.Second).String())
	}
	rightParts = append(rightParts, ctxStr)
	if m.tokensPerSec > 0 {
		rightParts = append(rightParts, fmt.Sprintf("%.1f tok/s", m.tokensPerSec))
	}
	right := strings.Join(rightParts, " │ ")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + subHeaderStyle.Render(right)
	return statusBarStyle.Width(m.width).Render(line)
}

func (m Model) inputView() string {
	if m.state == stateGenerating || m.state == stateConfirming {
		return systemStyle.Render(" waiting…")
	}
	return m.input.View()
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
