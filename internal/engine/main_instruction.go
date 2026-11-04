package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

// Instruct records main-agent guidance without changing the plan or completion conditions.
func (e *Engine) Instruct(ctx context.Context, data []byte) (Result, error) {
	lock, err := e.mutation(ctx)
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	var instruction ledger.MainInstruction
	if err := e.forms["main-instruction"].Decode(data, &instruction); err != nil {
		return Result{}, err
	}
	if _, err := e.append(&instruction); err != nil {
		return Result{}, err
	}
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}

func (e *Engine) mainInstructions(audience, stageID string) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	var text strings.Builder
	for _, record := range records {
		instruction, ok := record.Payload.(*ledger.MainInstruction)
		if !ok || instruction.Audience != "" && instruction.Audience != audience || stageID != "" && instruction.StageID != "" && instruction.StageID != stageID {
			continue
		}
		fmt.Fprintf(&text, "\nRecord: %s\n", record.ID)
		if instruction.StageID != "" {
			fmt.Fprintf(&text, "Target stage: %s\n", instruction.StageID)
		}
		text.WriteString(instruction.Text)
		text.WriteString("\n")
	}
	return text.String(), nil
}

func mainInstructionBrief(body brief.Brief, text string) brief.Brief {
	if text != "" {
		body.Text += "\n\n## Main agent instructions\n\nThese instructions do not change the approved plan or completion conditions. Plan changes require devkit plan and approval.\n" + text
	}
	return body
}
