package types_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lfu/warren/internal/parser"
	"github.com/lfu/warren/internal/state"
	"github.com/lfu/warren/internal/types"
)

// ============================================================================
// Phase 2 audit Batch 3a — Item #19: parser/state pattern contract.
//
// The Critique pre-locked Decision #19-A: contract test, NOT shared regex
// tables. The shared vocabulary lives in `internal/types/tools.go`
// (`CanonicalToolNames` + `CanonicalFileOps`). Both `internal/parser` and
// `internal/state` MUST recognize every entry — but they own their own
// pattern tables, output shapes, and precedence concerns.
//
// This contract pins the recognition coverage so adding a new tool to
// `types.CanonicalToolNames` will fail loudly here until both packages
// catch up. Reverse direction is also covered: removing `notebookedit`
// from parser's `toolExtractors` (the concrete Batch 3a divergence fix)
// would break TestParser_NotebookEdit_NowRecognized.
//
// Capitalization contract (Critique §19-B):
//   - parser is case-SENSITIVE on the canonical lowercase tool_name
//   - state's Evidence string is display-only (case-insensitive match)
// ============================================================================

// canonicalizeTool returns the CapitalCase form Claude Code emits for the
// given canonical lowercase name. The mapping mirrors the visible UI labels
// (e.g. `bash` → `Bash`, `websearch` → `WebSearch`, `notebookedit` →
// `NotebookEdit`). Hand-coded because Go has no built-in title-cased
// translation that handles `webfetch` → `WebFetch`.
func canonicalizeTool(t *testing.T, lower string) string {
	t.Helper()
	switch lower {
	case "bash":
		return "Bash"
	case "agent":
		return "Agent"
	case "skill":
		return "Skill"
	case "lsp":
		return "LSP"
	case "websearch":
		return "WebSearch"
	case "webfetch":
		return "WebFetch"
	case "grep":
		return "Grep"
	case "glob":
		return "Glob"
	case "notebookedit":
		return "NotebookEdit"
	}
	t.Fatalf("canonicalizeTool: no UI-label mapping for tool %q — add it here when extending CanonicalToolNames", lower)
	return ""
}

// canonicalizeFileOp mirrors canonicalizeTool for the three file ops.
func canonicalizeFileOp(t *testing.T, lower string) string {
	t.Helper()
	switch lower {
	case "read":
		return "Read"
	case "edit":
		return "Edit"
	case "write":
		return "Write"
	}
	t.Fatalf("canonicalizeFileOp: no UI-label mapping for op %q", lower)
	return ""
}

// canonicalLineForTool returns the `● <Tool>(arg)` shape the parser
// expects (level-4 anchored-exact tier) for the given canonical tool.
func canonicalLineForTool(t *testing.T, tool string) string {
	t.Helper()
	return fmt.Sprintf("● %s(arg)", canonicalizeTool(t, tool))
}

// canonicalLineForFileOp returns the parser's canonical line for op:
//   - read: `● Read <tail>` (parser's freeform-tail level-3 shape)
//   - edit: `● Edit(/path)`
//   - write: `● Write(/path)`
//
// State's `reToolCall` requires parens for ALL ops, so the parser-side
// fixtures here differ from the state-side fixtures (read uses parens for
// state). This divergence is intentional and permitted by the contract
// per Critique §19: parser owns the `Read <tail>` shape because Claude
// Code emits both `● Read /file` and `● Read 3 files`; state cares only
// about the executing signal and matches the paren-anchored form.
func canonicalLineForFileOp(t *testing.T, op string) string {
	t.Helper()
	switch op {
	case "read":
		return "● Read /tmp/foo.go"
	case "edit":
		return "● Edit(/tmp/foo.go)"
	case "write":
		return "● Write(/tmp/foo.go)"
	}
	t.Fatalf("canonicalLineForFileOp: unknown op %q", op)
	return ""
}

// stateLineForFileOp returns the state-detector canonical line for op,
// using the paren-anchored shape that `reToolCall` requires.
func stateLineForFileOp(t *testing.T, op string) string {
	t.Helper()
	switch op {
	case "read":
		return "● Read(/tmp/foo.go)"
	case "edit":
		return "● Edit(/tmp/foo.go)"
	case "write":
		return "● Write(/tmp/foo.go)"
	}
	t.Fatalf("stateLineForFileOp: unknown op %q", op)
	return ""
}

// ----------------------------------------------------------------------------
// TestCanonicalToolNames_ParserCoverage — every name in CanonicalToolNames
// produces exactly one parser event with `activity_type=tool` and a
// case-sensitive lowercase `tool_name` matching the canonical entry.
// ----------------------------------------------------------------------------

func TestCanonicalToolNames_ParserCoverage(t *testing.T) {
	p := parser.NewActivityParser()

	for _, tool := range types.CanonicalToolNames {
		t.Run(tool, func(t *testing.T) {
			line := canonicalLineForTool(t, tool)
			result, err := p.Parse("agent-1", line)
			if err != nil {
				t.Fatalf("parser.Parse(%q) returned error: %v", line, err)
			}
			// Filter to tool-type activities so a stray prompt/chat
			// emission would not silently mask a missing extractor.
			var toolEvents []string
			for _, a := range result.Activities {
				if a.ActivityType == "tool" {
					toolEvents = append(toolEvents, a.Metadata["tool_name"])
				}
			}
			if len(toolEvents) != 1 {
				t.Fatalf("parser.Parse(%q): expected exactly 1 activity_type=tool event, got %d (tool_names=%v); full activities=%+v", line, len(toolEvents), toolEvents, result.Activities)
			}
			if toolEvents[0] != tool {
				t.Fatalf("parser.Parse(%q): tool_name = %q, want %q (case-sensitive lowercase per Critique §19-B)", line, toolEvents[0], tool)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestCanonicalToolNames_StateCoverage — every canonical tool produces a
// StateExecuting signal whose Evidence mentions the tool name. State's
// Evidence is case-insensitive per Critique §19-B (it's a display string).
// ----------------------------------------------------------------------------

func TestCanonicalToolNames_StateCoverage(t *testing.T) {
	d := state.NewStateDetector()

	for _, tool := range types.CanonicalToolNames {
		t.Run(tool, func(t *testing.T) {
			line := canonicalLineForTool(t, tool)
			// State's reToolCall only fires for lines in the bottom 10
			// of the capture. A single line guarantees that.
			result := d.DetectFromContent(line)
			if result.State != types.StateExecuting {
				t.Fatalf("state.DetectFromContent(%q): result.State = %q, want %q", line, result.State, types.StateExecuting)
			}
			// Evidence collection is in result.Signals (formatted strings).
			joined := strings.ToLower(strings.Join(result.Signals, "\n"))
			if !strings.Contains(joined, strings.ToLower(tool)) {
				t.Fatalf("state.DetectFromContent(%q): no Signal evidence mentions tool %q (case-insensitive); signals=%v", line, tool, result.Signals)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestCanonicalFileOps_ParserCoverage — every name in CanonicalFileOps
// produces one parser event with `activity_type=file` and the correct
// `operation` metadata.
// ----------------------------------------------------------------------------

func TestCanonicalFileOps_ParserCoverage(t *testing.T) {
	p := parser.NewActivityParser()

	for _, op := range types.CanonicalFileOps {
		t.Run(op, func(t *testing.T) {
			line := canonicalLineForFileOp(t, op)
			result, err := p.Parse("agent-1", line)
			if err != nil {
				t.Fatalf("parser.Parse(%q) returned error: %v", line, err)
			}
			var fileEvents []string
			for _, a := range result.Activities {
				if a.ActivityType == "file" {
					fileEvents = append(fileEvents, a.Metadata["operation"])
				}
			}
			if len(fileEvents) != 1 {
				t.Fatalf("parser.Parse(%q): expected exactly 1 activity_type=file event, got %d (operations=%v); full activities=%+v", line, len(fileEvents), fileEvents, result.Activities)
			}
			if fileEvents[0] != op {
				t.Fatalf("parser.Parse(%q): operation = %q, want %q (case-sensitive lowercase)", line, fileEvents[0], op)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestCanonicalFileOps_StateCoverage — file ops also map to StateExecuting
// in the state detector (`reToolCall` covers Read/Edit/Write alongside the
// non-file tools — historical conflation that the contract permits per
// Critique §19, since state cares only about the executing signal).
// ----------------------------------------------------------------------------

func TestCanonicalFileOps_StateCoverage(t *testing.T) {
	d := state.NewStateDetector()

	for _, op := range types.CanonicalFileOps {
		t.Run(op, func(t *testing.T) {
			line := stateLineForFileOp(t, op)
			result := d.DetectFromContent(line)
			if result.State != types.StateExecuting {
				t.Fatalf("state.DetectFromContent(%q): result.State = %q, want %q", line, result.State, types.StateExecuting)
			}
			joined := strings.ToLower(strings.Join(result.Signals, "\n"))
			if !strings.Contains(joined, strings.ToLower(op)) {
				t.Fatalf("state.DetectFromContent(%q): no Signal evidence mentions op %q (case-insensitive); signals=%v", line, op, result.Signals)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// TestParser_NotebookEdit_NowRecognized — the concrete pre-Batch-3a
// divergence. Before this fix, `● NotebookEdit(/tmp/foo.ipynb)` fell
// through parser's level-4 toolExtractors to level-5 (chat fallback) and
// produced `activity_type=chat` instead of `activity_type=tool`. State
// already recognized NotebookEdit via reToolCall; parser did not.
// ----------------------------------------------------------------------------

func TestParser_NotebookEdit_NowRecognized(t *testing.T) {
	p := parser.NewActivityParser()
	line := "● NotebookEdit(/tmp/foo.ipynb)"
	result, err := p.Parse("agent-1", line)
	if err != nil {
		t.Fatalf("parser.Parse(%q): %v", line, err)
	}
	// Pre-fix: this line produced exactly one chat event. Post-fix: it
	// must produce a tool event with tool_name=notebookedit and MUST NOT
	// produce a chat fallback.
	var (
		toolNames []string
		chatCount int
	)
	for _, a := range result.Activities {
		switch a.ActivityType {
		case "tool":
			toolNames = append(toolNames, a.Metadata["tool_name"])
		case "chat":
			chatCount++
		}
	}
	if len(toolNames) != 1 || toolNames[0] != "notebookedit" {
		t.Fatalf("NotebookEdit regression: expected 1 tool event with tool_name=notebookedit, got tools=%v", toolNames)
	}
	if chatCount != 0 {
		t.Fatalf("NotebookEdit regression: line should not fall through to chat fallback; got chatCount=%d, activities=%+v", chatCount, result.Activities)
	}
}
