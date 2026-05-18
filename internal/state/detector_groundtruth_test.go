package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lfu/warren/internal/types"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	testdataDir := filepath.Join(filepath.Dir(thisFile), "..", "testdata")
	data, err := os.ReadFile(filepath.Join(testdataDir, name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return string(data)
}

func TestGroundTruth_IdleCompletedLocalhost(t *testing.T) {
	content := loadFixture(t, "idle_completed_localhost.txt")
	d := NewStateDetector()
	result := d.DetectFromContent(content)
	t.Logf("State: %s, Confidence: %.2f", result.State, result.Confidence)
	for _, s := range result.Signals {
		t.Logf("  Signal: %s", s)
	}
	if result.State != types.StateIdle && result.State != types.StateFinished {
		t.Errorf("expected StateIdle or StateFinished, got %s", result.State)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %.2f", result.Confidence)
	}
}

func TestGroundTruth_RunningLocalhost(t *testing.T) {
	content := loadFixture(t, "running_localhost.txt")
	d := NewStateDetector()
	result := d.DetectFromContent(content)
	t.Logf("State: %s, Confidence: %.2f", result.State, result.Confidence)
	for _, s := range result.Signals {
		t.Logf("  Signal: %s", s)
	}
	if result.State != types.StateExecuting {
		t.Errorf("expected StateExecuting, got %s", result.State)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %.2f", result.Confidence)
	}
}

func TestGroundTruth_AskingQuestionFoundry2(t *testing.T) {
	content := loadFixture(t, "asking_question_foundry2.txt")
	d := NewStateDetector()
	result := d.DetectFromContent(content)
	t.Logf("State: %s, Confidence: %.2f", result.State, result.Confidence)
	for _, s := range result.Signals {
		t.Logf("  Signal: %s", s)
	}
	if result.State != types.StateAskingQuestion {
		t.Errorf("expected StateAskingQuestion, got %s", result.State)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %.2f", result.Confidence)
	}
}

func TestGroundTruth_AskingPermissionFoundry2(t *testing.T) {
	content := loadFixture(t, "asking_permission_foundry2.txt")
	d := NewStateDetector()
	result := d.DetectFromContent(content)
	t.Logf("State: %s, Confidence: %.2f", result.State, result.Confidence)
	for _, s := range result.Signals {
		t.Logf("  Signal: %s", s)
	}
	if result.State != types.StateWaitingPermission {
		t.Errorf("expected StateWaitingPermission, got %s", result.State)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %.2f", result.Confidence)
	}
}

func TestGroundTruth_IdleCompletedFoundry2(t *testing.T) {
	content := loadFixture(t, "idle_completed_foundry2.txt")
	d := NewStateDetector()
	result := d.DetectFromContent(content)
	t.Logf("State: %s, Confidence: %.2f", result.State, result.Confidence)
	for _, s := range result.Signals {
		t.Logf("  Signal: %s", s)
	}
	if result.State != types.StateIdle && result.State != types.StateFinished {
		t.Errorf("expected StateIdle or StateFinished, got %s", result.State)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %.2f", result.Confidence)
	}
}
