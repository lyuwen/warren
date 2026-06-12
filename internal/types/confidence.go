package types

// Per-event confidence tiers.
//
// Phase 2 audit Item #22 introduced per-event confidence — a property of the
// regex tier that matched a line, NOT the surrounding session context. The
// four tiers below correspond to the existing pattern groups in
// `internal/parser/activity.go` and `internal/state/detector.go`. Every
// `regexp.MustCompile` call in those files is annotated with its tier so the
// confidence-tier mapping is one-to-one with the regex itself — no drift
// between hand-picked numbers and tier scores.
//
// The values 0.95 / 0.85 / 0.65 / 0.50 are committed (the Critique
// pre-locked them — do NOT change without re-running plan validation).
const (
	// ConfAnchoredExact: the regex pins a unique Claude Code UI element to
	// a start-of-line anchor or to an exact whole-string. False positives
	// are vanishingly rare. Examples:
	//   ^\s*●\s+Bash\(
	//   Esc to cancel · Tab to amend
	ConfAnchoredExact = 0.95

	// ConfAnchoredFuzzy: the regex anchors to a known UI prefix but accepts
	// a free-form tail (path, question text, etc.). The shape is specific
	// but the content can vary. Examples:
	//   ^\s*●\s+Read\s+(.+)
	//   ^\s*●\s+.+\?\s*$
	ConfAnchoredFuzzy = 0.85

	// ConfCaseProse: case-insensitive prose matchers for legacy captures
	// (pre-Claude-Code-v2 UI or hand-written tool descriptions). They match
	// real signals most of the time but can fire on coincidental phrasing.
	// Examples:
	//   (?i)bash\s+tool
	//   (?i)permission required
	ConfCaseProse = 0.65

	// ConfSubstringKey: `strings.Contains` keyword scans. The weakest tier,
	// most likely to false-positive on chat discussion of the keyword.
	// Examples:
	//   strings.Contains(content, "error")
	//   legacy idle words ("waiting for input", "ready for next", …)
	ConfSubstringKey = 0.50
)
