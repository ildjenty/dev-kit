package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/unitdone"
)

const usage = "devkit status [--json] [branch]"

const archonTimeout = 20 * time.Second

func Commands() []cli.Command {
	return []cli.Command{{Area: "status", Usage: usage, Run: run}}
}

type Branch struct {
	Branch    string     `json:"branch"`
	Unit      string     `json:"unit"`
	Workflow  string     `json:"workflow"`
	Outcome   string     `json:"outcome"`
	At        string     `json:"recorded_at"`
	RunID     string     `json:"run_id"`
	Questions []Question `json:"questions"`
	Back      any        `json:"back,omitempty"`
	Stop      any        `json:"stop,omitempty"`
	Carried   any        `json:"carried,omitempty"`
	Restarts  int        `json:"restarts"`
	Limit     int        `json:"restart_limit"`
	Next      string     `json:"next"`
}

type Question struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Target        any      `json:"target"`
	NextWorkflows []string `json:"next_workflows"`
}

func run(args []string, env cli.Env) error {
	asJSON := false
	branch := ""
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "-") || branch != "":
			return cli.Usage(usage)
		default:
			branch = arg
		}
	}
	root, err := mainRoot(env)
	if err != nil {
		return err
	}
	state, why := StateDir(env, root)
	if state == "" {
		if asJSON {
			return env.JSON(map[string]any{"branches": []Branch{}, "note": why})
		}
		_, err := fmt.Fprintln(env.Stdout, why)
		return err
	}
	limit := 3
	if c, err := config.Load(env.Cwd); err == nil && c.Limits.Restarts > 0 {
		limit = c.Limits.Restarts
	}
	branches, err := Read(filepath.Join(state, "records"), branch, limit)
	if err != nil {
		return err
	}
	if asJSON {
		return env.JSON(map[string]any{"state_dir": state, "branches": branches})
	}
	return show(env, state, branch, branches)
}

func mainRoot(env cli.Env) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = env.Cwd
	cmd.Env = env.Environ
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %s", env.Cwd)
	}
	common := strings.TrimSpace(string(out))
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common), nil
	}
	return common, nil
}

func StateDir(env cli.Env, root string) (string, string) {
	if dir := env.Getenv("STATE_DIR"); dir != "" {
		return dir, ""
	}
	home := env.Getenv("ARCHON_HOME")
	if home == "" {
		home = filepath.Join(env.Getenv("HOME"), ".archon")
	}
	ctx, cancel := context.WithTimeout(context.Background(), archonTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "archon", "project", "list", "--json")
	cmd.Dir = root
	cmd.Env = env.Environ
	out, err := cmd.Output()
	if err != nil {
		return "", "Could not list Archon projects (archon project list --json failed); set STATE_DIR to the project's state folder, or check `devkit doctor`."
	}
	var listed struct {
		Projects []struct {
			Key        string `json:"key"`
			DefaultCwd string `json:"default_cwd"`
		} `json:"projects"`
	}
	if json.Unmarshal(out, &listed) != nil {
		return "", "Could not read `archon project list --json`; set STATE_DIR to the project's state folder."
	}
	want := canonical(root)
	for _, p := range listed.Projects {
		if p.Key != "" && canonical(p.DefaultCwd) == want {
			return filepath.Join(home, "workspaces", filepath.FromSlash(p.Key), "state"), ""
		}
	}
	return "", "No workflow has run in this repository yet (Archon has no project for " + root + ")."
}

func canonical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func Read(records, only string, limit int) ([]Branch, error) {
	entries, err := os.ReadDir(records)
	if os.IsNotExist(err) {
		return []Branch{}, nil
	}
	if err != nil {
		return nil, err
	}
	branches := []Branch{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, err := url.PathUnescape(entry.Name())
		if err != nil {
			name = entry.Name()
		}
		if only != "" && name != only {
			continue
		}
		b, ok, err := readBranch(filepath.Join(records, entry.Name()), name, limit)
		if err != nil {
			return nil, err
		}
		if ok {
			branches = append(branches, b)
		}
	}
	sort.Slice(branches, func(i, j int) bool { return branches[i].At > branches[j].At })
	return branches, nil
}

func readBranch(dir, name string, limit int) (Branch, bool, error) {
	h, err := record.History(dir)
	if err != nil {
		return Branch{}, false, err
	}
	if len(h.Runs) == 0 {
		return Branch{}, false, nil
	}
	b := Branch{Branch: name, Questions: []Question{}, Restarts: record.RestartsOf(h), Limit: limit}
	b.Unit, err = unitdone.RequestUnit(dir)
	if err != nil {
		return Branch{}, false, err
	}
	last := h.Runs[len(h.Runs)-1]
	var result record.Document
	for _, r := range h.Results {
		if r["run_id"] == last["run_id"] {
			result = r
		}
	}
	if result == nil {
		b.Workflow, b.RunID, b.At = text(last["workflow"]), text(last["run_id"]), text(last["started_at"])
		b.Outcome = "no result"
		b.Next = fmt.Sprintf("Run %s of %s has no result yet: it is running, or it failed. Check `archon workflow get %s`.", b.RunID, b.Workflow, b.RunID)
		return b, true, nil
	}
	b.Workflow, b.Outcome, b.At, b.RunID = text(result["workflow"]), text(result["outcome"]), text(result["recorded_at"]), text(result["run_id"])
	b.Back, b.Stop, b.Carried = result["back"], result["stop"], result["carried"]
	if qs, ok := result["questions"].([]any); ok {
		for _, raw := range qs {
			q, _ := raw.(map[string]any)
			id := text(q["id"])
			if answer, err := record.ReadKeyed(dir, "answer", id); err == nil && answer != nil {
				continue
			}
			next := []string{}
			if list, ok := q["next_workflows"].([]any); ok {
				for _, w := range list {
					next = append(next, text(w))
				}
			}
			b.Questions = append(b.Questions, Question{ID: id, Text: text(q["text"]), Target: q["target"], NextWorkflows: next})
		}
	}
	b.Next = next(b, result)
	return b, true, nil
}

func next(b Branch, result record.Document) string {
	switch b.Outcome {
	case "question":
		if len(b.Questions) == 0 {
			return fmt.Sprintf("Every question is answered; start %s again on %s.", b.Workflow, b.Branch)
		}
		if len(b.Questions) == 1 && isProposal(b.Questions[0]) {
			return fmt.Sprintf("Relay the approval question. On approval, start %s on %s with `--input approve=%s`; to request changes, answer %s with `--input answers=...`.", b.Workflow, b.Branch, text(asMap(b.Questions[0].Target)["ref"]), b.Questions[0].ID)
		}
		ids := []string{}
		for _, q := range b.Questions {
			ids = append(ids, q.ID)
		}
		workflow := b.Workflow
		if len(b.Questions[0].NextWorkflows) > 0 {
			workflow = b.Questions[0].NextWorkflows[0]
		}
		return fmt.Sprintf("Relay %s to the owner verbatim, then start %s on %s with their answers (`--input answers=...`).", strings.Join(ids, ", "), workflow, b.Branch)
	case "back":
		to := text(asMap(b.Back)["to"])
		return fmt.Sprintf("Start %s on %s; its writer receives the premise and observation recorded here.", to, b.Branch)
	case "stop":
		s := asMap(b.Stop)
		state := text(s["state"])
		if b.Workflow == "judge" && strings.HasPrefix(state, "Blocking findings remain") {
			return fmt.Sprintf("Start fix on %s.", b.Branch)
		}
		advice := fmt.Sprintf("Stopped: %s — explain this and the unresolved items to the owner.", strings.TrimRight(state, "。. "))
		if b.Restarts >= b.Limit {
			advice += fmt.Sprintf(" The branch reached its restart limit (%d of %d): starting again needs the owner's words passed as `--input continue=...`.", b.Restarts, b.Limit)
		}
		return advice
	}
	switch b.Workflow {
	case "refine":
		return "The proposal is approved and committed. Publish " + b.Branch + " as a pull request; after it merges, start the units it created, or the next topic (`devkit decisions topics`)."
	case "retro":
		return "The corrections are approved and committed. Publish " + b.Branch + " as a pull request; start its units after it merges."
	case "requirements":
		return "Start plan on " + b.Branch + "."
	case "plan":
		return "Start implement on " + b.Branch + "."
	case "implement", "fix":
		return "Start judge on " + b.Branch + "."
	case "judge":
		return "Start pr on " + b.Branch + "."
	case "pr":
		return "The pull request is open (`gh pr view " + b.Branch + "`). The owner reviews and merges it."
	case "sync":
		if loops, ok := result["loops"].([]any); ok {
			for _, raw := range loops {
				l := asMap(raw)
				if rounds, ok := l["rounds"].(float64); ok && rounds > 0 && strings.Contains(text(l["name"]), "conflict") {
					return "Conflicts were resolved, so the change is no longer the one judged: start judge on " + b.Branch + "."
				}
			}
		}
		return "Start pr on " + b.Branch + " to push the rebased branch."
	}
	return ""
}

func isProposal(q Question) bool { return text(asMap(q.Target)["kind"]) == "proposal" }

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func show(env cli.Env, state, only string, branches []Branch) error {
	w := env.Stdout
	if len(branches) == 0 {
		if only != "" {
			_, err := fmt.Fprintf(w, "No workflow has run on %s.\n", only)
			return err
		}
		_, err := fmt.Fprintln(w, "No workflow has run in this repository yet.")
		return err
	}
	for i, b := range branches {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s: %s %s (%s)\n", b.Branch, b.Workflow, b.Outcome, b.At)
		if b.Unit != "" {
			fmt.Fprintf(w, "  unit: %s\n", b.Unit)
		}
		for _, q := range b.Questions {
			fmt.Fprintf(w, "  %s: %s\n", q.ID, indent(q.Text))
		}
		if s := asMap(b.Stop); s != nil {
			for _, u := range listOf(s["unresolved"]) {
				fmt.Fprintf(w, "  - %s\n", u)
			}
		}
		if bk := asMap(b.Back); bk != nil {
			fmt.Fprintf(w, "  premise: %s\n  observation: %s\n", text(bk["premise"]), text(bk["observation"]))
		}
		fmt.Fprintf(w, "  next: %s\n", b.Next)
	}
	_, err := fmt.Fprintf(w, "\nRecords: %s\n", filepath.Join(state, "records"))
	return err
}

func listOf(v any) []string {
	out := []string{}
	if list, ok := v.([]any); ok {
		for _, item := range list {
			out = append(out, text(item))
		}
	}
	return out
}

func indent(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n    ") }
