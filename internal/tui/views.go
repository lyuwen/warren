package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSessionList renders the session list view
func (m Model) renderSessionList() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render("Warren - Agent Sessions")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Session list
	if len(m.sessionList) == 0 {
		b.WriteString(normalItemStyle.Render("No active sessions"))
		b.WriteString("\n")
	} else {
		for i, agentID := range m.sessionList {
			// Get agent state
			state, err := m.warren.GetSessionState(agentID)
			stateStr := "unknown"
			if err == nil {
				stateStr = string(state)
			}

			// Format line
			indicator := GetStateIndicator(stateStr)
			line := fmt.Sprintf("%s %s (%s)", indicator, agentID, stateStr)

			// Apply selection style
			if i == m.selectedIndex {
				line = selectedItemStyle.Render(line)
			} else {
				line = normalItemStyle.Render(line)
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Notification badge
	notifCount := len(m.notifications)
	if notifCount > 0 {
		badge := notificationBadgeStyle.Render(fmt.Sprintf(" %d notifications ", notifCount))
		b.WriteString(badge)
		b.WriteString("\n\n")
	}

	// Help text
	help := helpStyle.Render("↑/↓: navigate • enter: details • n: notifications • q: quit")
	b.WriteString(help)

	return borderStyle.Render(b.String())
}

// renderAgentDetail renders the agent detail view
func (m Model) renderAgentDetail() string {
	var b strings.Builder

	if m.selectedAgentID == "" {
		return borderStyle.Render("No agent selected")
	}

	// Title
	title := titleStyle.Render(fmt.Sprintf("Agent: %s", m.selectedAgentID))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Current state
	state, err := m.warren.GetSessionState(m.selectedAgentID)
	if err != nil {
		b.WriteString(normalItemStyle.Render(fmt.Sprintf("Error: %v", err)))
	} else {
		stateStyle := GetStateStyle(string(state))
		b.WriteString(normalItemStyle.Render("State: "))
		b.WriteString(stateStyle.Render(string(state)))
		b.WriteString("\n\n")
	}

	// Artifact profile
	profile, err := m.warren.GetArtifactProfile(m.selectedAgentID)
	if err == nil && profile != nil {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Files Touched:"))
		b.WriteString("\n")

		if len(profile.FilesVisited) == 0 {
			b.WriteString(normalItemStyle.Render("  No files yet"))
			b.WriteString("\n")
		} else {
			// Show up to 10 most recent files
			maxFiles := 10
			if len(profile.FilesVisited) < maxFiles {
				maxFiles = len(profile.FilesVisited)
			}

			for i := 0; i < maxFiles; i++ {
				filePath := profile.FilesVisited[len(profile.FilesVisited)-1-i]
				edited := ""
				for _, editedFile := range profile.FilesEdited {
					if editedFile == filePath {
						edited = " [edited]"
						break
					}
				}
				b.WriteString(normalItemStyle.Render(fmt.Sprintf("  • %s%s", filePath, edited)))
				b.WriteString("\n")
			}

			if len(profile.FilesVisited) > maxFiles {
				b.WriteString(normalItemStyle.Render(fmt.Sprintf("  ... and %d more", len(profile.FilesVisited)-maxFiles)))
				b.WriteString("\n")
			}
		}

		b.WriteString("\n")
		b.WriteString(normalItemStyle.Render(fmt.Sprintf("Total: %d reads, %d edits, %d writes",
			profile.TotalReads, profile.TotalEdits, profile.TotalWrites)))
		b.WriteString("\n")

		// Repo roots
		if len(profile.RepoRoots) > 0 {
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Bold(true).Render("Repositories:"))
			b.WriteString("\n")
			for _, repo := range profile.RepoRoots {
				b.WriteString(normalItemStyle.Render(fmt.Sprintf("  • %s", repo)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")

	// Help text
	help := helpStyle.Render("←/esc: back • q: quit")
	b.WriteString(help)

	return borderStyle.Render(b.String())
}

// renderNotifications renders the notifications view
func (m Model) renderNotifications() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render("Notifications")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Notification list
	if len(m.notifications) == 0 {
		b.WriteString(normalItemStyle.Render("No notifications"))
		b.WriteString("\n")
	} else {
		for _, notif := range m.notifications {
			b.WriteString(notificationBadgeStyle.Render("!"))
			b.WriteString(" ")
			b.WriteString(normalItemStyle.Render(notif))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Help text
	help := helpStyle.Render("←/esc: back • q: quit")
	b.WriteString(help)

	return borderStyle.Render(b.String())
}
