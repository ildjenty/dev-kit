package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
	"github.com/oklog/ulid/v2"
)

func newEngine(ctx context.Context, cwd, session string) (*engine.Engine, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return nil, err
	}
	if session != "" && !localName(session) {
		return nil, fmt.Errorf("invalid session ID: %q", session)
	}
	runRoot := filepath.Join(w.Root(), ".devkit/run")
	workID := ""
	if session != "" {
		observation, err := hook.ReadSession(runRoot, session)
		if err != nil {
			return nil, err
		}
		if observation != nil {
			workID = observation.WorkID
		}
	}
	if workID == "" {
		branch, err := w.Branch(ctx)
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(filepath.Join(runRoot, "work"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			records, err := fsstore.New(filepath.Join(runRoot, "work", entry.Name())).List()
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				if record.Kind != string(ledger.KindWork) {
					continue
				}
				var work ledger.Work
				if err := fsstore.DecodePayload(record.Payload, &work); err != nil {
					return nil, err
				}
				if work.Branch == branch {
					workID = work.ID
				}
			}
		}
	}
	if workID != "" && !localName(workID) {
		return nil, fmt.Errorf("invalid work ID: %q", workID)
	}
	return engine.New(engine.Options{
		Workspace: w, SessionID: session, WorkID: workID, KitVersion: kit.Version,
		RunRoot: runRoot, WorkRoot: filepath.Join(runRoot, "work", workID),
		NewWork: func() (string, string) {
			id := ulid.Make().String()
			return id, filepath.Join(runRoot, "work", id)
		},
		ReadConfig: func(ctx context.Context, ref string) (config.Config, error) {
			data, err := w.ReadFile(ctx, ref, "devkit.toml")
			if err != nil {
				return config.Config{}, err
			}
			return config.Parse(data)
		},
		LoadSchemas: func() (map[string][]byte, error) {
			head, err := w.Head(ctx)
			if err != nil {
				return nil, err
			}
			source, err := w.ReadFile(ctx, head, "devkit.toml")
			if err != nil {
				return nil, err
			}
			settings, err := config.Parse(source)
			if err != nil {
				return nil, err
			}
			if settings.KitVersion != kit.Version {
				return nil, fmt.Errorf("configuration and binary kit versions do not match")
			}
			schemas := make(map[string][]byte)
			for _, name := range engine.SchemaNames() {
				schemas[name], err = kit.Schema(name)
				if err != nil {
					return nil, err
				}
			}
			return schemas, nil
		},
	})
}
