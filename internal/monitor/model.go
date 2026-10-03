package monitor

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/sysmon"
)

const historyLen = 60

type Model struct {
	client     *backend.Client
	ollamaRoot string

	width, height int
	quitting      bool

	cpuPrev    sysmon.CPUSample
	cpuPercent float64
	cpuHistory []float64

	mem   sysmon.Memory
	memOK bool

	gpu        sysmon.GPU
	gpuOK      bool
	gpuHistory []float64

	running    []backend.LoadedModel
	runningErr string

	lastUpdate time.Time
}

func New(ollamaRoot string) Model {
	return Model{
		client:     backend.NewClient(ollamaRoot),
		ollamaRoot: ollamaRoot,
	}
}

func Run(ollamaRoot string) error {
	p := tea.NewProgram(New(ollamaRoot), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type tickMsg time.Time

type sampleMsg struct {
	cpuPercent float64
	cpuNext    sysmon.CPUSample
	mem        sysmon.Memory
	memOK      bool
	gpu        sysmon.GPU
	gpuOK      bool
}

type runningMsg struct {
	models []backend.LoadedModel
	err    error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) sampleCmd() tea.Cmd {
	prev := m.cpuPrev
	return func() tea.Msg {
		cpuPercent, next, _ := sysmon.ReadCPU(prev)
		mem, memOK := sysmon.ReadMemory()
		gpu, gpuOK := sysmon.ReadGPU()
		return sampleMsg{
			cpuPercent: cpuPercent,
			cpuNext:    next,
			mem:        mem,
			memOK:      memOK,
			gpu:        gpu,
			gpuOK:      gpuOK,
		}
	}
}

func (m Model) fetchRunningCmd() tea.Cmd {
	client := m.client
	root := m.ollamaRoot
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		models, err := client.RunningModels(ctx, root)
		return runningMsg{models: models, err: err}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.sampleCmd(), m.fetchRunningCmd())
}

func pushHistory(hist []float64, v float64) []float64 {
	hist = append(hist, v)
	if len(hist) > historyLen {
		hist = hist[len(hist)-historyLen:]
	}
	return hist
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc, tea.KeyCtrlD:
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(tickCmd(), m.sampleCmd(), m.fetchRunningCmd())

	case sampleMsg:
		m.cpuPrev = msg.cpuNext
		m.cpuPercent = msg.cpuPercent
		m.cpuHistory = pushHistory(m.cpuHistory, msg.cpuPercent)
		m.mem = msg.mem
		m.memOK = msg.memOK
		m.gpu = msg.gpu
		m.gpuOK = msg.gpuOK
		if msg.gpuOK {
			m.gpuHistory = pushHistory(m.gpuHistory, msg.gpu.BusyPercent)
		}
		m.lastUpdate = time.Now()
		return m, nil

	case runningMsg:
		if msg.err != nil {
			m.runningErr = msg.err.Error()
			m.running = nil
		} else {
			m.runningErr = ""
			m.running = msg.models
		}
		return m, nil
	}
	return m, nil
}
