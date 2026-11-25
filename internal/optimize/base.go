package optimize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var warningCount = regexp.MustCompile(`[1-9][0-9]* warnings?`)

func validateYAML(data []byte) error { var v any; return yaml.Unmarshal(data, &v) }
func validateArchonConfig(root string) error {
	data, err := os.ReadFile(filepath.Join(root, ".archon/config.yaml"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var s struct {
		Worktree struct {
			BaseBranch        string   `yaml:"baseBranch"`
			PostCreate        []string `yaml:"postCreate"`
			PostCreateTimeout *float64 `yaml:"postCreateTimeout"`
		} `yaml:"worktree"`
	}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.Worktree.PostCreateTimeout != nil && *s.Worktree.PostCreateTimeout <= 0 {
		return fmt.Errorf("worktree.postCreateTimeout must be positive")
	}
	return nil
}
