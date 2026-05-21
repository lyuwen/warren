package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lfu/warren/internal/core"
)

func TestHandleGetTopology(t *testing.T) {
	// Create a test Warren instance with default config
	tmpDir := t.TempDir()
	config := core.DefaultConfig()
	config.DBPath = tmpDir + "/test.db"
	config.ConfigDir = tmpDir

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	// Create test server
	server := &Server{
		warren:              warren,
		conversationService: core.NewConversationServiceWithTTL(warren.CacheTTL()),
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/topology", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetTopology(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := response["servers"]; !ok {
		t.Error("Expected 'servers' field in response")
	}
}

func TestHandleGetTopologyMethodNotAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	config := core.DefaultConfig()
	config.DBPath = tmpDir + "/test.db"
	config.ConfigDir = tmpDir

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	server := &Server{
		warren:              warren,
		conversationService: core.NewConversationServiceWithTTL(warren.CacheTTL()),
	}

	// Test POST method (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/api/topology", nil)
	w := httptest.NewRecorder()

	server.handleGetTopology(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetTopologyServer(t *testing.T) {
	tmpDir := t.TempDir()
	config := core.DefaultConfig()
	config.DBPath = tmpDir + "/test.db"
	config.ConfigDir = tmpDir

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	server := &Server{
		warren:              warren,
		conversationService: core.NewConversationServiceWithTTL(warren.CacheTTL()),
	}

	// Test with server name
	req := httptest.NewRequest(http.MethodGet, "/api/topology/servers/local", nil)
	w := httptest.NewRecorder()

	server.handleGetTopologyServer(w, req)

	// Should return 404 if server not found, or 200 if found
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200, 404, or 500, got %d", w.Code)
	}
}

func TestHandleGetTopologyServerNoName(t *testing.T) {
	tmpDir := t.TempDir()
	config := core.DefaultConfig()
	config.DBPath = tmpDir + "/test.db"
	config.ConfigDir = tmpDir

	warren, err := core.NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	server := &Server{
		warren:              warren,
		conversationService: core.NewConversationServiceWithTTL(warren.CacheTTL()),
	}

	// Test without server name
	req := httptest.NewRequest(http.MethodGet, "/api/topology/servers/", nil)
	w := httptest.NewRecorder()

	server.handleGetTopologyServer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
