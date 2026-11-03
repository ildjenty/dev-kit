// Package engine advances the state machine, computes instructions, accepts
// results, runs verification and judgment commands, and writes briefs.
// Git operations belong to workspace, external writes to external, and record
// storage to ledger/fsstore. Engine never launches subagents.
//
// New accepts checkout, work, storage, configuration, and kit values from cmd/devkit.
// Start, Next, Submit, Plan,
// WithdrawPlan, Answer, External, Note, Pause, Resume, Abort, and Status expose
// the runtime operations. RecoverExternal only queries interrupted writes.
// Callers supply JSON bytes for submitted files and agent interpretations for
// observed human answers. Result points to generated briefs and target paths.
// InstructionRunner supplies the runner resolver used by hook collection.
// NormativeReferences shares sorted tree/blob identities with CLI display.
//
// Plan checks use the plan aspect and devkit-judge. Detection instructions ask
// the same runner as the required check for form.DetectionResult.
// Options defers forward-asset loading so pause, resume, and status can use the
// schemas in the work base commit while configuration is being edited.
// Instruction records retain issuance facts for rebuilding deleted briefs.
// Commit recovery records the starting HEAD in a commit_base artifact before
// committing, then compares the current HEAD tree with the evaluated target. The passing
// index at .devkit/detect.json is a JSON array of check.DetectionTest values.
// Registered judgment commands receive the JSON request on standard input;
// detection commands receive the request and detection examples together.
//
// Source of truth: docs/architecture/modules.md#engine
package engine
