package retro

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/proposal"
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
	return proposal.CommittedErrors(ctx, draft)
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
	for path := range obj(input["backlog"]) {
		if e = os.Remove(filepath.Join(ctx.Cwd, path)); e != nil && !os.IsNotExist(e) {
			return nil, e
		}
	}
	paths, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	p, e = proposal.Commit(ctx, paths, fmt.Sprintf("chore: propose retro improvements v%d", num(input["next_proposal_version"])), doc{"draft": draft, "base_commit": input["proposal_base"], "ready": false})
	if e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "baseline.json"), doc{"commit": obj(p["content"])["commit"]}); e != nil {
		return nil, e
	}
	return p, nil
}

func finishProposal(ctx record.ContextData, p, content doc) error {
	return proposal.Save(ctx, p, content)
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
