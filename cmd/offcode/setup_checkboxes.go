package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/setup"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

type preservationPicker struct {
	selected  map[string]bool
	cursor    int
	cancelled bool
}

func (m preservationPicker) Init() tea.Cmd { return nil }
func (m preservationPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			m.cursor = (m.cursor + len(setup.PreserveGroups) - 1) % len(setup.PreserveGroups)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(setup.PreserveGroups)
		case " ":
			id := setup.PreserveGroups[m.cursor].ID
			m.selected[id] = !m.selected[id]
		case "a":
			for _, g := range setup.PreserveGroups {
				m.selected[g.ID] = true
			}
		case "n":
			for _, g := range setup.PreserveGroups {
				m.selected[g.ID] = false
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m preservationPicker) View() string {
	var out strings.Builder
	out.WriteString("Choose which Ollama settings to preserve:\n\n")
	for i, g := range setup.PreserveGroups {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := " "
		if m.selected[g.ID] {
			mark = "x"
		}
		fmt.Fprintf(&out, "%s %d. [%s] %s\n", cursor, i+1, mark, g.Label)
	}
	out.WriteString("\nUp/Down: move · Space: toggle · A: all · N: none · Enter: continue\n")
	out.WriteString("Unchecked settings are not saved/restored. Current environment and running servers remain unchanged.\n")
	out.WriteString("Unchecked context uses Offcode's 8192-token default.\n")
	return out.String()
}
func (m preservationPicker) choices() []string {
	result := []string{}
	for _, g := range setup.PreserveGroups {
		if m.selected[g.ID] {
			result = append(result, g.ID)
		}
	}
	return result
}
func choosePreservation(selected []string) ([]string, error) {
	m := preservationPicker{selected: map[string]bool{}}
	for _, id := range selected {
		m.selected[id] = true
	}
	if term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		result, err := tea.NewProgram(m).Run()
		if err != nil {
			return nil, err
		}
		picker := result.(preservationPicker)
		if picker.cancelled {
			return nil, fmt.Errorf("setup cancelled")
		}
		return picker.choices(), nil
	}
	// A byte-at-a-time reader avoids buffering later wizard answers when stdin
	// is redirected. Real terminals use the arrow/space checkbox widget above.
	in := bufio.NewReaderSize(singleByteReader{os.Stdin}, 16)
	for {
		fmt.Print(m.View())
		fmt.Print("Toggle a checkbox by number (1-7), all, none; Enter to continue: ")
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("checkbox input: %w", err)
		}
		line = strings.TrimSpace(line)
		switch line {
		case "":
			return m.choices(), nil
		case "all":
			for _, g := range setup.PreserveGroups {
				m.selected[g.ID] = true
			}
		case "none":
			for _, g := range setup.PreserveGroups {
				m.selected[g.ID] = false
			}
		default:
			n, err := strconv.Atoi(line)
			if err != nil || n < 1 || n > len(setup.PreserveGroups) {
				fmt.Println("Choose a checkbox number, all or none.")
				continue
			}
			id := setup.PreserveGroups[n-1].ID
			m.selected[id] = !m.selected[id]
		}
	}
}

type singleByteReader struct{ io.Reader }

func (r singleByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}
