package tmux

import (
	"fmt"
	"strings"
	"time"
)

// ControlLoop provides a high-level interface for capture → validate → send → verify workflows
type ControlLoop struct {
	client *Client
}

// NewControlLoop creates a new control loop
func NewControlLoop(client *Client) *ControlLoop {
	return &ControlLoop{
		client: client,
	}
}

// StateValidator is a function that validates pane state from captured content
type StateValidator func(content string) (bool, error)

// ActionFunc is a function that performs an action on a pane
type ActionFunc func(paneID string) error

// VerificationFunc is a function that verifies the result of an action
type VerificationFunc func(beforeContent, afterContent string) (bool, error)

// ControlLoopOptions configures the control loop behavior
type ControlLoopOptions struct {
	// PreCaptureDelay is the delay before initial capture
	PreCaptureDelay time.Duration
	// PostActionDelay is the delay after action before verification capture
	PostActionDelay time.Duration
	// MaxRetries is the maximum number of retries if verification fails
	MaxRetries int
	// CaptureOptions for pane capture
	CaptureOptions *CaptureOptions
}

// DefaultControlLoopOptions returns sensible defaults
func DefaultControlLoopOptions() *ControlLoopOptions {
	return &ControlLoopOptions{
		PreCaptureDelay:  100 * time.Millisecond,
		PostActionDelay:  500 * time.Millisecond,
		MaxRetries:       3,
		CaptureOptions:   DefaultCaptureOptions(),
	}
}

// ExecuteWithVerification executes a control loop: capture → validate → action → verify
func (cl *ControlLoop) ExecuteWithVerification(
	paneID string,
	validator StateValidator,
	action ActionFunc,
	verifier VerificationFunc,
	opts *ControlLoopOptions,
) error {
	if opts == nil {
		opts = DefaultControlLoopOptions()
	}

	// Validate pane exists
	if err := cl.client.ValidatePane(paneID); err != nil {
		return fmt.Errorf("pane validation failed: %w", err)
	}

	// Pre-capture delay
	if opts.PreCaptureDelay > 0 {
		time.Sleep(opts.PreCaptureDelay)
	}

	// Initial capture
	beforeCapture, err := cl.client.CapturePane(paneID, opts.CaptureOptions)
	if err != nil {
		return fmt.Errorf("initial capture failed: %w", err)
	}

	// Validate state
	valid, err := validator(beforeCapture.Content)
	if err != nil {
		return fmt.Errorf("state validation failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("pane state validation failed: expected state not found")
	}

	// Execute action
	if err := action(paneID); err != nil {
		return fmt.Errorf("action execution failed: %w", err)
	}

	// Post-action delay
	if opts.PostActionDelay > 0 {
		time.Sleep(opts.PostActionDelay)
	}

	// Verification capture
	afterCapture, err := cl.client.CapturePane(paneID, opts.CaptureOptions)
	if err != nil {
		return fmt.Errorf("verification capture failed: %w", err)
	}

	// Verify result
	verified, err := verifier(beforeCapture.Content, afterCapture.Content)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	if !verified {
		return fmt.Errorf("verification failed: expected state change not detected")
	}

	return nil
}

// Common validators and verifiers

// ContainsValidator returns a validator that checks if content contains a string
func ContainsValidator(needle string) StateValidator {
	return func(content string) (bool, error) {
		return strings.Contains(content, needle), nil
	}
}

// NotContainsValidator returns a validator that checks if content does not contain a string
func NotContainsValidator(needle string) StateValidator {
	return func(content string) (bool, error) {
		return !strings.Contains(content, needle), nil
	}
}

// ContentChangedVerifier returns a verifier that checks if content changed
func ContentChangedVerifier() VerificationFunc {
	return func(before, after string) (bool, error) {
		return before != after, nil
	}
}

// ContainsAfterVerifier returns a verifier that checks if after-content contains a string
func ContainsAfterVerifier(needle string) VerificationFunc {
	return func(before, after string) (bool, error) {
		return strings.Contains(after, needle), nil
	}
}

// NotContainsAfterVerifier returns a verifier that checks if after-content does not contain a string
func NotContainsAfterVerifier(needle string) VerificationFunc {
	return func(before, after string) (bool, error) {
		return !strings.Contains(after, needle), nil
	}
}

// ApprovePermissionPrompt is a high-level helper to approve a permission prompt
func (cl *ControlLoop) ApprovePermissionPrompt(paneID string, promptText string) error {
	validator := ContainsValidator(promptText)
	action := func(paneID string) error {
		return cl.client.SendEnter(paneID)
	}
	verifier := NotContainsAfterVerifier(promptText)

	return cl.ExecuteWithVerification(paneID, validator, action, verifier, nil)
}

// SendReplyToQuestion is a high-level helper to send a reply to a question
func (cl *ControlLoop) SendReplyToQuestion(paneID string, questionText string, reply string) error {
	validator := ContainsValidator(questionText)
	action := func(paneID string) error {
		if err := cl.client.SendText(paneID, reply, &SendTextOptions{Literal: true}); err != nil {
			return err
		}
		return cl.client.SendEnter(paneID)
	}
	verifier := ContainsAfterVerifier(reply)

	return cl.ExecuteWithVerification(paneID, validator, action, verifier, nil)
}

// DetectStateChange monitors a pane for state changes
func (cl *ControlLoop) DetectStateChange(paneID string, checkInterval time.Duration, timeout time.Duration) (string, string, error) {
	// Initial capture
	before, err := cl.client.GetRecentContent(paneID, 100)
	if err != nil {
		return "", "", err
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		after, err := cl.client.GetRecentContent(paneID, 100)
		if err != nil {
			return "", "", err
		}

		if after.Content != before.Content {
			return before.Content, after.Content, nil
		}
	}

	return "", "", fmt.Errorf("no state change detected within timeout")
}
