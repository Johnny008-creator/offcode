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
	if m.picker != pickerNone {
		bottom = m.pickerView()
		footer = footerStyle.Render(" ↑↓ navigate · enter select · esc back")
	} else {
		bottom = lipgloss.JoinVertical(lipgloss.Left, m.statusView(), m.inputView())
		footer = footerStyle.Render(" ctrl+d exit · ctrl+c stop/exit · enter send · ↑↓/pgup/pgdn scroll · ctrl+t thinking · ctrl+y copy · /settings")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, bottom, footer)
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
	}

	var lines []string
	lines = append(lines, headerStyle.Render(title))

	switch {
	case m.picker == pickerModel && m.pickerLoading:
		lines = append(lines, m.spinner.View()+" loading models…")
	case len(m.pickerItems) == 0:
		lines = append(lines, systemStyle.Render("(nothing to show)"))
	default:
		for i, item := range m.pickerItems {
			if i == m.pickerCursor {
				lines = append(lines, assistantLabelStyle.Render("> "+item))
			} else {
				lines = append(lines, subHeaderStyle.Render("  "+item))
			}
		}
	}

	box := strings.Join(lines, "\n")
	width := m.width - 4
	if width < 10 {
		width = 10
	}
	return confirmBoxStyle.Width(width).Render(box)
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
	sub := subHeaderStyle.Render(fmt.Sprintf(" model=%s  backend=%s  mode=%s", m.cfg.Model, m.cfg.Backend, m.cfg.Mode))
	line1 := title + sub
	line2 := strings.Repeat("─", max(0, m.width))
	return line1 + "\n" + line2
}

func (m Model) statusView() string {
	var left string
	switch {
	case m.state == stateConfirming && m.pendingConfirm != nil:
		left = confirmBoxStyle.Render(fmt.Sprintf("allow %s(%s)? [y/n]", m.pendingConfirm.Function.Name, truncate(m.pendingConfirm.Function.Arguments, 80)))
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
	return inputPromptStyle.Render("") + m.input.View()
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
