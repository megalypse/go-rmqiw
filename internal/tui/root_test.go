package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewReplacesJourneyListWithConnectionError(t *testing.T) {
	model := &RootView{
		router:           staticModel("journey list"),
		profileName:      "local",
		connectionStatus: profileDisconnected,
		connectionErr:    errors.New("connection refused"),
	}

	view := model.View()
	if !strings.Contains(view, "Connection error: connection refused") {
		t.Fatalf("expected connection error in view, got %q", view)
	}
	if strings.Contains(view, "journey list") {
		t.Fatalf("expected connection error to replace journey list, got %q", view)
	}
}

type staticModel string

func (staticModel) Init() tea.Cmd {
	return nil
}

func (m staticModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m staticModel) View() string {
	return string(m)
}
