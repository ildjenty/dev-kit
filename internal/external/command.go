package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

// Commands receive the original JSON on stdin and DEVKIT_OPERATION_ID as their
// external idempotency key. Both write and query commands return one JSON object
// on stdout: {"status":"succeeded|failed|unknown","result":"...","external_id":"..."}.
// A nonzero exit or malformed response cannot establish an external outcome.
type commandResult struct {
	Status     ledger.ExternalStatus `json:"status"`
	Result     string                `json:"result,omitempty"`
	ExternalID string                `json:"external_id,omitempty"`
	notStarted bool
}

func (e *External) invoke(ctx context.Context, command config.Command, input json.RawMessage, id string) commandResult {
	ctx, cancel := context.WithTimeout(ctx, command.Timeout())
	defer cancel()
	result := e.run(ctx, command, input, id)
	if err := ctx.Err(); err != nil {
		return commandResult{Status: ledger.ExternalUnknown, Result: fmt.Sprintf("External command outcome is unknown: %v", err)}
	}
	return result
}

func (e *External) runCommand(ctx context.Context, command config.Command, input json.RawMessage, id string) commandResult {
	path, err := e.executablePath(command.Executable)
	if err != nil {
		return commandResult{Status: ledger.ExternalFailed, Result: err.Error(), notStarted: true}
	}
	cmd := exec.CommandContext(ctx, path, command.Args...)
	// Inherited output pipes must not keep Wait blocked after exit or timeout.
	cmd.WaitDelay = command.Timeout()
	cmd.Dir = e.root
	cmd.Env = append(os.Environ(), "DEVKIT_OPERATION_ID="+id)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return commandResult{Status: ledger.ExternalFailed, Result: fmt.Sprintf("Start external command: %v", err), notStarted: true}
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		return commandResult{Status: ledger.ExternalUnknown, Result: fmt.Sprintf("External command outcome is unknown: %v", ctx.Err())}
	case err = <-waited:
	}
	if err != nil {
		return commandResult{Status: ledger.ExternalUnknown, Result: fmt.Sprintf("External command outcome is unknown: %v; %s", err, stderr.String())}
	}
	var result commandResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return commandResult{Status: ledger.ExternalUnknown, Result: fmt.Sprintf("Read external command outcome: %v", err)}
	}
	switch result.Status {
	case ledger.ExternalSucceeded, ledger.ExternalFailed, ledger.ExternalUnknown:
		return result
	default:
		return commandResult{Status: ledger.ExternalUnknown, Result: "External command did not report a recognized outcome"}
	}
}
