package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Johnny008-creator/offcode/internal/sysmon"
	"github.com/Johnny008-creator/offcode/internal/tiers"
)

var (
	colorAccent = lipgloss.Color("12")
	colorGood   = lipgloss.Color("10")
	colorWarn   = lipgloss.Color("11")
	colorHot    = lipgloss.Color("9")
	colorDim    = lipgloss.Color("8")

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	labelStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	boxStyle    = lipgloss.NewStyle().Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorDim)
)

var sparkChars = []rune("▁▂▃▄▅▆▇█")

func sparkline(history []float64, width int) string {
	if len(history) == 0 {
		return strings.Repeat(" ", width)
	}
	start := 0
	if len(history) > width {
		start = len(history) - width
	}
	vis := history[start:]

	var b strings.Builder
	for _, v := range vis {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		idx := int(v / 100 * float64(len(sparkChars)-1))
		b.WriteRune(sparkChars[idx])
	}
	for b.Len() < width {
		b.WriteRune(' ')
	}
	return b.String()
}

func percentColor(p float64) lipgloss.Color {
	switch {
	case p >= 90:
		return colorHot
	case p >= 70:
		return colorWarn
	default:
		return colorGood
	}
}

func bar(percent float64, width int) string {
	if width < 1 {
		width = 1
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	style := lipgloss.NewStyle().Foreground(percentColor(percent))
	return style.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", width-filled))
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "starting…"
	}

	var out strings.Builder
	out.WriteString(headerStyle.Render("offcode monitor"))
	out.WriteString(dimStyle.Render("  " + m.ollamaRoot))
	out.WriteString("\n")
	out.WriteString(strings.Repeat("─", m.width))
	out.WriteString("\n\n")

	barWidth := m.width - 45
	if barWidth < 10 {
		barWidth = 10
	}
	sparkWidth := m.width - 14
	if sparkWidth < 10 {
		sparkWidth = 10
	}

	out.WriteString(m.renderCPU(barWidth, sparkWidth))
	out.WriteString("\n\n")
	out.WriteString(m.renderMemory(barWidth))
	out.WriteString("\n\n")
	out.WriteString(m.renderGPU(barWidth, sparkWidth))
	out.WriteString("\n\n")
	out.WriteString(m.renderModels())
	out.WriteString("\n\n")
	out.WriteString(dimStyle.Render(fmt.Sprintf("updated %s ago  ·  q / esc / ctrl+c to quit", timeSince(m.lastUpdate))))

	return out.String()
}

func timeSince(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return time.Since(t).Round(time.Second).String()
}

func (m Model) renderCPU(barWidth, sparkWidth int) string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("CPU"))
	b.WriteString(fmt.Sprintf("  %5.1f%%  %s\n", m.cpuPercent, bar(m.cpuPercent, barWidth)))
	b.WriteString(dimStyle.Render("history  ") + sparkline(m.cpuHistory, sparkWidth))
	return b.String()
}

func (m Model) renderMemory(barWidth int) string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Memory"))
	if !m.memOK {
		b.WriteString(dimStyle.Render("  n/a"))
		return b.String()
	}
	pct := float64(m.mem.UsedBytes) / float64(m.mem.TotalBytes) * 100
	b.WriteString(fmt.Sprintf("  %5.1f%%  %s  %s / %s",
		pct, bar(pct, barWidth), sysmon.FormatBytes(m.mem.UsedBytes), sysmon.FormatBytes(m.mem.TotalBytes)))
	return b.String()
}

func (m Model) renderGPU(barWidth, sparkWidth int) string {
	var b strings.Builder
	label := "GPU"
	if m.gpuOK && m.gpu.Name != "" {
		label = "GPU — " + m.gpu.Name
	}
	b.WriteString(labelStyle.Render(label))
	if !m.gpuOK {
		b.WriteString(dimStyle.Render("  n/a (no supported GPU sysfs interface found)"))
		return b.String()
	}
	b.WriteString(fmt.Sprintf("  %5.1f%%  %s\n", m.gpu.BusyPercent, bar(m.gpu.BusyPercent, barWidth)))
	b.WriteString(dimStyle.Render("history  ") + sparkline(m.gpuHistory, sparkWidth) + "\n")

	vramPct := 0.0
	if m.gpu.VRAMTotal > 0 {
		vramPct = float64(m.gpu.VRAMUsed) / float64(m.gpu.VRAMTotal) * 100
	}
	b.WriteString(fmt.Sprintf("VRAM     %5.1f%%  %s  %s / %s\n",
		vramPct, bar(vramPct, barWidth), sysmon.FormatBytes(m.gpu.VRAMUsed), sysmon.FormatBytes(m.gpu.VRAMTotal)))
	b.WriteString(dimStyle.Render(fmt.Sprintf("%.0f°C  ·  %.1f W", m.gpu.TempC, m.gpu.PowerW)))
	return b.String()
}

func (m Model) renderModels() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Loaded models (Ollama)"))
	b.WriteString("\n")
	if m.runningErr != "" {
		b.WriteString(dimStyle.Render("  could not reach Ollama: " + m.runningErr))
		return b.String()
	}
	if len(m.running) == 0 {
		b.WriteString(dimStyle.Render("  (none loaded — a model loads on first request and unloads after its idle timeout)"))
		return b.String()
	}
	for _, r := range m.running {
		name := tiers.HeaderLabel(r.Name)
		expires := "—"
		if r.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, r.ExpiresAt); err == nil {
				d := time.Until(t)
				if d > 0 {
					expires = d.Round(time.Second).String()
				} else {
					expires = "expiring"
				}
			}
		}
		fmt.Fprintf(&b, "  %-28s  vram %-10s  ctx %-6d  unloads in %s\n",
			name, sysmon.FormatBytes(r.SizeVRAMBytes), r.ContextLength, expires)
	}
	return strings.TrimRight(b.String(), "\n")
}
