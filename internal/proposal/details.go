package proposal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/status"
)

func detailsPath(dir string, p Document) string {
	return filepath.Join(dir, "proposals", fmt.Sprintf("%d.details.json", num(p["version"])))
}

func SaveDetails(ctx record.ContextData, p Document) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(detailsPath(ctx.Dir, p), append(data, '\n'), 0644)
}

func Commands() []cli.Command {
	const usage = "devkit proposal show <branch>"
	return []cli.Command{{Area: "proposal", Name: "show", Usage: usage, Run: func(args []string, env cli.Env) error {
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
			return cli.Usage(usage)
		}
		common, err := gitx.Run(env.Cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return err
		}
		root := common
		if filepath.Base(common) == ".git" {
			root = filepath.Dir(common)
		}
		state, why := status.StateDir(env, root)
		if state == "" {
			return fmt.Errorf("%s", why)
		}
		dir := record.RecordDir(state, args[0])
		p, err := record.Latest(dir, "proposal")
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("no proposal for branch %s", args[0])
		}
		data, err := os.ReadFile(detailsPath(dir, p))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(data)
		return err
	}}}
}
