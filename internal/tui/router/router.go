package router

import tea "github.com/charmbracelet/bubbletea"

type Router interface {
	Push(route tea.Model)
}
