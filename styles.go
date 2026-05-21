package main

import "github.com/charmbracelet/lipgloss"

var (
	styleTime  = lipgloss.NewStyle().Faint(true)
	styleInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stylePair  = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	stylePath  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styleKey   = lipgloss.NewStyle().Faint(true)
	styleOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	styleBad   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

func styleLevel(level string) lipgloss.Style {
	switch level {
	case "INFO":
		return styleInfo
	case "WARN":
		return styleWarn
	case "ERROR":
		return styleError
	default:
		return lipgloss.NewStyle()
	}
}
