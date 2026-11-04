package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
)

func (e *Engine) lockStart() (*os.File, error) {
	checkout, err := os.Open(e.Root())
	if err != nil {
		return nil, err
	}
	// Lock the existing checkout directory without creating runtime artifacts.
	if err := syscall.Flock(int(checkout.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		checkout.Close()
		if workErr := e.requireUnusedCheckout(); workErr != nil {
			return nil, workErr
		}
		return nil, fmt.Errorf("another start is using this checkout: %w", err)
	}
	return checkout, nil
}

func (e *Engine) requireUnusedCheckout() error {
	root := filepath.Join(e.runRoot(), "work")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		work, err := unfinishedWork(filepath.Join(root, entry.Name()))
		if err != nil {
			return err
		}
		if work != nil {
			return fmt.Errorf("checkout already has unfinished work %s on branch %s; start another loop in a separate worktree (git worktree add <path> -b <branch> or the host's worktree feature) and open a new session there", work.ID, work.Branch)
		}
	}
	return nil
}

func unfinishedWork(root string) (*ledger.Work, error) {
	records, err := fsstore.New(root).List()
	if err != nil {
		return nil, err
	}
	var work *ledger.Work
	status := ledger.WorkActive
	for _, record := range records {
		switch ledger.Kind(record.Kind) {
		case ledger.KindWork:
			work = &ledger.Work{}
			if err := fsstore.DecodePayload(record.Payload, work); err != nil {
				return nil, err
			}
		case ledger.KindStateChange:
			var change ledger.StateChange
			if err := fsstore.DecodePayload(record.Payload, &change); err != nil {
				return nil, err
			}
			if change.WorkStatus != "" {
				status = change.WorkStatus
			}
		}
	}
	if status == ledger.WorkCompleted || status == ledger.WorkAborted {
		return nil, nil
	}
	return work, nil
}
