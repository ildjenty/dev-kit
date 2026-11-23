package criteria

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type Options struct {
	Root string
	Base string
	Dir  string
}
type Source struct {
	Dir   string   `json:"dir,omitempty"`
	Base  string   `json:"base"`
	Paths []string `json:"paths"`
}

func CopyCriteria(o Options) (Source, error) {
	source := filepath.Join(o.Dir, "source.json")
	data, err := os.ReadFile(source)
	if err == nil {
		var result Source
		if err := json.Unmarshal(data, &result); err != nil {
			return Source{}, err
		}
		result.Dir = o.Dir
		return result, nil
	}
	if !os.IsNotExist(err) {
		return Source{}, err
	}
	c, err := config.Load(o.Root)
	if err != nil {
		return Source{}, err
	}
	files, err := gitx.Run(o.Root, "ls-tree", "-r", "--name-only", "-z", o.Base, "--", ":(literal)"+c.Standards)
	if err != nil {
		return Source{}, err
	}
	if files == "" {
		return Source{}, fmt.Errorf("%s does not exist at %s", c.Standards, o.Base)
	}
	if err := os.MkdirAll(o.Dir, 0755); err != nil {
		return Source{}, err
	}
	cmd := exec.Command("git", "archive", "--format=tar", o.Base, "--", c.Standards)
	cmd.Dir = o.Root
	archive, err := cmd.Output()
	if err != nil {
		return Source{}, err
	}
	tar := exec.Command("tar", "-x", "-f", "-", "-C", o.Dir)
	tar.Stdin = strings.NewReader(string(archive))
	if output, err := tar.CombinedOutput(); err != nil {
		return Source{}, fmt.Errorf("%w: %s", err, output)
	}
	result := Source{Base: o.Base, Paths: []string{c.Standards}}
	data, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Source{}, err
	}
	if err := os.WriteFile(source, append(data, '\n'), 0644); err != nil {
		return Source{}, err
	}
	result.Dir = o.Dir
	return result, nil
}
func Commands() []cli.Command {
	return []cli.Command{{Area: "criteria", Usage: "devkit criteria", Run: func(args []string, env cli.Env) error {
		if env.Getenv("ARTIFACTS_DIR") == "" {
			return fmt.Errorf("ARTIFACTS_DIR must be set")
		}
		ctx, err := record.Context(env)
		if err != nil {
			return err
		}
		requests, err := record.List(env.Path(ctx.Dir), "request")
		if err != nil {
			return err
		}
		if len(requests) == 0 {
			return fmt.Errorf("no request is recorded yet")
		}
		root, err := gitx.Run(ctx.Cwd, "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		base, _ := requests[0]["criteria_commit"].(string)
		dir := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "criteria")
		result, err := CopyCriteria(Options{root, base, env.Path(dir)})
		if err != nil {
			return err
		}
		result.Dir = dir
		return env.JSON(result)
	}}}
}
