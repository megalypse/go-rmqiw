package tui

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/tui/components"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
	"github.com/megalypse/go/rmqiw/internal/tui/views"
)

func NewModel() (*RootView, error) {
	killWatch := make(chan struct{}, 1)

	return &RootView{
		killWatch: killWatch,
	}, nil

}

type RootView struct {
	router    tea.Model
	killWatch chan struct{}
}

func (m *RootView) Init() tea.Cmd {
	ctx, _ := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	go func() {
		<-ctx.Done()
		m.killWatch <- struct{}{}
	}()

	return nil
}

func (m *RootView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.router, cmd = m.router.Update(msg)

	select {
	case <-m.killWatch:
		return m, tea.Quit
	default:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m *RootView) View() string {
	if m.router == nil {
		m.router = views.NewViewSelectJourney()
	}

	return components.NewTitle("RMQ in Wonderland") +
		ui.LineBreak +
		m.router.View()
}
