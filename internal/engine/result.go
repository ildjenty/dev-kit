package engine

import (
	"context"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func (e *Engine) rejectResult(instruction ledger.Instruction, obs observedResult, reason error) (bool, error) {
	_, err := e.append(&ledger.ResultRejection{InstructionID: instruction.ID, ResultPath: obs.path, Reason: reason.Error()})
	return true, err
}

func (e *Engine) reissueRejected(ctx context.Context, state *ledger.State) (bool, error) {
	records, err := e.records()
	if err != nil {
		return false, err
	}
	rejected := map[string]*ledger.ResultRejection{}
	for _, record := range records {
		if rejection, ok := record.Payload.(*ledger.ResultRejection); ok {
			rejected[rejection.InstructionID] = rejection
		}
	}
	for _, instruction := range state.UnacceptedInstructions() {
		failure := rejected[instruction.ID]
		if failure == nil {
			continue
		}
		count := 0
		for _, record := range records {
			if issued, ok := record.Payload.(*ledger.Instruction); ok && issued.Kind == instruction.Kind && issued.TargetKey == instruction.TargetKey && rejected[issued.ID] != nil {
				count++
			}
		}
		return true, e.retryRejected(ctx, state, instruction, failure, count)
	}
	return false, nil
}

func (e *Engine) retryRejected(ctx context.Context, state *ledger.State, meta ledger.Instruction, failure *ledger.ResultRejection, count int) error {
	data, err := e.readInstruction(meta.ID)
	if err != nil {
		return err
	}
	meta.Reissue++
	meta.ID = instructionID(e.workID, meta.Kind, meta.TargetKey, meta.Reissue)
	meta.SessionID = e.sessionID
	data.Instruction.ID, data.Instruction.Brief = meta.ID, ""
	if meta.Kind == form.InstructionUnitWork {
		if err := e.prepareUnitRetry(ctx, state, meta); err != nil {
			return err
		}
		data.Instruction.TargetPath = meta.TargetPath
	}
	if data.Request != nil {
		request := *data.Request
		request.InstructionID = meta.ID
		data.Request = &request
	}
	body, err := e.regenerateBrief(ctx, meta, data)
	if err != nil {
		return err
	}
	// Tell the reissued subagent why its previous result was rejected, so the retry can differ.
	brief := body.Text + "\n\n## Previous result rejected\n\nThe previous result for this request was rejected by devkit:\n\n" + failure.Reason + "\n\nReturn a result that resolves this reason.\n"
	if _, err := e.publishInstruction(meta, true, data, brief); err != nil {
		return err
	}
	if count < 3 {
		return nil
	}
	text := fmt.Sprintf("The subagent result was rejected after two failed reissues for the same target. Provide instructions before retrying.\nInstruction: %s\nReason: %s\nResult: %s", failure.InstructionID, failure.Reason, failure.ResultPath)
	q := e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "malformed_result", text, "")
	q.Evidence = []form.Reference{{External: failure.ResultPath}}
	_, err = e.saveQuestion(state, q, "")
	return err
}
