package state

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/types"
)

// TestStateDetector_Performance_DetectFromContent tests that content detection is fast
func TestStateDetector_Performance_DetectFromContent(t *testing.T) {
	detector := NewStateDetector()

	// Generate realistic content
	content := generateRealisticContent(1000) // 1000 lines

	// Measure detection time
	start := time.Now()
	result := detector.DetectFromContent(content)
	elapsed := time.Since(start)

	// Should complete in under 10ms
	if elapsed > 10*time.Millisecond {
		t.Errorf("DetectFromContent took %v, expected < 10ms", elapsed)
	}

	// Should still produce valid result
	if result.State == types.StateUnknown && result.Confidence < 0.3 {
		t.Error("Performance test should still produce reasonable detection")
	}

	t.Logf("DetectFromContent processed %d lines in %v", 1000, elapsed)
}

// TestStateDetector_Performance_DetectFromActivities tests that activity detection is fast
func TestStateDetector_Performance_DetectFromActivities(t *testing.T) {
	detector := NewStateDetector()

	// Generate many activities
	activities := generateManyActivities(100) // 100 activities

	// Measure detection time
	start := time.Now()
	result := detector.DetectFromActivities(activities)
	elapsed := time.Since(start)

	// Should complete in under 10ms
	if elapsed > 10*time.Millisecond {
		t.Errorf("DetectFromActivities took %v, expected < 10ms", elapsed)
	}

	// Should produce valid result
	if result.State == types.StateUnknown {
		t.Error("Performance test should produce valid state")
	}

	t.Logf("DetectFromActivities processed %d activities in %v", 100, elapsed)
}

// TestStateDetector_Performance_RepeatedDetection tests repeated detection calls
func TestStateDetector_Performance_RepeatedDetection(t *testing.T) {
	detector := NewStateDetector()
	content := generateRealisticContent(500)

	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		detector.DetectFromContent(content)
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(iterations)

	// Average should be well under 10ms
	if avgTime > 10*time.Millisecond {
		t.Errorf("Average detection time %v, expected < 10ms", avgTime)
	}

	t.Logf("Average detection time over %d iterations: %v", iterations, avgTime)
}

// TestStateDetector_Performance_LargeContent tests handling of very large content
func TestStateDetector_Performance_LargeContent(t *testing.T) {
	detector := NewStateDetector()

	// Generate very large content (10,000 lines)
	content := generateRealisticContent(10000)

	start := time.Now()
	result := detector.DetectFromContent(content)
	elapsed := time.Since(start)

	// Should still complete reasonably fast (under 50ms)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Large content detection took %v, expected < 50ms", elapsed)
	}

	// Should still produce valid result
	if result == nil {
		t.Error("Should produce result even for large content")
	}

	t.Logf("Processed %d lines in %v", 10000, elapsed)
}

// TestStateDetector_Performance_ManyActivities tests handling of many activities
func TestStateDetector_Performance_ManyActivities(t *testing.T) {
	detector := NewStateDetector()

	// Generate many activities (1000)
	activities := generateManyActivities(1000)

	start := time.Now()
	result := detector.DetectFromActivities(activities)
	elapsed := time.Since(start)

	// Should complete in reasonable time (under 50ms)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Many activities detection took %v, expected < 50ms", elapsed)
	}

	// Should produce valid result
	if result == nil {
		t.Error("Should produce result even for many activities")
	}

	t.Logf("Processed %d activities in %v", 1000, elapsed)
}

// TestStateDetector_Performance_ConcurrentDetection tests concurrent detection calls
func TestStateDetector_Performance_ConcurrentDetection(t *testing.T) {
	detector := NewStateDetector()
	content := generateRealisticContent(500)

	// Run concurrent detections
	concurrency := 10
	iterations := 100

	start := time.Now()
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				detector.DetectFromContent(content)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	elapsed := time.Since(start)
	totalOps := concurrency * iterations
	avgTime := elapsed / time.Duration(totalOps)

	t.Logf("Concurrent detection: %d ops in %v (avg %v per op)", totalOps, elapsed, avgTime)

	// Should handle concurrent access without issues
	if avgTime > 20*time.Millisecond {
		t.Errorf("Concurrent detection avg time %v, expected < 20ms", avgTime)
	}
}

// BenchmarkStateDetector_DetectFromContent benchmarks content detection
func BenchmarkStateDetector_DetectFromContent(b *testing.B) {
	detector := NewStateDetector()
	content := generateRealisticContent(500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectFromContent(content)
	}
}

// BenchmarkStateDetector_DetectFromActivities benchmarks activity detection
func BenchmarkStateDetector_DetectFromActivities(b *testing.B) {
	detector := NewStateDetector()
	activities := generateManyActivities(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectFromActivities(activities)
	}
}

// BenchmarkStateDetector_ShouldTransition benchmarks transition logic
func BenchmarkStateDetector_ShouldTransition(b *testing.B) {
	detector := NewStateDetector()
	result := &DetectionResult{
		State:      types.StateExecuting,
		Confidence: 0.85,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.ShouldTransition(types.StateIdle, result, 0.8)
	}
}

// Helper functions

func generateRealisticContent(lines int) string {
	var builder strings.Builder

	templates := []string{
		"User: Can you help me with this task?",
		"Assistant: Of course! Let me analyze the code.",
		"Reading file: /path/to/file.go",
		"Executing command: go test ./...",
		"Running tests...",
		"Test passed: TestExample",
		"Editing file: /path/to/another.go",
		"Writing file: /path/to/output.txt",
		"Error: something went wrong",
		"Task completed successfully",
		"Permission required to proceed [y/n]",
		"Should I continue with this approach?",
		"Analyzing the codebase...",
		"Found 5 potential issues",
		"Applying fix to main.go",
	}

	for i := 0; i < lines; i++ {
		template := templates[i%len(templates)]
		builder.WriteString(fmt.Sprintf("%s\n", template))
	}

	return builder.String()
}

func generateManyActivities(count int) []*events.AgentActivityEvent {
	activities := make([]*events.AgentActivityEvent, count)
	baseTime := time.Now()

	activityTypes := []string{"chat", "file", "tool", "prompt"}
	promptTypes := []string{"permission", "question", ""}

	for i := 0; i < count; i++ {
		activityType := activityTypes[i%len(activityTypes)]
		metadata := make(map[string]string)

		if activityType == "prompt" {
			metadata["prompt_type"] = promptTypes[i%len(promptTypes)]
		} else if activityType == "tool" {
			metadata["tool_name"] = "bash"
		} else if activityType == "file" {
			metadata["operation"] = "read"
		} else if activityType == "chat" {
			metadata["role"] = "user"
		}

		activities[i] = &events.AgentActivityEvent{
			AgentID:      "test-agent",
			ActivityType: activityType,
			Content:      fmt.Sprintf("Activity %d", i),
			Metadata:     metadata,
			Timestamp:    baseTime.Add(-time.Duration(count-i) * time.Second),
		}
	}

	return activities
}
