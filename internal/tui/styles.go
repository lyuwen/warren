package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Color scheme
	colorGreen  = lipgloss.Color("#00ff00")
	colorYellow = lipgloss.Color("#ffff00")
	colorBlue   = lipgloss.Color("#00ffff")
	colorRed    = lipgloss.Color("#ff0000")
	colorGray   = lipgloss.Color("#808080")
	colorWhite  = lipgloss.Color("#ffffff")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(lipgloss.Color("#5f5fff")).
			Padding(0, 1)

	// Status styles by state
	statusIdleStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	statusThinkingStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	statusWaitingStyle = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)

	statusFinishedStyle = lipgloss.NewStyle().
				Foreground(colorGray).
				Bold(true)

	// List item styles
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(lipgloss.Color("#5f5fff")).
				Bold(true).
				Padding(0, 1)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Padding(0, 1)

	// Help text
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Padding(1, 0)

	// Border styles
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5f5fff")).
			Padding(1, 2)

	// Notification badge
	notificationBadgeStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorRed).
				Bold(true).
				Padding(0, 1)

	// Conversation message styles
	userMessageStyle = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true)

	assistantMessageStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	toolCallStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)
)

// GetStateStyle returns the appropriate style for an agent state
func GetStateStyle(state string) lipgloss.Style {
	switch state {
	case "idle":
		return statusIdleStyle
	case "thinking", "executing":
		return statusThinkingStyle
	case "waiting_permission", "asking_question":
		return statusWaitingStyle
	case "error":
		return statusErrorStyle
	case "finished", "stopped":
		return statusFinishedStyle
	default:
		return normalItemStyle
	}
}

// GetStateIndicator returns a colored indicator for the state
func GetStateIndicator(state string) string {
	style := GetStateStyle(state)
	return style.Render("●")
}
