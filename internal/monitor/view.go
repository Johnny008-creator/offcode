package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Johnny008-creator/offcode/internal/sysmon"
	"github.com/Johnny008-creator/offcode/internal/tiers"
	"github.com/Johnny008-creator/offcode/internal/usagelog"
)

var (
	colorAccent = lipgloss.Color("12")
	colorGood   = lipgloss.Color("10")
	colorWarn   = lipgloss.Color("11")
	colorHot    = lipgloss.Color("9")
	colorDim    = lipgloss.Color("8")

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	boxStyle    = lipgloss.NewStyle().Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorDim)
)

const chartRows = 6

// multiRowChart renders a bottom-up filled bar chart of history (0-100
// scale) as `rows` lines of `width` characters, most recent sample on the
// right. Each row is colored by how "hot" that band is, so the chart
// gradients from green (low) through yellow to red (near 100%) similar to
// a classic system monitor.
func multiRowChart(history []float64, width, rows int) []string {
	if width < 1 {
		width = 1
	}
	cols := make([]int, width)
	start := 0
	if len(history) > width {
		start = len(history) - width
	}
	vis := history[start:]
	pad := width - len(vis)
	if pad < 0 {
		pad = 0
	}
	for i, v := range vis {
		if i+pad >= width {
			break
		}
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		filled := int(v / 100 * float64(rows))
		if filled == 0 && v > 0 {
			filled = 1
		}
		cols[pad+i] = filled
	}

	lines := make([]string, rows)
	for r := 0; r < rows; r++ {
		threshold := rows - r
		bandPercent := float64(threshold) / float64(rows) * 100
		style := lipgloss.NewStyle().Foreground(percentColor(bandPercent))
		var b strings.Builder
		for c := 0; c < width; c++ {
			if cols[c] >= threshold {
				b.WriteRune('█')
			} else {
				b.WriteRune(' ')
			}
		}
		lines[r] = style.Render(b.String())
	}
	return lines
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

// padLine right-pads s with spaces to width visible columns, using
// lipgloss's ANSI-aware width measurement (not byte length).
func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// box renders a titled, bordered panel exactly `outerWidth` columns wide.
// Deliberately does NOT use boxStyle.Width() on multi-line ANSI-styled
// content — lipgloss's width-constrained Render() reflows pre-joined
// multi-line styled text instead of just drawing a border around it
// (hit this exact bug class before in the chat TUI's plan panel; see
// docs/DECISIONS.md). Instead, every line is padded to the same visible
// width by hand first, then the border is drawn around already-uniform
// content with no Width() call to trigger a reflow.
func box(title, body string, outerWidth int) string {
	const overhead = 4 // 2 padding + 2 border
	w := outerWidth - overhead
	if w < 1 {
		w = 1
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = padLine(line, w)
	}
	content := padLine(titleStyle.Render(title), w) + "\n" + strings.Join(lines, "\n")
	return boxStyle.Render(content)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "starting…"
	}

	innerWidth := m.width - 4 // account for box border + padding
	if innerWidth < 10 {
		innerWidth = 10
	}
	barWidth := innerWidth - 40
	if barWidth < 10 {
		barWidth = 10
	}

	var out strings.Builder
	out.WriteString(headerStyle.Render("offcode monitor"))
	out.WriteString(dimStyle.Render("  " + m.ollamaRoot))
	out.WriteString("\n\n")

	out.WriteString(box("CPU", m.renderCPU(barWidth, innerWidth), m.width))
	out.WriteString("\n")
	out.WriteString(box("Memory", m.renderMemory(barWidth), m.width))
	out.WriteString("\n")
	out.WriteString(box(m.gpuTitle(), m.renderGPU(barWidth, innerWidth), m.width))
	out.WriteString("\n")
	out.WriteString(box("Loaded models (Ollama)", m.renderModels(), m.width))
	out.WriteString("\n")
	out.WriteString(box("Usage (all-time)", m.renderUsage(), m.width))
	out.WriteString("\n")
	out.WriteString(dimStyle.Render(fmt.Sprintf("updated %s ago  ·  q / esc / ctrl+c to quit", timeSince(m.lastUpdate))))

	return out.String()
}

func timeSince(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return time.Since(t).Round(time.Second).String()
}

func (m Model) renderCPU(barWidth, chartWidth int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%5.1f%%  %s\n", m.cpuPercent, bar(m.cpuPercent, barWidth))
	for _, line := range multiRowChart(m.cpuHistory, chartWidth, chartRows) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderMemory(barWidth int) string {
	if !m.memOK {
		return dimStyle.Render("n/a")
	}
	pct := float64(m.mem.UsedBytes) / float64(m.mem.TotalBytes) * 100
	return fmt.Sprintf("%5.1f%%  %s  %s / %s",
		pct, bar(pct, barWidth), sysmon.FormatBytes(m.mem.UsedBytes), sysmon.FormatBytes(m.mem.TotalBytes))
}

func (m Model) gpuTitle() string {
	if m.gpuOK && m.gpu.Name != "" {
		return "GPU — " + m.gpu.Name
	}
	return "GPU"
}

func (m Model) renderGPU(barWidth, chartWidth int) string {
	if !m.gpuOK {
		return dimStyle.Render("n/a (no supported GPU sysfs interface found)")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "busy  %5.1f%%  %s\n", m.gpu.BusyPercent, bar(m.gpu.BusyPercent, barWidth))
	for _, line := range multiRowChart(m.gpuHistory, chartWidth, chartRows) {
		b.WriteString(line)
		b.WriteString("\n")
	}

	vramPct := 0.0
	if m.gpu.VRAMTotal > 0 {
		vramPct = float64(m.gpu.VRAMUsed) / float64(m.gpu.VRAMTotal) * 100
	}
	fmt.Fprintf(&b, "vram  %5.1f%%  %s  %s / %s\n",
		vramPct, bar(vramPct, barWidth), sysmon.FormatBytes(m.gpu.VRAMUsed), sysmon.FormatBytes(m.gpu.VRAMTotal))
	b.WriteString(dimStyle.Render(fmt.Sprintf("%.0f°C  ·  %.1f W", m.gpu.TempC, m.gpu.PowerW)))
	return b.String()
}

func (m Model) renderModels() string {
	if m.runningErr != "" {
		return dimStyle.Render("could not reach Ollama: " + m.runningErr)
	}
	if len(m.running) == 0 {
		return dimStyle.Render("(none loaded — a model loads on first request and unloads after its idle timeout)")
	}

	var b strings.Builder
	for i, r := range m.running {
		name := tiers.HeaderLabel(r.Name)
		expires := "—"
		if r.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, r.ExpiresAt); err == nil {
				if d := time.Until(t); d > 0 {
					expires = d.Round(time.Second).String()
				} else {
					expires = "expiring"
				}
			}
		}
		fmt.Fprintf(&b, "%-28s  vram %-10s  ctx %-6d  unloads in %s",
			name, sysmon.FormatBytes(r.SizeVRAMBytes), r.ContextLength, expires)
		if i < len(m.running)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) renderUsage() string {
	records, err := usagelog.ReadAll()
	if err != nil {
		return dimStyle.Render("could not read usage log: " + err.Error())
	}
	summaries := usagelog.Summarize(records, time.Time{})
	report := usagelog.RenderReport(summaries, "")
	return strings.TrimLeft(report, "\n")
}
