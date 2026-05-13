package core

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.PollInterval != 500*time.Millisecond {
		t.Errorf("expected PollInterval 500ms, got %v", config.PollInterval)
	}
	if config.MinConfidence != 0.7 {
		t.Errorf("expected MinConfidence 0.7, got %v", config.MinConfidence)
	}
	if config.DBPath != "warren.db" {
		t.Errorf("expected DBPath 'warren.db', got %v", config.DBPath)
	}
	if config.ConfigDir != ".warren" {
		t.Errorf("expected ConfigDir '.warren', got %v", config.ConfigDir)
	}
	if config.EventRetentionPeriod != 30*24*time.Hour {
		t.Errorf("expected EventRetentionPeriod 30 days, got %v", config.EventRetentionPeriod)
	}
	if config.EventPruningInterval != 24*time.Hour {
		t.Errorf("expected EventPruningInterval 24 hours, got %v", config.EventPruningInterval)
	}
	if config.CacheTTL != 5*time.Second {
		t.Errorf("expected CacheTTL 5 seconds, got %v", config.CacheTTL)
	}
	if config.RegistryPruneThreshold != 24*time.Hour {
		t.Errorf("expected RegistryPruneThreshold 24 hours, got %v", config.RegistryPruneThreshold)
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	config := DefaultConfig()
	if err := config.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestConfigValidate_PollInterval(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		expectError bool
		errorMsg    string
	}{
		{"zero", 0, true, "must be positive"},
		{"negative", -1 * time.Second, true, "must be positive"},
		{"too small", 50 * time.Millisecond, true, "must be at least 100ms"},
		{"minimum valid", 100 * time.Millisecond, false, ""},
		{"valid", 500 * time.Millisecond, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.PollInterval = tt.interval
			err := config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestConfigValidate_MinConfidence(t *testing.T) {
	tests := []struct {
		name        string
		confidence  float64
		expectError bool
	}{
		{"negative", -0.1, true},
		{"zero", 0.0, false},
		{"valid", 0.7, false},
		{"one", 1.0, false},
		{"too high", 1.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.MinConfidence = tt.confidence
			err := config.Validate()

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestConfigValidate_DBPath(t *testing.T) {
	config := DefaultConfig()
	config.DBPath = ""
	err := config.Validate()

	if err == nil {
		t.Error("expected error for empty DBPath, got nil")
	}
	if !strings.Contains(err.Error(), "DBPath cannot be empty") {
		t.Errorf("expected error about DBPath, got %v", err)
	}
}

func TestConfigValidate_ConfigDir(t *testing.T) {
	config := DefaultConfig()
	config.ConfigDir = ""
	err := config.Validate()

	if err == nil {
		t.Error("expected error for empty ConfigDir, got nil")
	}
	if !strings.Contains(err.Error(), "ConfigDir cannot be empty") {
		t.Errorf("expected error about ConfigDir, got %v", err)
	}
}

func TestConfigValidate_EventRetentionPeriod(t *testing.T) {
	tests := []struct {
		name        string
		period      time.Duration
		expectError bool
	}{
		{"zero", 0, true},
		{"negative", -1 * time.Hour, true},
		{"valid", 30 * 24 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.EventRetentionPeriod = tt.period
			err := config.Validate()

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestConfigValidate_EventPruningInterval(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		expectError bool
	}{
		{"zero", 0, true},
		{"negative", -1 * time.Hour, true},
		{"valid", 24 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.EventPruningInterval = tt.interval
			err := config.Validate()

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestConfigValidate_CacheTTL(t *testing.T) {
	tests := []struct {
		name        string
		ttl         time.Duration
		expectError bool
		errorMsg    string
	}{
		{"zero", 0, true, "must be positive"},
		{"negative", -1 * time.Second, true, "must be positive"},
		{"valid small", 1 * time.Second, false, ""},
		{"valid default", 5 * time.Second, false, ""},
		{"valid large", 1 * time.Hour, false, ""},
		{"too large", 2 * time.Hour, true, "must be at most 1 hour"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CacheTTL = tt.ttl
			err := config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestConfigValidate_RegistryPruneThreshold(t *testing.T) {
	tests := []struct {
		name        string
		threshold   time.Duration
		expectError bool
		errorMsg    string
	}{
		{"zero", 0, true, "must be positive"},
		{"negative", -1 * time.Hour, true, "must be positive"},
		{"too small", 30 * time.Minute, true, "must be at least 1 hour"},
		{"minimum valid", 1 * time.Hour, false, ""},
		{"valid", 24 * time.Hour, false, ""},
		{"valid large", 7 * 24 * time.Hour, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.RegistryPruneThreshold = tt.threshold
			err := config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
