package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lfu/warren/internal/claude"
	"github.com/lfu/warren/internal/core"
)

// View represents different screens in the TUI
type View int

const (
	ViewSessionList View = iota
	ViewAgentDetail
	ViewConversation
	ViewNotifications
)

// Model is the main Bubble Tea model
type Model struct {
	warren               *core.Warren
	conversationService  *core.ConversationService
	currentView          View
	sessionList          []string // List of agent IDs
	selectedIndex        int
	selectedAgentID      string
	notifications        []string
	conversationMessages []*claude.Message
	conversationScroll   int
	conversationError    string
	width                int
	height               int
	err                  error
	quitting             bool
}

// NewModel creates a new TUI model
func NewModel(warren *core.Warren) Model {
	return Model{
		warren:              warren,
		conversationService: core.NewConversationService(),
		currentView:         ViewSessionList,
		sessionList:         []string{},
		selectedIndex:       0,
		width:               80,
		height:              24,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

// tickCmd returns a command that ticks every 500ms
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Refresh data from Warren
		m.refreshData()
		return m, tickCmd()

	case error:
		m.err = msg
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.currentView == ViewConversation {
			if m.conversationScroll > 0 {
				m.conversationScroll--
			}
		} else if m.selectedIndex > 0 {
			m.selectedIndex--
			m.updateSelectedAgent()
		}
		return m, nil

	case "down", "j":
		if m.currentView == ViewConversation {
			maxScroll := len(m.conversationMessages) - 1
			if m.conversationScroll < maxScroll {
				m.conversationScroll++
			}
		} else {
			maxIndex := len(m.sessionList) - 1
			if m.selectedIndex < maxIndex {
				m.selectedIndex++
				m.updateSelectedAgent()
			}
		}
		return m, nil

	case "enter", "right", "l":
		if m.currentView == ViewSessionList && len(m.sessionList) > 0 {
			m.currentView = ViewAgentDetail
		}
		return m, nil

	case "c":
		if m.currentView == ViewAgentDetail && m.selectedAgentID != "" {
			m.currentView = ViewConversation
			m.loadConversation()
		}
		return m, nil

	case "left", "h", "esc":
		if m.currentView == ViewConversation {
			m.currentView = ViewAgentDetail
		} else if m.currentView != ViewSessionList {
			m.currentView = ViewSessionList
		}
		return m, nil

	case "n":
		m.currentView = ViewNotifications
		return m, nil

	case "tab":
		// Cycle through views
		m.currentView = (m.currentView + 1) % 4
		if m.currentView == ViewConversation && m.selectedAgentID != "" {
			m.loadConversation()
		}
		return m, nil
	}

	return m, nil
}

// refreshData updates the model with fresh data from Warren
func (m *Model) refreshData() {
	sessions := m.warren.GetAllSessions()
	m.sessionList = make([]string, 0, len(sessions))
	for _, session := range sessions {
		m.sessionList = append(m.sessionList, session.AgentID)
	}

	// Update selected agent if list changed
	if m.selectedIndex >= len(m.sessionList) {
		m.selectedIndex = len(m.sessionList) - 1
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
	}
	m.updateSelectedAgent()

	// Refresh notifications
	notifs, err := m.warren.GetUnconsumedNotifications()
	if err == nil {
		m.notifications = make([]string, 0, len(notifs))
		for _, notif := range notifs {
			m.notifications = append(m.notifications, fmt.Sprintf("[%s] %s", notif.AgentID, notif.Message))
		}
	}
}

// updateSelectedAgent updates the currently selected agent ID
func (m *Model) updateSelectedAgent() {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.sessionList) {
		m.selectedAgentID = m.sessionList[m.selectedIndex]
	} else {
		m.selectedAgentID = ""
	}
}

// loadConversation loads conversation history for the selected agent
func (m *Model) loadConversation() {
	if m.selectedAgentID == "" {
		m.conversationError = "No agent selected"
		return
	}

	// Get agent session from Warren
	session, err := m.warren.GetSession(m.selectedAgentID)
	if err != nil {
		m.conversationError = fmt.Sprintf("Failed to get session: %v", err)
		m.conversationMessages = nil
		return
	}

	// Get server info
	server, err := m.warren.GetServer(session.ServerName)
	if err != nil {
		m.conversationError = fmt.Sprintf("Failed to get server: %v", err)
		m.conversationMessages = nil
		return
	}

	// Get pane info
	pane, err := m.warren.GetPane(session, server)
	if err != nil {
		m.conversationError = fmt.Sprintf("Failed to get pane: %v", err)
		m.conversationMessages = nil
		return
	}

	// Load conversation from Claude session files
	messages, err := m.conversationService.GetRecentMessages(session, server, pane, 50)
	if err != nil {
		m.conversationError = fmt.Sprintf("Failed to load conversation: %v", err)
		m.conversationMessages = nil
		return
	}

	// Filter to user/assistant messages only
	m.conversationMessages = claude.FilterUserAssistant(messages)
	m.conversationError = ""
	m.conversationScroll = 0
}

// View renders the current view
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	switch m.currentView {
	case ViewSessionList:
		return m.renderSessionList()
	case ViewAgentDetail:
		return m.renderAgentDetail()
	case ViewConversation:
		return m.renderConversation()
	case ViewNotifications:
		return m.renderNotifications()
	default:
		return "Unknown view\n"
	}
}
