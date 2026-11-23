package initialize

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/kit"
)

func Commands() []cli.Command {
	return []cli.Command{{Area: "init", Usage: "devkit init [--force] [dir]", Run: func(args []string, env cli.Env) error {
		force := false
		dir := ""
		for _, arg := range args {
			if arg == "--force" {
				force = true
			} else if strings.HasPrefix(arg, "-") || dir != "" {
				return cli.Usage("devkit init [--force] [dir]")
			} else {
				dir = arg
			}
		}
		target := env.Path(dir)
		return fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			dest := filepath.Join(target, filepath.FromSlash(path))
			owned := strings.HasPrefix(path, ".archon/workflows/dev/") || strings.HasPrefix(path, ".claude/skills/dev-discuss/") || strings.HasPrefix(path, ".claude/skills/dev-runs/")
			if !owned && !force {
				if _, err := os.Stat(dest); err == nil {
					_, err = fmt.Fprintln(env.Stdout, "Left alone: "+path)
					return err
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			data, err := kit.Files.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return err
			}
			_, err = fmt.Fprintln(env.Stdout, "Wrote: "+path)
			return err
		})
	}}}
}
