package types

// CanonicalToolNames is the authoritative, lowercase list of every Claude
// Code tool name that Warren must recognize. It is the shared vocabulary
// between `internal/parser` (which emits typed activity events) and
// `internal/state` (which detects StateExecuting from the same lines).
//
// The two packages MUST NOT share regex tables — each owns its own anchoring,
// precedence ladder, and output event shape — but they MUST both produce
// non-empty signals for every entry below. The contract is enforced by
// `internal/types/contract_test.go`.
//
// Adding a new tool: append to this slice first, then update parser's
// `toolExtractors` (level 4 in the precedence ladder) and state's
// `reToolCall` regex. The contract test fails loudly until both sides catch
// up.
//
// Names are lowercase because the parser's `tool_name` event metadata is
// canonical-lowercase (the state detector's `Evidence` string is
// display-only and may carry mixed case).
var CanonicalToolNames = []string{
	"bash",
	"agent",
	"skill",
	"lsp",
	"websearch",
	"webfetch",
	"grep",
	"glob",
	"notebookedit",
}

// CanonicalFileOps is the authoritative list of file-operation names emitted
// by the parser as `activity_type=file` with `operation=<op>` metadata. The
// state detector folds these into StateExecuting; the parser separates them
// from generic tools (`activity_type=tool`) because UI consumers care which
// kind of work the agent is doing.
//
// Names are lowercase for the same reason as CanonicalToolNames.
var CanonicalFileOps = []string{
	"read",
	"edit",
	"write",
}
