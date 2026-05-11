package tmux

import (
	"testing"
	"time"
)

func TestControlLoop_DetectStateChange(t *testing.T) {
	// This is a placeholder test - real testing requires a live tmux session
	// In practice, you would:
	// 1. Create a test tmux session
	// 2. Send a command that produces output
	// 3. Verify state change is detected

	// For now, just test that the control loop can be created
	executor := NewLocalExecutor()
	client := NewClient(executor)
	loop := NewControlLoop(client)

	if loop == nil {
		t.Fatal("Failed to create control loop")
	}
}

func TestValidators(t *testing.T) {
	tests := []struct {
		name      string
		validator StateValidator
		content   string
		expected  bool
	}{
		{
			name:      "ContainsValidator - found",
			validator: ContainsValidator("test"),
			content:   "this is a test string",
			expected:  true,
		},
		{
			name:      "ContainsValidator - not found",
			validator: ContainsValidator("missing"),
			content:   "this is a test string",
			expected:  false,
		},
		{
			name:      "NotContainsValidator - not found",
			validator: NotContainsValidator("missing"),
			content:   "this is a test string",
			expected:  true,
		},
		{
			name:      "NotContainsValidator - found",
			validator: NotContainsValidator("test"),
			content:   "this is a test string",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.validator(tt.content)
			if err != nil {
				t.Fatalf("Validator returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestVerifiers(t *testing.T) {
	tests := []struct {
		name     string
		verifier VerificationFunc
		before   string
		after    string
		expected bool
	}{
		{
			name:     "ContentChangedVerifier - changed",
			verifier: ContentChangedVerifier(),
			before:   "before",
			after:    "after",
			expected: true,
		},
		{
			name:     "ContentChangedVerifier - unchanged",
			verifier: ContentChangedVerifier(),
			before:   "same",
			after:    "same",
			expected: false,
		},
		{
			name:     "ContainsAfterVerifier - found",
			verifier: ContainsAfterVerifier("test"),
			before:   "before",
			after:    "after test content",
			expected: true,
		},
		{
			name:     "ContainsAfterVerifier - not found",
			verifier: ContainsAfterVerifier("missing"),
			before:   "before",
			after:    "after content",
			expected: false,
		},
		{
			name:     "NotContainsAfterVerifier - not found",
			verifier: NotContainsAfterVerifier("missing"),
			before:   "before",
			after:    "after content",
			expected: true,
		},
		{
			name:     "NotContainsAfterVerifier - found",
			verifier: NotContainsAfterVerifier("test"),
			before:   "before",
			after:    "after test content",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.verifier(tt.before, tt.after)
			if err != nil {
				t.Fatalf("Verifier returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultControlLoopOptions(t *testing.T) {
	opts := DefaultControlLoopOptions()

	if opts.PreCaptureDelay != 100*time.Millisecond {
		t.Errorf("Expected PreCaptureDelay 100ms, got %v", opts.PreCaptureDelay)
	}

	if opts.PostActionDelay != 500*time.Millisecond {
		t.Errorf("Expected PostActionDelay 500ms, got %v", opts.PostActionDelay)
	}

	if opts.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", opts.MaxRetries)
	}

	if opts.CaptureOptions == nil {
		t.Error("Expected CaptureOptions to be set")
	}
}
