package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lfu/warren/internal/events"
)

// ActivityParser parses captured pane content into structured activity events.
//
// Design (Phase 2 audit, Batch 1, Item 2 — Option C):
//
// Earlier the parser ran four independent passes (parseChat,
// parseFileInteractions, parseToolUsage, parsePrompts) over the same input.
// Because the chat regex `●\s+[^(].+` matched any "● ..." line — including
// `● Read /file`, `● Bash(...)`, `● Edit(...)` — the same line could be
// reported as both a chat event AND a typed (file/tool) event. That double-
// emission, plus several legacy over-broad regexes (`(?i)running\s+tests`,
// `(?i)executing\s+command:`, `(?i)Read\s+tool.*?file_path`), produced
// duplicate and false-positive activity events.
//
// We do not have RE2 lookaround in Go, so a regex-only fix is impossible.
// Instead, Parse() runs a single line-by-line dispatcher with a precedence
// ladder. Each line produces AT MOST ONE event:
//
//  1. Permission — most specific anchors first
//     ("Esc to cancel · Tab to amend", "Allowed by auto mode", "❯ N. Option")
//  2. Question   — `●` + line ending in `?` (within the last 10 non-empty
//     lines only, preserving the existing tail-only behaviour)
//  3. File       — `● Read|Edit|Write` + path
//  4. Tool       — `● Bash|Agent|Skill|LSP|WebSearch|WebFetch|Grep|Glob`
//  5. Chat       — `●` or `❯` fallback (everything else with those prefixes)
//
// Degradation contract: a brand-new tool name (say `● Task(...)`) will not
// match level 4, so it falls through to level 5 and is emitted as a chat
// event with role=assistant. That is correct — Warren still records the
// line, and adding the new tool to level 4 later promotes it to a tool
// event without further changes. No silent data loss.
//
// Out of scope for this batch: unification with `internal/state/` patterns
// (tracked separately as Batch 3 #19).
type ActivityParser struct {
	// Level 1: permission anchors. Most specific first.
	permissionAnchors []*regexp.Regexp

	// Level 2: question detection patterns. Applied only to the last 10
	// non-empty lines.
	questionLineAnchors []*regexp.Regexp

	// Level 3: file operation extractors. Each carries an operation tag
	// ("read"/"edit"/"write") because the regex alone cannot disambiguate
	// `● Read N files` from `● Edit(p)` cheaply.
	fileExtractors []fileExtractor

	// Level 4: tool extractors. Each carries the canonical tool name so we
	// avoid the case-inconsistent switch the previous implementation used
	// (`strings.Contains(match, "Bash")` paired with a `(?i)` regex match).
	toolExtractors []toolExtractor

	// Level 5: chat fallback prefixes (Claude Code UI `●` / `❯`, plus
	// legacy `user:` / `assistant:` / `claude:`).
	chatPrefixCC     *regexp.Regexp // ● ...
	chatPrefixUser   *regexp.Regexp // ❯ ...
	chatLegacyUser   *regexp.Regexp // ^user:
	chatLegacyAssist *regexp.Regexp // ^(assistant|claude):

	// Permission content scan (whole-content, NOT per-line) for legacy
	// "permission required" / "[y/n]" prose that does not start with `●`.
	permissionContentAnchors []*regexp.Regexp

	// Block-level question shapes (whole-content): the multiple-choice
	// detector ("1. ..." / "2. ..." within the trailing 10 non-empty lines)
	// and the natural-language fallbacks ("Would you like...?", etc.).
	multipleChoiceLine *regexp.Regexp
	naturalQuestion    []*regexp.Regexp
}

type fileExtractor struct {
	re        *regexp.Regexp
	operation string // "read" / "edit" / "write"
}

type toolExtractor struct {
	re   *regexp.Regexp
	name string // "bash" / "agent" / "skill" / "lsp" / "websearch" / "webfetch" / "grep" / "glob"
}

// NewActivityParser creates a new activity parser
func NewActivityParser() *ActivityParser {
	return &ActivityParser{
		permissionAnchors: []*regexp.Regexp{
			// Real Claude Code UI: "Esc to cancel · Tab to amend" footer
			regexp.MustCompile(`Esc to cancel\s*·?\s*Tab to amend`),
			// Real Claude Code UI: choice selector "❯ N. Option"
			regexp.MustCompile(`^\s*❯\s+\d+\.\s+`),
			// Real Claude Code UI: "Allowed by auto mode"
			regexp.MustCompile(`(?i)Allowed by auto mode`),
		},
		questionLineAnchors: []*regexp.Regexp{
			// Real Claude Code UI: assistant question — `● ... ?` ending in '?'.
			regexp.MustCompile(`^\s*●\s+.+\?\s*$`),
			// Legacy AskUserQuestion tool surface.
			regexp.MustCompile(`(?i)AskUserQuestion`),
			// Legacy question shapes (single-line).
			regexp.MustCompile(`^What would you like .*\?$`),
			regexp.MustCompile(`^Should I .*\?$`),
			regexp.MustCompile(`^Would you like .*\?$`),
			regexp.MustCompile(`^Do you want .*\?$`),
			regexp.MustCompile(`^How should I .*\?$`),
			regexp.MustCompile(`^Which .*would you prefer\?$`),
		},
		fileExtractors: []fileExtractor{
			// `● Read <path-or-summary>`. The argument is free-form because
			// Claude Code emits both `● Read /foo/bar.go` and
			// `● Read 3 files`; the captured group is whatever follows.
			{re: regexp.MustCompile(`^\s*●\s+Read\s+(.+)`), operation: "read"},
			{re: regexp.MustCompile(`^\s*●\s+Edit\((.+?)\)`), operation: "edit"},
			{re: regexp.MustCompile(`^\s*●\s+Write\((.+?)\)`), operation: "write"},
			// Legacy "reading file: <path>" prose lines (kept for backward
			// compatibility with older captures).
			{re: regexp.MustCompile(`(?i)^reading\s+file:\s+(.+)$`), operation: "read"},
			{re: regexp.MustCompile(`(?i)^editing\s+file:\s+(.+)$`), operation: "edit"},
			{re: regexp.MustCompile(`(?i)^writing\s+file:\s+(.+)$`), operation: "write"},
		},
		toolExtractors: []toolExtractor{
			{re: regexp.MustCompile(`^\s*●\s+Bash\(`), name: "bash"},
			{re: regexp.MustCompile(`^\s*●\s+Agent\(`), name: "agent"},
			{re: regexp.MustCompile(`^\s*●\s+Skill\(`), name: "skill"},
			{re: regexp.MustCompile(`^\s*●\s+LSP\(`), name: "lsp"},
			{re: regexp.MustCompile(`^\s*●\s+WebSearch\(`), name: "websearch"},
			{re: regexp.MustCompile(`^\s*●\s+WebFetch\(`), name: "webfetch"},
			{re: regexp.MustCompile(`^\s*●\s+Grep\(`), name: "grep"},
			{re: regexp.MustCompile(`^\s*●\s+Glob\(`), name: "glob"},
			// Legacy "Bash tool / LSP tool / WebSearch tool" prose. Kept
			// case-insensitive and substring-matching to preserve backward
			// compatibility with older captures (audit specifically left
			// these in scope and only flagged `(?i)running\s+tests` and
			// `(?i)executing\s+command:` as over-broad).
			{re: regexp.MustCompile(`(?i)bash\s+tool`), name: "bash"},
			{re: regexp.MustCompile(`(?i)lsp\s+tool`), name: "lsp"},
			{re: regexp.MustCompile(`(?i)websearch\s+tool`), name: "websearch"},
			// Legacy "executing command:" prose anchored to start-of-line.
			// The unanchored `(?i)executing\s+command:` it replaces matched
			// inside chat lines (audit #2: duplicate emission).
			{re: regexp.MustCompile(`(?i)^executing\s+command:`), name: "bash"},
			// Note: the unanchored `(?i)running\s+tests` from the legacy
			// patterns is deliberately dropped — it fired on plain chat like
			// "I'm running tests now" and produced spurious tool events.
		},
		chatPrefixCC:     regexp.MustCompile(`^\s*●\s+.+`),
		chatPrefixUser:   regexp.MustCompile(`^\s*❯\s+.+`),
		chatLegacyUser:   regexp.MustCompile(`(?i)^user:`),
		chatLegacyAssist: regexp.MustCompile(`(?i)^(assistant|claude):`),
		permissionContentAnchors: []*regexp.Regexp{
			regexp.MustCompile(`(?i)permission\s+required`),
			regexp.MustCompile(`(?i)approve\s+or\s+deny`),
			regexp.MustCompile(`(?i)waiting\s+for\s+approval`),
			regexp.MustCompile(`(?i)\[y/n\]`),
			regexp.MustCompile(`(?i)allow\s+this\s+action`),
		},
		multipleChoiceLine: regexp.MustCompile(`^\d+\.\s+.+$`),
		naturalQuestion: []*regexp.Regexp{
			regexp.MustCompile(`^What would you like .*\?$`),
			regexp.MustCompile(`^Should I .*\?$`),
			regexp.MustCompile(`^Would you like .*\?$`),
			regexp.MustCompile(`^Do you want .*\?$`),
			regexp.MustCompile(`^How should I .*\?$`),
			regexp.MustCompile(`^Which .*would you prefer\?$`),
		},
	}
}

// ParseResult contains the results of parsing
type ParseResult struct {
	Activities    []*events.AgentActivityEvent
	Confidence    float64
	DetectedTypes []string
}

// Parse analyzes captured content and extracts activity events.
//
// Each non-empty line is classified by a single-dispatch precedence ladder
// (see the ActivityParser doc comment) and produces at most one event. A
// final whole-content sweep adds block-level prompts (permission prose,
// multiple-choice questions, natural-language questions) that span multiple
// lines or do not anchor to a `●`/`❯` prefix.
func (p *ActivityParser) Parse(agentID string, content string) (*ParseResult, error) {
	result := &ParseResult{
		Activities:    []*events.AgentActivityEvent{},
		DetectedTypes: []string{},
	}

	lines := strings.Split(content, "\n")
	timestamp := time.Now()

	// Build the "is this line in the last 10 non-empty lines" set for the
	// question detector. Real Claude Code questions live at the bottom of
	// the pane; matching anywhere produces false positives on transcript
	// history.
	tailQuestionLines := lastNonEmptyLineSet(lines, 10)

	seenTypes := map[string]bool{}

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		activity := p.classifyLine(agentID, line, i, tailQuestionLines, timestamp)
		if activity == nil {
			continue
		}

		result.Activities = append(result.Activities, activity)
		t := activityTopType(activity)
		if !seenTypes[t] {
			seenTypes[t] = true
			result.DetectedTypes = append(result.DetectedTypes, t)
		}
	}

	// Block-level sweeps (whole-content, NOT per-line). These run AFTER
	// the per-line dispatch so multi-line shapes can fire even if every
	// individual line was already classified above.

	if extra := p.scanPermissionContent(agentID, content, timestamp); extra != nil {
		result.Activities = append(result.Activities, extra)
		if !seenTypes["prompt"] {
			seenTypes["prompt"] = true
			result.DetectedTypes = append(result.DetectedTypes, "prompt")
		}
	}

	if extra := p.scanMultipleChoice(agentID, lines, timestamp); extra != nil {
		result.Activities = append(result.Activities, extra)
		if !seenTypes["prompt"] {
			seenTypes["prompt"] = true
			result.DetectedTypes = append(result.DetectedTypes, "prompt")
		}
	} else if extras := p.scanNaturalQuestions(agentID, lines, timestamp); len(extras) > 0 {
		// Only fire the natural-language fallback when there is no
		// multiple-choice question — otherwise we'd double-emit.
		for _, e := range extras {
			result.Activities = append(result.Activities, e)
		}
		if !seenTypes["prompt"] {
			seenTypes["prompt"] = true
			result.DetectedTypes = append(result.DetectedTypes, "prompt")
		}
	}

	// Confidence: same shape as the previous implementation so existing
	// thresholds keep meaning what they meant before.
	if len(result.Activities) > 0 {
		result.Confidence = 0.8
		if len(result.DetectedTypes) > 2 {
			result.Confidence = 0.95
		}
	}

	return result, nil
}

// classifyLine runs the precedence ladder over a single trimmed line.
// Returns nil when the line matches no level.
func (p *ActivityParser) classifyLine(agentID, line string, _ int, tailQuestionLines map[string]bool, ts time.Time) *events.AgentActivityEvent {
	// Level 1: Permission anchors (most specific).
	for _, re := range p.permissionAnchors {
		if re.MatchString(line) {
			return &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "prompt",
				Content:      line,
				Metadata:     map[string]string{"prompt_type": "permission"},
				Timestamp:    ts,
			}
		}
	}

	// Level 2: Question — assistant `● ... ?` line in the tail window.
	if tailQuestionLines[line] {
		for _, re := range p.questionLineAnchors {
			if re.MatchString(line) {
				return &events.AgentActivityEvent{
					AgentID:      agentID,
					ActivityType: "prompt",
					Content:      line,
					Metadata:     map[string]string{"prompt_type": "question"},
					Timestamp:    ts,
				}
			}
		}
	}

	// Level 3: File operations.
	for _, fe := range p.fileExtractors {
		if m := fe.re.FindStringSubmatch(line); m != nil {
			filePath := ""
			if len(m) > 1 {
				filePath = strings.TrimSpace(m[1])
			}
			return &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "file",
				Content:      line,
				Metadata: map[string]string{
					"operation": fe.operation,
					"file_path": filePath,
				},
				Timestamp: ts,
			}
		}
	}

	// Level 4: Tool usage. Tool name comes from the extractor table, NOT a
	// case-sensitive `strings.Contains` switch (audit fix: the previous
	// switch used literal "Bash" against a `(?i)` regex match and silently
	// misclassified legacy `bash tool` lines as "unknown").
	for _, te := range p.toolExtractors {
		if te.re.MatchString(line) {
			return &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "tool",
				Content:      line,
				Metadata:     map[string]string{"tool_name": te.name},
				Timestamp:    ts,
			}
		}
	}

	// Level 5: Chat fallback.
	switch {
	case p.chatPrefixCC.MatchString(line):
		return chatEvent(agentID, line, "assistant", ts)
	case p.chatPrefixUser.MatchString(line):
		return chatEvent(agentID, line, "user", ts)
	case p.chatLegacyUser.MatchString(line):
		return chatEvent(agentID, line, "user", ts)
	case p.chatLegacyAssist.MatchString(line):
		return chatEvent(agentID, line, "assistant", ts)
	}

	return nil
}

func chatEvent(agentID, line, role string, ts time.Time) *events.AgentActivityEvent {
	return &events.AgentActivityEvent{
		AgentID:      agentID,
		ActivityType: "chat",
		Content:      line,
		Metadata:     map[string]string{"role": role},
		Timestamp:    ts,
	}
}

// activityTopType maps an activity to the top-level "detected type" string
// used in ParseResult.DetectedTypes. Tools/files/chat/prompt are surfaced
// distinctly even though the per-line dispatcher classifies each event.
func activityTopType(a *events.AgentActivityEvent) string {
	return a.ActivityType
}

// lastNonEmptyLineSet returns a set of trimmed lines that are among the
// last n non-empty lines of the input. Used by the question detector.
func lastNonEmptyLineSet(lines []string, n int) map[string]bool {
	set := map[string]bool{}
	count := 0
	for i := len(lines) - 1; i >= 0 && count < n; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		set[l] = true
		count++
	}
	return set
}

// scanPermissionContent looks for legacy "permission required" style prose
// that doesn't anchor to a `●` line. Emits at most one event.
func (p *ActivityParser) scanPermissionContent(agentID, content string, ts time.Time) *events.AgentActivityEvent {
	for _, re := range p.permissionContentAnchors {
		if re.MatchString(content) {
			return &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "prompt",
				Content:      re.FindString(content),
				Metadata:     map[string]string{"prompt_type": "permission"},
				Timestamp:    ts,
			}
		}
	}
	return nil
}

// scanMultipleChoice detects a numbered-list multiple-choice question in
// the last 10 non-empty lines. Returns a single prompt event listing all
// detected options.
func (p *ActivityParser) scanMultipleChoice(agentID string, lines []string, ts time.Time) *events.AgentActivityEvent {
	last := lastNonEmptyLines(lines, 10)
	options := []string{}
	for _, line := range last {
		if p.multipleChoiceLine.MatchString(line) {
			options = append(options, line)
		}
	}
	if len(options) < 2 {
		return nil
	}
	return &events.AgentActivityEvent{
		AgentID:      agentID,
		ActivityType: "prompt",
		Content:      strings.Join(options, "\n"),
		Metadata: map[string]string{
			"prompt_type":   "question",
			"question_type": "multiple_choice",
			"option_count":  fmt.Sprintf("%d", len(options)),
		},
		Timestamp: ts,
	}
}

// scanNaturalQuestions matches plain-text "Would you like ...?" shapes in
// the last 10 non-empty lines. Skips comment-ish lines (#, //, list
// bullets) to keep markdown chatter out of the result.
func (p *ActivityParser) scanNaturalQuestions(agentID string, lines []string, ts time.Time) []*events.AgentActivityEvent {
	var out []*events.AgentActivityEvent
	last := lastNonEmptyLines(lines, 10)
	for _, line := range last {
		if strings.HasPrefix(line, "//") ||
			strings.HasPrefix(line, "#") ||
			strings.Contains(line, "```") ||
			strings.HasPrefix(line, "*") ||
			strings.HasPrefix(line, "-") {
			continue
		}
		for _, re := range p.naturalQuestion {
			if re.MatchString(line) {
				out = append(out, &events.AgentActivityEvent{
					AgentID:      agentID,
					ActivityType: "prompt",
					Content:      line,
					Metadata:     map[string]string{"prompt_type": "question"},
					Timestamp:    ts,
				})
				break
			}
		}
	}
	return out
}

// lastNonEmptyLines returns the last n non-empty trimmed lines, in original
// order.
func lastNonEmptyLines(lines []string, n int) []string {
	out := []string{}
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		out = append([]string{l}, out...)
	}
	return out
}

// ExtractRecentChat extracts the most recent chat messages.
//
// Mirrors the per-line chat classifier in classifyLine so the "most recent
// chat" view stays consistent with what the dispatcher emits as chat
// events. Legacy "user:" / "assistant:" / "claude:" prefixes still count.
func (p *ActivityParser) ExtractRecentChat(content string, maxMessages int) []string {
	lines := strings.Split(content, "\n")
	messages := []string{}

	isChat := func(line string) bool {
		return p.chatPrefixCC.MatchString(line) ||
			p.chatPrefixUser.MatchString(line) ||
			p.chatLegacyUser.MatchString(line) ||
			p.chatLegacyAssist.MatchString(line)
	}

	for i := len(lines) - 1; i >= 0 && len(messages) < maxMessages; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if isChat(line) {
			messages = append([]string{line}, messages...)
		}
	}
	return messages
}
