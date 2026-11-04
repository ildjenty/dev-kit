package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

// Check reports a single doctor observation without changing repository files.
type Check struct {
	Name    string
	OK      bool
	Message string
}

// Report contains configuration, repository norms, git, work compatibility, and hook observations.
type Report struct {
	Root   string
	Checks []Check
}

// Healthy reports whether every requested check succeeded.
func (r Report) Healthy() bool {
	for _, check := range r.Checks {
		if !check.OK {
			return false
		}
	}
	return true
}

// Doctor verifies installation and recorded work versions without writing files.
// The caller supplies the current host session ID. An empty ID leaves hook
// activity unverified; observations from other sessions never substitute for it.
// SessionStart observation does not prove that every other hook event has run.
func Doctor(ctx context.Context, cwd, sessionID string) (Report, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return Report{}, err
	}
	r := Report{Root: w.Root()}
	data, err := os.ReadFile(filepath.Join(r.Root, "devkit.toml"))
	var c config.Config
	if err == nil {
		c, err = config.Parse(data)
	}
	r.add("devkit.toml", "Configuration is valid.", err)
	if err == nil {
		var versionErr error
		if c.KitVersion != kit.Version {
			versionErr = fmt.Errorf("kit version mismatch: binary %q, devkit.toml %q", kit.Version, c.KitVersion)
		}
		r.add("kit_version", "Binary and configuration kit versions match.", versionErr)
	}
	r.checkHostFiles()
	paths := c.RepositoryPaths()
	for _, name := range []string{paths.Principles, paths.Stages, paths.Checks, paths.Plans, paths.Templates} {
		info, err := os.Stat(filepath.Join(r.Root, filepath.FromSlash(name)))
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("expected a directory: %s", name)
		}
		r.add(name, "Norm directory is present.", err)
	}
	r.add(paths.Rules, "Rules file is present.", requireFile(filepath.Join(r.Root, filepath.FromSlash(paths.Rules))))
	for _, name := range []string{
		".agents/skills/devkit/SKILL.md",
		".claude/agents/devkit-judge.md", ".claude/agents/devkit-planner.md",
		".claude/agents/devkit-worker.md",
		".codex/agents/devkit-judge.toml", ".codex/agents/devkit-planner.toml",
		".codex/agents/devkit-worker.toml",
	} {
		r.add(name, "File is present.", requireFile(filepath.Join(r.Root, name)))
	}
	for _, name := range []string{".devkit/run/sessions", ".devkit/run/work"} {
		info, err := os.Stat(filepath.Join(r.Root, name))
		if errors.Is(err, os.ErrNotExist) {
			r.add(name, "Runtime directory will be created when needed.", nil)
			continue
		}
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("expected a directory: %s", name)
		}
		r.add(name, "Directory is present.", err)
	}
	target, err := os.Readlink(filepath.Join(r.Root, ".claude/skills/devkit"))
	if err == nil && target != "../../.agents/skills/devkit" {
		err = fmt.Errorf("skill link must point to ../../.agents/skills/devkit; existing link was preserved")
	}
	r.add("skill_link", "Claude Code links to the canonical devkit skill.", err)
	ignored, err := runtimeIgnored(ctx, r.Root)
	if err == nil && !ignored {
		err = fmt.Errorf(".devkit/run/ is not ignored; run devkit init and review its additions")
	}
	r.add("gitignore", "Git ignores the runtime directory.", err)
	_, headErr := w.Head(ctx)
	if headErr != nil {
		headErr = fmt.Errorf("HEAD is unavailable; commit the installed initialization files before starting work: %w", headErr)
	}
	r.add("HEAD", "HEAD exists.", headErr)
	status, statusErr := runGit(ctx, r.Root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching", "--", "devkit.toml", paths.Plans)
	if statusErr == nil && len(status) != 0 {
		statusErr = fmt.Errorf("initialization files are uncommitted; commit devkit.toml and %s before starting work:\n%s", paths.Plans, status)
	}
	r.add("init_committed", "Configuration and plans are committed.", statusErr)
	r.checkWorkVersions(c.KitVersion)
	r.checkSession(sessionID)
	return r, nil
}

func (r *Report) add(name, success string, err error) {
	check := Check{Name: name, OK: err == nil, Message: success}
	if err != nil {
		check.Message = err.Error()
	}
	r.Checks = append(r.Checks, check)
}

func (r *Report) checkHostFiles() {
	for _, fragment := range hostFragments {
		before, err := os.ReadFile(filepath.Join(r.Root, fragment.target))
		if err == nil {
			var required, after []byte
			required, err = kit.HostFragment(strings.TrimPrefix(fragment.source, "hosts/"))
			if err == nil {
				after, err = fragment.merge(before, required)
			}
			if err == nil && !bytes.Equal(before, after) {
				err = fmt.Errorf("required kit settings are missing; run devkit init and review its additions")
			}
		}
		r.add(fragment.target, "Required kit settings are present.", err)
	}
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular file: %s", path)
	}
	return err
}

func (r *Report) checkWorkVersions(configVersion string) {
	root := filepath.Join(r.Root, ".devkit/run/work")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		r.add("work_versions", "", err)
		return
	}
	var validator *ledger.Validator
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if validator == nil {
			validator, err = workVersionValidator()
			if err != nil {
				r.add("work_versions", "", err)
				return
			}
		}
		err := checkWorkVersion(filepath.Join(root, entry.Name()), validator, configVersion)
		r.add("work_version/"+entry.Name(), "The recorded Work kit version is compatible.", err)
	}
}

func (r *Report) checkSession(sessionID string) {
	if sessionID == "" {
		r.add("hook_session", "", fmt.Errorf("hook activity is unverified without a current session ID. %s", trustGuidance))
		return
	}
	if !fs.ValidPath(sessionID) || strings.Contains(sessionID, "/") || sessionID == "." {
		r.add("hook_session", "", fmt.Errorf("invalid session ID: %q", sessionID))
		return
	}
	root := filepath.Join(r.Root, ".devkit/run/sessions", sessionID)
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		r.add("hook_session", "", err)
		return
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-SessionStart.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err == nil {
			var observation struct {
				Host       string    `json:"host"`
				SessionID  string    `json:"session_id"`
				ObservedAt time.Time `json:"observed_at"`
				WorkID     string    `json:"work_id,omitempty"`
			}
			err = json.Unmarshal(data, &observation)
			if err == nil && (observation.Host == "" || observation.ObservedAt.IsZero()) {
				err = fmt.Errorf("session observation %s requires host and observed_at", entry.Name())
			}
			if err == nil && observation.SessionID != sessionID {
				err = fmt.Errorf("session observation %s does not identify session %q", entry.Name(), sessionID)
			}
		}
		if err != nil {
			r.add("hook_session", "", err)
			return
		}
		r.add("hook_session", "SessionStart was observed for the current session; other hook events are checked when their observations are required.", nil)
		return
	}
	r.add("hook_session", "", fmt.Errorf("SessionStart observation is missing for session %q. %s", sessionID, trustGuidance))
}

func workVersionValidator() (*ledger.Validator, error) {
	sources := make(map[string][]byte)
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
		source, err := kit.Schema(name)
		if err != nil {
			return nil, err
		}
		sources[name] = source
	}
	return ledger.NewValidator(sources)
}

func checkWorkVersion(root string, validator *ledger.Validator, configVersion string) error {
	records, err := ledger.List(fsstore.New(root), validator)
	found := false
	if err == nil {
		for _, record := range records {
			work, ok := record.Payload.(*ledger.Work)
			if !ok {
				continue
			}
			found = true
			if work.KitVersion != kit.Version || configVersion != "" && work.KitVersion != configVersion {
				err = fmt.Errorf("Work kit version %q does not match binary %q and configuration %q", work.KitVersion, kit.Version, configVersion)
				break
			}
		}
		if !found {
			err = fmt.Errorf("Work record is missing")
		}
	}
	return err
}
