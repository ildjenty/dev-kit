package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/source"
)

// externalNormChanges lists documents of norm sources outside the repository whose fixed
// copies differ between two references.
func (e *Engine) externalNormChanges(ctx context.Context, before, after string) ([]string, error) {
	if e.workRoot() == "" {
		return nil, nil
	}
	external, err := e.externalSources(ctx, after)
	if err != nil || len(external) == 0 {
		return nil, err
	}
	old, err := e.pin(ctx, before, false)
	if err != nil {
		return nil, err
	}
	current, err := e.pin(ctx, after, false)
	if err != nil {
		return nil, err
	}
	changed := []string{}
	for name, item := range external {
		if !item.Norm {
			continue
		}
		for _, id := range differentDocuments(old[name], current[name]) {
			changed = append(changed, name+"/"+id)
		}
	}
	slices.Sort(changed)
	return changed, nil
}

func differentDocuments(before, after sourceManifest) []string {
	ids := []string{}
	for id, hash := range after {
		if before[id] != hash {
			ids = append(ids, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// sourceChangeText lists changed outside documents for a loop-edit question.
func (e *Engine) sourceChangeText(ctx context.Context, before, after string) (string, error) {
	live, err := e.externalNormChanges(ctx, before, after)
	if err != nil {
		return "", err
	}
	edits, err := e.sourceChanges(ctx, after, true)
	if err != nil {
		return "", err
	}
	if len(live) == 0 && len(edits) == 0 {
		return "", nil
	}
	var text strings.Builder
	text.WriteString("\nChanged documents of norm sources outside the repository:\n")
	for _, id := range live {
		fmt.Fprintf(&text, "- %s (changed in the source since the confirmed norms were fixed)\n", id)
	}
	for _, change := range edits {
		fmt.Fprintf(&text, "- %s/%s (edited copy: %s; fixed copy it started from: %s)\n", change.Source, change.ID, e.editDirectory(change.Source), change.Base)
	}
	return text.String(), nil
}

// readOnlyChanges lists changed files inside read-only sources of the repository. Improvements
// to read-only sources are recorded as lessons and handed over to their maintainers instead.
func (e *Engine) readOnlyChanges(settings config.Config, paths []string) []string {
	blocked := []string{}
	for name, item := range settings.KnowledgeSources() {
		location, inside := e.repositoryLocation(item)
		if !inside || settings.ValidateImprovementDestination(name) == nil {
			continue
		}
		for _, changed := range paths {
			if changed == location || strings.HasPrefix(changed, location+"/") {
				blocked = append(blocked, changed+" (source "+name+")")
			}
		}
	}
	slices.Sort(blocked)
	return blocked
}

// approvalRecord returns the answer that approved the round's target at its gate, if any.
func approvalRecord(state *ledger.State, run *ledger.StageRun, round *ledger.Round) string {
	target := approvalTree(*round.ReturnedTarget)
	record := ""
	for _, q := range state.Questions() {
		if q.StoppedStage != run.StageID || q.ApprovalTarget != target || !slices.Contains(q.Evidence, form.Reference{RecordID: round.ID}) {
			continue
		}
		if answer, ok := state.AnsweredQuestions()[q.ID]; ok && answer.Answer.Interpretation == ledger.AnswerApproved {
			record = answer.RecordID
		}
	}
	return record
}

// applySourceWrites writes the approved edits of outside sources through their adapters before
// the stage is committed. It returns handled when the round must be refreshed or fixed instead.
func (e *Engine) applySourceWrites(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	names, err := e.editCopies()
	if err != nil || len(names) == 0 {
		return Result{}, false, err
	}
	current, err := e.sourceIdentity()
	if err != nil {
		return Result{}, false, err
	}
	if current != round.ReturnedTarget.Sources {
		r, err := e.refreshTarget(ctx, state, run, round, "")
		return r, true, err
	}
	sources, err := e.knowledgeSources(ctx, roundReference(round))
	if err != nil {
		return Result{}, false, err
	}
	approval := approvalRecord(state, run, round)
	conflicts := []sourceChange{}
	for _, name := range names {
		conflicted, err := e.writeSource(ctx, round, name, sources[name], approval)
		if err != nil {
			return Result{}, false, err
		}
		conflicts = append(conflicts, conflicted...)
	}
	if len(conflicts) > 0 {
		reason, err := e.rebaseConflicts(sources, conflicts)
		if err != nil {
			return Result{}, false, err
		}
		r, err := e.fixRound(ctx, state, p, stage, run, round, reason)
		return r, true, err
	}
	_, err = e.pin(ctx, round.ReturnedTarget.Tree, true)
	return Result{}, false, err
}

// writeSource applies the changes of one editable copy and returns the conflicting changes.
// After every change is applied, the copy's base becomes the written documents.
func (e *Engine) writeSource(ctx context.Context, round *ledger.Round, name string, item config.Source, approval string) ([]sourceChange, error) {
	changes, err := e.editChanges(name)
	if err != nil || len(changes) == 0 {
		return nil, err
	}
	if !item.CanWrite() {
		return nil, fmt.Errorf("source %s is read-only; record the improvement as a lesson for its maintainers", name)
	}
	adapter, err := source.NewFiles(e.Root(), item.Path, nil, "", true)
	if err != nil {
		return nil, err
	}
	conflicts := []sourceChange{}
	for _, change := range changes {
		applied, err := e.writeDocument(ctx, round, adapter, change, approval)
		if err != nil {
			return nil, err
		}
		if !applied {
			conflicts = append(conflicts, change)
		}
	}
	if len(conflicts) > 0 {
		return conflicts, nil
	}
	documents, err := e.editDocuments(name)
	if err != nil {
		return nil, err
	}
	manifest := sourceManifest{}
	for id, body := range documents {
		if manifest[id], err = e.storeBlob(body); err != nil {
			return nil, err
		}
	}
	return nil, e.writeEditBase(name, manifest)
}

// writeDocument records a running write, compares and updates through the adapter, and
// records the outcome. A repeated operation reuses its recorded outcome; an interrupted one
// is resolved by comparing the current document with the expected and written contents.
func (e *Engine) writeDocument(ctx context.Context, round *ledger.Round, adapter *source.Files, change sourceChange, approval string) (bool, error) {
	operation := hash([]byte(strings.Join([]string{round.ID, change.Source, change.ID, change.Base, change.contentHash()}, "\x00")))
	write, beforeID, err := e.recordedWrite(operation)
	if err != nil {
		return false, err
	}
	if write != nil && write.Status != ledger.SourceWriteRunning {
		return write.Status == ledger.SourceWriteApplied, nil
	}
	if write != nil {
		if status := e.interruptedWrite(adapter, change); status != "" {
			return status == ledger.SourceWriteApplied, e.recordWriteOutcome(*write, beforeID, status)
		}
	} else {
		running := ledger.SourceWrite{OperationID: operation, RoundID: round.ID, Source: change.Source, Document: change.ID, ExpectedHash: change.Base, ContentHash: change.contentHash(), Target: *round.ReturnedTarget, ApprovalID: approval, Status: ledger.SourceWriteRunning}
		if err := e.active(); err != nil {
			return false, err
		}
		if beforeID, err = e.append(&running); err != nil {
			return false, err
		}
		write = &running
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	err = adapter.Write(change.ID, change.Body, change.Base)
	if errors.Is(err, source.ErrConflict) {
		return false, e.recordWriteOutcome(*write, beforeID, ledger.SourceWriteConflict)
	}
	if err != nil {
		return false, err
	}
	return true, e.recordWriteOutcome(*write, beforeID, ledger.SourceWriteApplied)
}

func (e *Engine) recordedWrite(operation string) (*ledger.SourceWrite, string, error) {
	records, err := e.records()
	if err != nil {
		return nil, "", err
	}
	var latest *ledger.SourceWrite
	beforeID := ""
	for _, record := range records {
		if write, ok := record.Payload.(*ledger.SourceWrite); ok && write.OperationID == operation {
			latest = write
			if write.Status == ledger.SourceWriteRunning {
				beforeID = record.ID
			}
		}
	}
	return latest, beforeID, nil
}

// interruptedWrite decides an interrupted write from the current document. It returns an
// empty status when the document still has its expected content and the write must be retried.
func (e *Engine) interruptedWrite(adapter *source.Files, change sourceChange) ledger.SourceWriteStatus {
	documents, err := adapter.Select("")
	current := ""
	if err == nil {
		for _, document := range documents {
			if document.ID == change.ID {
				current = source.ContentHash(document.Body)
			}
		}
	}
	switch {
	case err != nil:
		return ledger.SourceWriteConflict
	case current == change.contentHash():
		return ledger.SourceWriteApplied
	case current == change.Base:
		return ""
	default:
		return ledger.SourceWriteConflict
	}
}

func (e *Engine) recordWriteOutcome(write ledger.SourceWrite, beforeID string, status ledger.SourceWriteStatus) error {
	if err := e.active(); err != nil {
		return err
	}
	write.Status, write.BeforeID = status, beforeID
	_, err := e.append(&write)
	return err
}

// rebaseConflicts fixes the current documents that changed outside this work as the new base of
// their copies, keeping the worker's edits, and explains how to review them.
func (e *Engine) rebaseConflicts(sources map[string]config.Source, conflicts []sourceChange) (string, error) {
	var text strings.Builder
	text.WriteString("Documents changed in their source after they were fixed, so devkit did not write them. Review each edit against the current document and submit again.\n")
	for _, change := range conflicts {
		manifest, err := e.fixExternal(change.Source, sources[change.Source])
		if err != nil {
			return "", err
		}
		base, err := e.editBase(change.Source)
		if err != nil {
			return "", err
		}
		if hash, ok := manifest[change.ID]; ok {
			base[change.ID] = hash
		} else {
			delete(base, change.ID)
		}
		if err := e.writeEditBase(change.Source, base); err != nil {
			return "", err
		}
		directory, err := e.fixedDirectory(change.Source, manifest)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&text, "- %s/%s: current document fixed at %s; edited copy at %s\n", change.Source, change.ID, directory, e.editDirectory(change.Source))
	}
	return text.String(), nil
}
