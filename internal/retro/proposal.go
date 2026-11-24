package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func oldProposal(p doc) bool {
	c := obj(p["content"])
	return c["proposal_path"] != nil || c["edits"] != nil || c["backlog"] != nil
}

func proposalChanges(ctx record.ContextData, input doc) ([]string, error) {
	paths := []string{}
	untracked, e := gitx.Run(ctx.Cwd, "ls-files", "--others", "--exclude-standard", "-z")
	if e != nil {
		return nil, e
	}
	out, e := gitx.Run(ctx.Cwd, "diff", "--name-only", "--no-renames", "-z", str(input["proposal_base"]), "--")
	if e != nil {
		return nil, e
	}
	for _, p := range strings.Split(out+"\x00"+untracked, "\x00") {
		if p != "" && !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func committedErrors(ctx record.ContextData, draft any) ([]string, doc, error) {
	p, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, nil, e
	}
	c := obj(p["content"])
	a, e := json.Marshal(draft)
	if e != nil {
		return nil, nil, e
	}
	b, e := json.Marshal(c["draft"])
	if e != nil {
		return nil, nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, nil, e
	}
	status, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return nil, nil, e
	}
	if oldProposal(p) || p["run_id"] != ctx.RunID || c["commit"] != head || status != "" || string(a) != string(b) {
		return []string{"checkout changed after the judgment or draft has no committed proposal; run retro check again"}, p, nil
	}
	return []string{}, p, nil
}

func commitProposal(ctx record.ContextData, artifacts string, input, draft doc) (doc, error) {
	if e := preflight(ctx); e != nil {
		return nil, e
	}
	problems, p, e := committedErrors(ctx, draft)
	if e != nil {
		return nil, e
	}
	if len(problems) == 0 && num(p["version"]) == num(input["next_proposal_version"]) {
		return p, nil
	}
	if num(input["next_proposal_version"]) != num(p["version"])+1 {
		return nil, fmt.Errorf("proposal version already has a commit; start the next draft version")
	}
	for path, body := range obj(input["backlog"]) {
		b, e := os.ReadFile(filepath.Join(ctx.Cwd, path))
		if os.IsNotExist(e) && str(obj(p["content"])["commit"]) != "" {
			tracked, lookup := gitx.Run(ctx.Cwd, "ls-tree", "--name-only", "HEAD", "--", path)
			if lookup != nil {
				return nil, lookup
			}
			if tracked == "" {
				continue
			}
		}
		if e != nil {
			return nil, e
		}
		if string(b) != str(body) {
			return nil, fmt.Errorf("backlog changed since prepare: %s", path)
		}
	}
	units, ids, e := materializedUnits(ctx, input, draft)
	if e != nil {
		return nil, e
	}
	for path, text := range units {
		if e = put(filepath.Join(ctx.Cwd, path), str(text)); e != nil {
			return nil, e
		}
	}
	for _, id := range stringsOf(obj(p["content"])["unit_ids"]) {
		if !slices.Contains(ids, id) {
			path := filepath.Join(ctx.Cwd, str(obj(input["folders"])["units"]), id+".md")
			if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
				return nil, e
			}
		}
	}
	for path := range obj(input["backlog"]) {
		if e = os.Remove(filepath.Join(ctx.Cwd, path)); e != nil && !os.IsNotExist(e) {
			return nil, e
		}
	}
	paths, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	if e = commitPaths(ctx, paths, fmt.Sprintf("chore: propose retro improvements v%d", num(input["next_proposal_version"]))); e != nil {
		return nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	written, e := record.Write(ctx, "proposal", doc{"content": doc{"draft": draft, "commit": head, "base_commit": input["proposal_base"], "unit_ids": ids, "ready": false}}, "")
	if e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "baseline.json"), doc{"commit": head}); e != nil {
		return nil, e
	}
	return written.Entry, nil
}

func finishProposal(ctx record.ContextData, proposal, content doc) error {
	proposal["content"] = content
	b, e := json.MarshalIndent(proposal, "", "  ")
	if e != nil {
		return e
	}
	path := filepath.Join(ctx.Dir, "proposals", fmt.Sprintf("%d.json", num(proposal["version"])))
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Join(ctx.Dir, "proposals"), ".retro-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(info.Mode().Perm()); e == nil {
		_, e = f.Write(append(b, '\n'))
	}
	closed := f.Close()
	if e != nil {
		return e
	}
	if closed != nil {
		return closed
	}
	return os.Rename(f.Name(), path)
}

func reviewedDraftErrors(ctx record.ContextData, artifacts string, entry doc) ([]string, error) {
	errors, p, e := committedErrors(ctx, entry["draft"])
	if e != nil {
		return nil, e
	}
	if entry["commit"] != obj(p["content"])["commit"] || num(entry["proposal_version"]) != num(p["version"]) {
		errors = append(errors, "proposal commit changed after the judgment")
	}
	draft, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(draft, entry["draft"]) {
		errors = append(errors, "draft changed after the judgment")
	}
	return errors, nil
}
