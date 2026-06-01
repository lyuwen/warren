package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Phase 2 audit Batch 1 — Item 2: parser refactor verification tests.
//
// These tests target the bug uncovered in the Phase 2 audit: the chat regex
// `^\s*●\s+[^(].+` only excludes a '(' immediately after the space, so lines
// like `● Read /file` or `● Bash(ls)` match BOTH the chat regex AND the
// file/tool regex, producing two events for one line ("duplicate-event bug").
//
// The Critique-recommended fix (Option C) unifies the four parsers into a
// single line-by-line pass with a precedence ladder:
//
//     permission > question > file > tool > chat
//
// Test classes here (per Architect brief + Critique pre-validation):
//
//   (a) Per-line invariant against the 5 real fixtures in internal/testdata/:
//       for every non-empty line, the number of events whose Content equals
//       that line is AT MOST 1.
//
//   (b) Precedence table test: explicit (line -> expected activity type)
//       enumeration covering each rung of the ladder, including the
//       "chat content that mentions tool words" cases that legacy
//       (?i)running\s+tests / (?i)executing\s+command: would have stolen
//       from chat.
//
//   (c) Case-inconsistent switch regression: parseToolUsage uses `(?i)` regex
//       matches but the switch checks `strings.Contains(match, "Bash")` —
//       literal case. Feeding lowercase `bash` legacy text must yield a
//       non-"unknown" tool name once the case-sensitivity bug is fixed.
//
// Tests reference fixtures by relative path so they continue to work in CI
// and worktrees. testdata/ is the canonical Go test fixture directory and
// is never excluded by `go test`.
// ============================================================================

// fixtureFiles enumerates the 5 real tmux captures saved during Phase 2 audit
// investigation. They live at the repo level under internal/testdata/.
var fixtureFiles = []string{
	"asking_permission_foundry2.txt",
	"asking_question_foundry2.txt",
	"idle_completed_foundry2.txt",
	"idle_completed_localhost.txt",
	"running_localhost.txt",
}

// loadFixture reads a fixture from ../testdata relative to the package.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	// internal/parser/ -> internal/testdata/
	path := filepath.Join("..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(data)
}

// -------- (a) Per-line invariant: no duplicate events per source line --------

// TestParser_RealFixtures_NoDuplicatePerLine is the central regression test
// for the audit's duplicate-event bug. For every non-empty line in each of
// the 5 real captures, at most one parsed event may have Content equal to
// that trimmed line.
//
// Why "Content equals trimmed line" and not "source line index": the current
// event model does not carry a source-line field. After Option C unifies
// parsing to a line-by-line pass, every event's Content should be the line
// it came from (or a substring thereof for things like `● Bash(ls)` extracted
// from a longer line). The invariant we can assert today: no two events
// share the same exact Content string when that string corresponds to a real
// fixture line. That is precisely what duplicate-event reproduction means.
//
// Pre-fix: this test FAILS — lines like `● Bash(...)` produce a chat event
// AND a tool event, both with the same Content.
// Post-fix (Option C): this test PASSES.
func TestParser_RealFixtures_NoDuplicatePerLine(t *testing.T) {
	parser := NewActivityParser()

	for _, fname := range fixtureFiles {
		fname := fname
		t.Run(fname, func(t *testing.T) {
			content := loadFixture(t, fname)
			result, err := parser.Parse("test-agent", content)
			if err != nil {
				t.Fatalf("parse %s: %v", fname, err)
			}

			// Index events by their Content string.
			countByContent := make(map[string]int)
			typesByContent := make(map[string][]string)
			for _, ev := range result.Activities {
				countByContent[ev.Content]++
				typesByContent[ev.Content] = append(typesByContent[ev.Content], ev.ActivityType)
			}

			// Walk every non-empty line in the fixture. If any line equals an
			// event Content string and was emitted >1 times, that is the
			// duplicate-event bug recurring.
			for lineNum, raw := range strings.Split(content, "\n") {
				line := strings.TrimSpace(raw)
				if line == "" {
					continue
				}
				if n := countByContent[line]; n > 1 {
					t.Errorf("fixture=%s line=%d %q produced %d events (types=%v); expected <=1 after Option C refactor",
						fname, lineNum+1, line, n, typesByContent[line])
				}
			}
		})
	}
}

// TestParser_RealFixtures_PrefixedTokenLines_NoDoubleEvent is a narrower
// invariant aimed at the exact bug class: any line starting with `●` followed
// by a known tool/file token (Read|Edit|Write|Bash|Agent|Skill|LSP|...) must
// produce at most one event, and that event must NOT be a chat event.
func TestParser_RealFixtures_PrefixedTokenLines_NoDoubleEvent(t *testing.T) {
	parser := NewActivityParser()

	// Tokens that should resolve to file/tool/permission, not chat.
	typedTokens := []string{
		"Read", "Edit", "Write", // file
		"Bash(", "Agent(", "Skill(", "LSP(", "WebSearch(", "WebFetch(", "Grep(", "Glob(", // tool
	}

	for _, fname := range fixtureFiles {
		fname := fname
		t.Run(fname, func(t *testing.T) {
			content := loadFixture(t, fname)
			result, err := parser.Parse("test-agent", content)
			if err != nil {
				t.Fatalf("parse %s: %v", fname, err)
			}

			// Build "events with chat type, keyed by Content" so we can detect
			// chat events that should have been file/tool events.
			chatContents := make(map[string]bool)
			for _, ev := range result.Activities {
				if ev.ActivityType == "chat" {
					chatContents[ev.Content] = true
				}
			}

			for lineNum, raw := range strings.Split(content, "\n") {
				line := strings.TrimSpace(raw)
				if !strings.HasPrefix(line, "●") {
					continue
				}
				// Strip the bullet+spaces to look at the next token.
				rest := strings.TrimSpace(strings.TrimPrefix(line, "●"))
				for _, tok := range typedTokens {
					if strings.HasPrefix(rest, tok) && chatContents[line] {
						t.Errorf("fixture=%s line=%d %q: line starts with `● %s` but parser emitted a CHAT event for it (Option C precedence ladder violated)",
							fname, lineNum+1, line, tok)
						break
					}
				}
			}
		})
	}
}

// -------- (b) Precedence table test --------

// TestParser_PrecedenceTable pins the precedence ladder
// (permission > question > file > tool > chat) with explicit input lines.
// Each row asserts the expected ActivityType and (where relevant) that
// the same input does NOT yield a competing event of the wrong type.
//
// This is the rung-by-rung specification of Option C; the implementer is
// expected to match it exactly.
func TestParser_PrecedenceTable(t *testing.T) {
	parser := NewActivityParser()

	type expect struct {
		wantType   string            // primary expected ActivityType
		wantMeta   map[string]string // required metadata key/value pairs (subset match)
		forbidType string            // an ActivityType that MUST NOT appear for this input
	}

	cases := []struct {
		name  string
		input string
		want  expect
	}{
		// file rung
		{
			name:  "file_read_absolute_path",
			input: "● Read /etc/hosts",
			want: expect{
				wantType:   "file",
				wantMeta:   map[string]string{"operation": "read"},
				forbidType: "chat",
			},
		},
		{
			name:  "file_edit",
			input: "● Edit(/tmp/foo.go)",
			want: expect{
				wantType:   "file",
				wantMeta:   map[string]string{"operation": "edit"},
				forbidType: "chat",
			},
		},
		// tool rung
		{
			name:  "tool_bash",
			input: "● Bash(ls -la)",
			want: expect{
				wantType:   "tool",
				wantMeta:   map[string]string{"tool_name": "bash"},
				forbidType: "chat",
			},
		},
		{
			name:  "tool_grep",
			input: "● Grep(pattern)",
			want: expect{
				wantType:   "tool",
				wantMeta:   map[string]string{"tool_name": "grep"},
				forbidType: "chat",
			},
		},
		// chat rung (assistant)
		{
			name:  "chat_assistant_plain",
			input: "● Hello world",
			want: expect{
				wantType: "chat",
				wantMeta: map[string]string{"role": "assistant"},
			},
		},
		// question rung (overrides chat)
		{
			name:  "question_assistant_ending_qmark",
			input: "● What should I do next?",
			want: expect{
				wantType:   "prompt",
				wantMeta:   map[string]string{"prompt_type": "question"},
				forbidType: "chat",
			},
		},
		// chat rung (user)
		{
			name:  "chat_user_input",
			input: "❯ run the tests",
			want: expect{
				wantType: "chat",
				wantMeta: map[string]string{"role": "user"},
			},
		},
		// permission rung (overrides everything else)
		{
			name:  "permission_choice_option",
			input: "❯ 2. Yes, and don't ask again",
			want: expect{
				wantType:   "prompt",
				wantMeta:   map[string]string{"prompt_type": "permission"},
				forbidType: "chat",
			},
		},
		{
			name:  "permission_allowed_by_auto_mode",
			input: "● Allowed by auto mode for Read",
			want: expect{
				wantType:   "prompt",
				wantMeta:   map[string]string{"prompt_type": "permission"},
				forbidType: "chat",
			},
		},
		{
			name:  "permission_escape_footer",
			input: " Esc to cancel · Tab to amend",
			want: expect{
				wantType: "prompt",
				wantMeta: map[string]string{"prompt_type": "permission"},
			},
		},
		// Over-broad legacy tool regexes must NOT steal chat content
		{
			name:  "chat_mentions_running_tests_must_not_become_tool",
			input: "● I'm running tests now",
			want: expect{
				wantType:   "chat",
				forbidType: "tool",
			},
		},
		{
			name:  "chat_mentions_executing_command_must_not_become_tool",
			input: "● Executing command: foo is what I'd usually do",
			want: expect{
				wantType:   "chat",
				forbidType: "tool",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.Parse("test-agent", tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			// Find the event for this line.
			var primary *eventSummary
			var allTypes []string
			for _, ev := range result.Activities {
				allTypes = append(allTypes, ev.ActivityType)
				if primary == nil && ev.ActivityType == tc.want.wantType {
					summary := summarize(ev.ActivityType, ev.Metadata)
					primary = &summary
				}
			}

			if primary == nil {
				t.Fatalf("input=%q: expected an event with ActivityType=%q, got types=%v",
					tc.input, tc.want.wantType, allTypes)
			}

			// Check forbidden type.
			if tc.want.forbidType != "" {
				for _, ty := range allTypes {
					if ty == tc.want.forbidType {
						t.Errorf("input=%q: forbidden ActivityType %q also emitted (types=%v) — precedence ladder violated",
							tc.input, tc.want.forbidType, allTypes)
						break
					}
				}
			}

			// Check required metadata subset.
			for k, v := range tc.want.wantMeta {
				got, ok := primary.metadata[k]
				if !ok {
					t.Errorf("input=%q: missing metadata key %q (got %v)", tc.input, k, primary.metadata)
					continue
				}
				if got != v {
					t.Errorf("input=%q: metadata[%q] = %q, want %q", tc.input, k, got, v)
				}
			}
		})
	}
}

type eventSummary struct {
	activityType string
	metadata     map[string]string
}

func summarize(at string, md map[string]string) eventSummary {
	cp := make(map[string]string, len(md))
	for k, v := range md {
		cp[k] = v
	}
	return eventSummary{activityType: at, metadata: cp}
}

// -------- (c) Case-inconsistent switch regression --------

// TestParser_ToolUsage_CaseInsensitive_LegacyBash exercises the case-
// inconsistency the Critique flagged: parseToolUsage matches with `(?i)Bash`
// but the switch uses `strings.Contains(match, "Bash")` — literal case. A
// lowercase legacy capture like `bash tool` therefore resolves to
// tool_name="unknown" today. After the fix it must be "bash".
//
// Pre-fix: FAILS (tool_name == "unknown").
// Post-fix: PASSES (tool_name == "bash").
func TestParser_ToolUsage_CaseInsensitive_LegacyBash(t *testing.T) {
	parser := NewActivityParser()

	content := "the agent used bash tool to run ls"
	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var got string
	for _, ev := range result.Activities {
		if ev.ActivityType == "tool" {
			got = ev.Metadata["tool_name"]
			break
		}
	}
	if got == "" {
		t.Fatalf("expected a tool event, got none (activities=%d)", len(result.Activities))
	}
	if got == "unknown" {
		t.Errorf("tool_name=%q for legacy lowercase 'bash tool' — case-inconsistent switch bug still present", got)
	}
	if got != "bash" {
		t.Errorf("tool_name=%q, want %q", got, "bash")
	}
}

// TestParser_ToolUsage_CaseInsensitive_LegacyLSP analogous regression for LSP.
// LSP already worked because the switch uses lowercase matchLower for "lsp",
// but pinning it prevents regression if someone "fixes" that branch back.
func TestParser_ToolUsage_CaseInsensitive_LegacyLSP(t *testing.T) {
	parser := NewActivityParser()

	content := "the agent used LSP tool to find references"
	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var got string
	for _, ev := range result.Activities {
		if ev.ActivityType == "tool" {
			got = ev.Metadata["tool_name"]
			break
		}
	}
	if got != "lsp" {
		t.Errorf("tool_name=%q, want %q", got, "lsp")
	}
}

// -------- (d) Fixture-level event-count smoke --------

// TestParser_RealFixtures_EmitSomeEvents is a sanity smoke test: each fixture
// represents a recognisable Claude Code UI state, so the parser must emit
// at least one event per fixture. Catches regressions that would silently
// disable parsing entirely.
func TestParser_RealFixtures_EmitSomeEvents(t *testing.T) {
	parser := NewActivityParser()

	for _, fname := range fixtureFiles {
		fname := fname
		t.Run(fname, func(t *testing.T) {
			content := loadFixture(t, fname)
			result, err := parser.Parse("test-agent", content)
			if err != nil {
				t.Fatalf("parse %s: %v", fname, err)
			}
			if len(result.Activities) == 0 {
				t.Errorf("fixture %s produced zero activities — parser likely disabled", fname)
			}
		})
	}
}

// TestParser_RealFixtures_PermissionFixtureEmitsPermission asserts that the
// asking_permission_foundry2.txt fixture, which contains the unmistakable
// "❯ 1. Yes" / "❯ 2. ..." choice block AND the "Esc to cancel · Tab to amend"
// footer, produces at least one permission prompt event.
func TestParser_RealFixtures_PermissionFixtureEmitsPermission(t *testing.T) {
	parser := NewActivityParser()
	content := loadFixture(t, "asking_permission_foundry2.txt")

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	found := false
	for _, ev := range result.Activities {
		if ev.ActivityType == "prompt" && ev.Metadata["prompt_type"] == "permission" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("asking_permission fixture did not produce a permission event")
	}
}

// TestParser_RealFixtures_QuestionFixtureEmitsQuestion asserts that the
// asking_question_foundry2.txt fixture, whose final assistant line is
// "● What are you working on today?", produces at least one question event.
func TestParser_RealFixtures_QuestionFixtureEmitsQuestion(t *testing.T) {
	parser := NewActivityParser()
	content := loadFixture(t, "asking_question_foundry2.txt")

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	found := false
	for _, ev := range result.Activities {
		if ev.ActivityType == "prompt" && ev.Metadata["prompt_type"] == "question" {
			found = true
			break
		}
	}
	if !found {
		// Acceptable failure mode: question may currently be misclassified as
		// chat. Surface this so the implementer addresses it.
		var allTypes []string
		for _, ev := range result.Activities {
			allTypes = append(allTypes, ev.ActivityType+":"+ev.Metadata["prompt_type"])
		}
		t.Errorf("asking_question fixture did not produce a question event (types=%v)", allTypes)
	}
}
