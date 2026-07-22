package tui

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	poller2 "github.com/megalypse/go/rmqiw/internal/domain/impl/poller"
	"github.com/megalypse/go/rmqiw/internal/domain/impl/publisher"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
	"github.com/megalypse/go/rmqiw/internal/tui/components"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
	"github.com/megalypse/go/rmqiw/internal/tui/views"
)

func NewModel() (*RootView, error) {
	killWatch := make(chan struct{}, 1)
	profile, err := cfg.GetCfg()
	if err != nil {
		return nil, err
	}

	return &RootView{
		router:           views.NewViewSelectJourney(),
		killWatch:        killWatch,
		profile:          profile,
		profileName:      profile.Name,
		connectionStatus: profileChecking,
	}, nil
}

type RootView struct {
	router           tea.Model
	killWatch        chan struct{}
	profile          *cfg.Config
	profileName      string
	profileErr       error
	connectionStatus profileConnectionStatus
	connectionErr    error
}

type profileConnectionStatus uint8

const (
	profileChecking profileConnectionStatus = iota
	profileConnected
	profileDisconnected
)

type profileConnectionMsg struct {
	profile *cfg.Config
	err     error
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

	return checkProfileConnections(m.profile)
}

func (m *RootView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if connectionMsg, ok := msg.(profileConnectionMsg); ok {
		if connectionMsg.profile == m.profile {
			m.connectionErr = connectionMsg.err
			if connectionMsg.err == nil {
				m.connectionStatus = profileConnected
			} else {
				m.connectionStatus = profileDisconnected
			}
		}
		return m, nil
	}

	select {
	case <-m.killWatch:
		return m, tea.Quit
	default:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "shift+tab":
				profile, err := cfg.NextProfile()
				m.profileErr = err
				if err == nil {
					m.profile = profile
					m.profileName = profile.Name
					m.connectionStatus = profileChecking
					m.connectionErr = nil
					return m, checkProfileConnections(profile)
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.router, cmd = m.router.Update(msg)
	return m, cmd
}

func (m *RootView) View() string {
	profileStatus := "Profile: " + m.profileName + " " + m.connectionIcon() + " (Shift+Tab to switch)"
	if m.profileErr != nil {
		profileStatus = "Failed to switch profile: " + m.profileErr.Error()
	}

	content := m.router.View()
	if m.connectionErr != nil {
		content = lipgloss.NewStyle().Foreground(colors.ErrorRed).Render(
			"Connection error: " + m.connectionErr.Error(),
		)
	}

	return components.NewTitle("RMQ in Wonderland") +
		ui.LineBreak +
		profileStatus +
		ui.LineSkip +
		content
}

func (m *RootView) connectionIcon() string {
	switch m.connectionStatus {
	case profileConnected:
		return lipgloss.NewStyle().Foreground(colors.SuccessGreen).Render("✓")
	case profileDisconnected:
		return lipgloss.NewStyle().Foreground(colors.ErrorRed).Render("✗")
	default:
		return lipgloss.NewStyle().Foreground(colors.MainColor).Render("◌")
	}
}

func checkProfileConnections(profile *cfg.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := poller2.CheckConnection(ctx, profile); err != nil {
			return profileConnectionMsg{profile: profile, err: err}
		}
		if err := publisher.CheckConnection(profile); err != nil {
			return profileConnectionMsg{profile: profile, err: err}
		}

		return profileConnectionMsg{profile: profile}
	}
}
