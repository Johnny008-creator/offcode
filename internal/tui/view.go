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
	status := m.statusView()
	input := m.inputView()
	footer := footerStyle.Render(" ctrl+d exit · ctrl+c stop/exit · enter send · ↑↓/pgup/pgdn scroll")

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, input, footer)
}

func (m Model) splashView() string {
	title := splashTitleStyle.Render("offcode")
	sub := splashSubStyle.Render(fmt.Sprintf("connecting to %s (%s)…", m.cfg.BaseURL, m.cfg.Model))
	content := lipgloss.JoinVertical(lipgloss.Center, title, "", sub)

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
	elapsed := time.Since(m.sessionStart).Round(time.Second)

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

	rightParts := []string{elapsed.String(), ctxStr}
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
