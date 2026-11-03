package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
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
		if strings.HasPrefix(branch, "devkit/") {
			workID = strings.TrimPrefix(branch, "devkit/")
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
			data, err := os.ReadFile(filepath.Join(w.Root(), "devkit.toml"))
			if err != nil {
				return nil, err
			}
			settings, err := config.Parse(data)
			if err != nil {
				return nil, err
			}
			k, err := kit.Open(w.Root(), settings.KitVersion)
			if err != nil {
				return nil, err
			}
			schemas := make(map[string][]byte)
			for _, name := range engine.SchemaNames() {
				schemas[name], err = k.ReadFile("schemas/" + name + ".json")
				if err != nil {
					return nil, err
				}
			}
			return schemas, nil
		},
	})
}
