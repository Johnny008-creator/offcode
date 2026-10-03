package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent   = lipgloss.Color("12") // bright blue
	colorAssist   = lipgloss.Color("10") // bright green
	colorThinking = lipgloss.Color("8")  // gray
	colorTool     = lipgloss.Color("11") // bright yellow
	colorResult   = lipgloss.Color("14") // bright cyan
	colorError    = lipgloss.Color("9")  // bright red
	colorDim      = lipgloss.Color("8")
	colorBorder   = lipgloss.Color("8")
	colorCommand  = lipgloss.Color("13") // bright magenta

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	subHeaderStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	assistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAssist)

	thinkingStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(colorThinking)

	toolLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTool)

	toolArgsStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	resultLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorResult)

	resultBodyStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError)

	systemStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorBorder)

	inputPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	commandPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorCommand)

	commandTextStyle = lipgloss.NewStyle().
				Foreground(colorCommand)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	splashSubStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	splashIconStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	confirmBoxStyle = lipgloss.NewStyle().
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorTool)
)
