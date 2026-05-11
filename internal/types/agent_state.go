package types

// AgentState represents the current state of an agent session
type AgentState string

const (
	StateUnknown           AgentState = "unknown"
	StateIdle              AgentState = "idle"
	StateThinking          AgentState = "thinking"
	StateExecuting         AgentState = "executing"
	StateWaitingPermission AgentState = "waiting_permission"
	StateAskingQuestion    AgentState = "asking_question"
	StateFinished          AgentState = "finished"
	StateError             AgentState = "error"
	StateStopped           AgentState = "stopped"
)

// IsActionable returns true if the agent needs user attention
func (s AgentState) IsActionable() bool {
	return s == StateWaitingPermission || s == StateAskingQuestion || s == StateError || s == StateFinished
}

// IsTerminal returns true if the agent has reached a terminal state
func (s AgentState) IsTerminal() bool {
	return s == StateFinished || s == StateStopped || s == StateError
}

// String returns the string representation of the state
func (s AgentState) String() string {
	return string(s)
}
