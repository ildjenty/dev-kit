package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/workspace"
	"github.com/oklog/ulid/v2"
)

func newTestEngine(ctx context.Context, root, session string) (*Engine, error) {
	w, err := workspace.Open(ctx, root)
	if err != nil {
		return nil, err
	}
	runRoot := filepath.Join(w.Root(), ".devkit/run")
	observation, err := hook.ReadSession(runRoot, session)
	if err != nil {
		return nil, err
	}
	id := ""
	if observation != nil {
		id = observation.WorkID
	}
	if id == "" {
		branch, err := w.Branch(ctx)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(branch, "devkit/") {
			id = strings.TrimPrefix(branch, "devkit/")
		}
	}
	return New(Options{
		Workspace: w, SessionID: session, WorkID: id, KitVersion: kit.Version,
		RunRoot: runRoot, WorkRoot: filepath.Join(runRoot, "work", id),
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
			schemas := map[string][]byte{}
			for _, name := range SchemaNames() {
				schemas[name], err = k.ReadFile("schemas/" + name + ".json")
				if err != nil {
					return nil, err
				}
			}
			return schemas, nil
		},
	})
}
