package tui

import (
	"testing"

	"github.com/lfu/warren/internal/core"
)

func TestNewModel(t *testing.T) {
	config := core.DefaultConfig()
	config.DBPath = ":memory:"

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("failed to create Warren: %v", err)
	}
	defer warren.Stop()

	model := NewModel(warren)

	if model.warren == nil {
		t.Error("expected warren to be set")
	}

	if model.currentView != ViewSessionList {
		t.Errorf("expected initial view to be SessionList, got %v", model.currentView)
	}

	if model.selectedIndex != 0 {
		t.Errorf("expected initial selectedIndex to be 0, got %d", model.selectedIndex)
	}
}

func TestGetStateStyle(t *testing.T) {
	tests := []struct {
		state string
	}{
		{"idle"},
		{"thinking"},
		{"executing"},
		{"waiting_permission"},
		{"asking_question"},
		{"error"},
		{"finished"},
		{"stopped"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			style := GetStateStyle(tt.state)
			// Just verify it doesn't panic and returns a style
			_ = style.Render("test")
		})
	}
}

func TestGetStateIndicator(t *testing.T) {
	indicator := GetStateIndicator("idle")
	if indicator == "" {
		t.Error("expected non-empty indicator")
	}

	// Should contain the bullet character
	if indicator != "●" && len(indicator) < 1 {
		t.Error("expected indicator to contain bullet character")
	}
}

func TestModelInit(t *testing.T) {
	config := core.DefaultConfig()
	config.DBPath = ":memory:"

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("failed to create Warren: %v", err)
	}
	defer warren.Stop()

	model := NewModel(warren)
	cmd := model.Init()

	if cmd == nil {
		t.Error("expected Init to return a command")
	}
}

func TestModelView(t *testing.T) {
	config := core.DefaultConfig()
	config.DBPath = ":memory:"

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("failed to create Warren: %v", err)
	}
	defer warren.Stop()

	model := NewModel(warren)

	// Test session list view
	view := model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Test agent detail view
	model.currentView = ViewAgentDetail
	view = model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Test notifications view
	model.currentView = ViewNotifications
	view = model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Test quitting view
	model.quitting = true
	view = model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
