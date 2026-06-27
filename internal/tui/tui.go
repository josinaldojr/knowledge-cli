package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Start launches the interactive terminal user interface.
func Start() error {
	m := NewModel()
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
