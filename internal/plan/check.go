package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/judge"
	"github.com/ildjenty/dev-kit/internal/record"
)

func collectChanges(ctx record.ContextData, base, commit string, c config.Config) (document, error) {
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return nil, err
	}
	changes, err := g.Changes(base, commit)
	if err != nil {
		return nil, err
	}
	items := []document{}
	for _, i := range config.DecisionChanges(c, changes) {
		row := document{"path": i.Path, "status": i.Status, "kind": i.Kind, "quality": i.Quality, "ask": i.Ask, "file": i.Path}
		change := i.Status + " " + i.Kind
		if i.OldPath != "" {
			row["oldPath"] = i.OldPath
			change += " from " + i.OldPath
		}
		row["change"] = change
		items = append(items, row)
	}
	for _, f := range changes.Files {
		if config.Covers(c.Decisions, f.Path) || f.Path == changes.Backlog {
			continue
		}
		if f.Status == "" || !strings.ContainsRune("ADRC", rune(f.Status[0])) {
			continue
		}
		found := false
		for _, i := range items {
			if i["quality"] == "structure" && i["file"] == f.Path {
				found = true
			}
		}
		if found {
			continue
		}
		change := f.Status + " file"
		row := document{"path": f.Path, "status": f.Status, "quality": "structure", "kind": "file", "ask": false, "file": f.Path}
		if f.OldPath != "" {
			row["oldPath"] = f.OldPath
			change += " from " + f.OldPath
		}
		row["change"] = change
		items = append(items, row)
	}
	return document{"base": base, "items": items, "files": changes.Files, "addedDirectories": changes.AddedDirectories, "removedDirectories": changes.RemovedDirectories}, nil
}
func commitSkeleton(ctx record.ContextData, goal string) (string, error) {
	status, err := gitx.Run(ctx.Cwd, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		if _, err = gitx.Run(ctx.Cwd, "add", "-A"); err != nil {
			return "", err
		}
		if _, err = gitx.Run(ctx.Cwd, "commit", "-q", "-m", "feat: skeleton: "+goal+"\n\nArchon-Run: "+ctx.RunID); err != nil {
			return "", err
		}
	}
	return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
}
func checkCommand(ctx record.ContextData, artifacts string, env []string, display string, phases ...string) (document, error) {
	if len(phases) != 1 || phases[0] != "structure" && phases[0] != "detail" {
		return nil, fmt.Errorf("plan phase must be structure or detail")
	}
	if display == "" {
		display = artifacts
	}
	input, err := read(filepath.Join(artifacts, "plan", "input.json"))
	if err != nil {
		return nil, err
	}
	previous, err := readOr(filepath.Join(rounds(artifacts), "latest.json"), document{"round": 0})
	if err != nil {
		return nil, err
	}
	phase := ""
	if len(phases) > 0 {
		phase = phases[0]
	}
	n := number(previous["round"]) + 1
	phaseRound := number(previous["phase_round"]) + 1
	if previous["phase"] != phase {
		phaseRound = 1
	}
	dir := filepath.Join(rounds(artifacts), fmt.Sprint(n))
	draft, err := readOr(filepath.Join(artifacts, "plan", "draft.json"), nil)
	if err != nil {
		return nil, err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return nil, err
	}
	base, err := g.MergeBase("")
	if err != nil {
		return nil, err
	}
	commit, err := commitSkeleton(ctx, text(object(input["requirements"])["goal"]))
	if err != nil {
		return nil, err
	}
	errors, err := draftErrors(draft, object(input["requirements"]), commit, phase)
	if err != nil {
		return nil, err
	}
	structureChanged := false
	if phase == "detail" {
		fixed, e := read(filepath.Join(artifacts, "plan", "structure.json"))
		if e != nil {
			return nil, e
		}
		paths, e := g.Changes(text(fixed["skeleton_commit"]), commit)
		if e != nil {
			return nil, e
		}
		for _, f := range paths.Files {
			if structuralFile(f.Status) {
				structureChanged = true
				errors = append(errors, "Restart plan for the required structure change: "+f.Status+" "+f.Path)
			}
		}
		if !reflect.DeepEqual(filter(entries(draft["decisions"]), func(d document) bool { return d["quality"] == "structure" }), entries(object(fixed["draft"])["decisions"])) {
			structureChanged = true
			errors = append(errors, "Restart plan to change the judged structure rows")
		}
	}
	if len(errors) == 0 {
		for _, row := range entries(draft["decisions"]) {
			if ref := text(row["ref"]); ref != "" && !slices.ContainsFunc(entries(input["agreed_decisions"]), func(d document) bool { return d["id"] == ref }) {
				errors = append(errors, "Unknown agreed decision: "+ref)
			}
		}
		for _, pending := range entries(input["pending_owner_rows"]) {
			if phase == "structure" && pending["quality"] != "structure" {
				continue
			}
			if !slices.ContainsFunc(entries(draft["decisions"]), func(d document) bool {
				return d["id"] == pending["id"] && d["decided_by"] == "owner" && d["when"] == "now"
			}) {
				errors = append(errors, "Pending owner row must be included: "+text(pending["id"]))
			}
		}
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	if len(c.SkeletonCheck) > 0 {
		childEnv := env
		if childEnv == nil {
			childEnv = os.Environ()
		}
		childEnv = append(slices.Clone(childEnv), "ARTIFACTS_DIR="+display, "BASE_BRANCH="+ctx.BaseBranch)
		gate, e := checks.RunChecks(checks.Options{Cwd: ctx.Cwd, ArtifactsDir: artifacts, BaseBranch: ctx.BaseBranch, Only: c.SkeletonCheck, Env: childEnv})
		if e != nil {
			message := e.Error()
			if len(message) > 4000 {
				message = message[len(message)-4000:]
			}
			errors = append(errors, "Skeleton checks could not run: "+message)
		} else if !gate.Passed {
			errors = append(errors, fmt.Sprintf("Skeleton checks did not pass: %s; results: %s", strings.Join(gate.Failing, ", "), filepath.Join(display, "checks", filepath.Base(gate.Results))))
		}
	}
	selected := []string{}
	if len(errors) == 0 {
		selected = qualitiesOf(draft)
		if phase == "structure" {
			selected = []string{"structure"}
		}
	}
	state := document{"run_id": ctx.RunID, "requirements_version": object(input["requirements"])["version"], "round": n, "phase": phase, "phase_round": phaseRound, "structure_changed": structureChanged, "valid": len(errors) == 0, "errors": errors, "skeleton_commit": commit, "base": base, "judges": []string{}}
	if state["valid"] == true {
		diff := filepath.Join(dir, "skeleton.patch")
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		patch, err := gitx.Run(ctx.Cwd, "diff", base, commit)
		if err != nil {
			return nil, err
		}
		if err = os.WriteFile(diff, []byte(patch), 0644); err != nil {
			return nil, err
		}
		facts := document{"before": filepath.Join(display, "plan", "facts-before.json"), "after": ""}
		if slices.Contains(selected, "structure") {
			after := filepath.Join(dir, "facts-after.json")
			facts["after"] = filepath.Join(rounds(display), fmt.Sprint(n), "facts-after.json")
			tracked, err := trackedAt(ctx.Cwd, commit)
			if err != nil {
				return nil, err
			}
			texts := requirementsTexts(input)
			for _, d := range entries(draft["decisions"]) {
				texts = append(texts, stringsOf(d["touches"])...)
			}
			paths := pathsIn(texts, tracked)
			f := factsOf(paths, tracked)
			f["commit"] = commit
			if err = write(after, f); err != nil {
				return nil, err
			}
		}
		changes, err := collectChanges(ctx, base, commit, c)
		if err != nil {
			return nil, err
		}
		if err = write(filepath.Join(dir, "changes.json"), changes); err != nil {
			return nil, err
		}
		if phase == "structure" {
			changed, e := g.Changes(base, commit)
			if e != nil {
				return nil, e
			}
			locality, e := judge.StructureFacts(g, base, commit, changed, draft)
			if e != nil {
				return nil, e
			}
			facts["locality"] = locality
		}
		skeleton := document{"base": base, "commit": commit, "diff": filepath.Join(rounds(display), fmt.Sprint(n), "skeleton.patch")}

		for _, change := range entries(changes["items"]) {
			q := text(change["quality"])
			if phase == "detail" && !slices.Contains(selected, q) {
				selected = append(selected, q)
			}
		}
		judges := []string{}
		for _, q := range stringsOf(input["qualities"]) {
			if phase == "structure" && q == "structure" || phase != "structure" && q != "structure" && (slices.Contains(selected, q) || q == "delivery") {
				judges = append(judges, q)
			}
		}
		state["judges"] = judges
		for _, q := range judges {
			if err = write(filepath.Join(dir, q+".json"), materials(q, input, draft, skeleton, facts, changes)); err != nil {
				return nil, err
			}
		}
		state["draft"] = draft
		state["changes"] = changes["items"]
		if phase == "structure" {
			for _, change := range entries(state["changes"]) {
				if change["quality"] != "structure" {
					continue
				}
				paths := []string{text(change["file"])}
				if old := text(change["oldPath"]); old != "" {
					paths = append(paths, old)
				}
				tree, e := g.Run(append([]string{"ls-tree", "-r", "-z", commit, "--"}, paths...)...)
				if e != nil {
					return nil, e
				}
				change["tree"] = tree
			}
		} else {
			fixed, e := read(filepath.Join(artifacts, "plan", "structure.json"))
			if e != nil {
				return nil, e
			}
			structure := filter(entries(fixed["changes"]), func(c document) bool { return c["quality"] == "structure" })
			state["changes"] = append(structure, filter(entries(state["changes"]), func(c document) bool { return c["quality"] != "structure" })...)
		}
	}
	if err = write(filepath.Join(dir, "state.json"), state); err != nil {
		return nil, err
	}
	if err = write(filepath.Join(rounds(artifacts), "latest.json"), state); err != nil {
		return nil, err
	}
	out := document{"round": n, "phase": phase, "phase_round": phaseRound, "structure_changed": structureChanged, "valid": state["valid"], "errors": errors, "skeleton_commit": commit, "materials": filepath.Join(rounds(display), fmt.Sprint(n))}
	for _, q := range stringsOf(input["qualities"]) {
		out[q] = slices.Contains(stringsOf(state["judges"]), q)
	}
	return out, nil
}

func structuralFile(status string) bool {
	return status != "" && strings.ContainsRune("ADRC", rune(status[0]))
}

func advance(ctx record.ContextData, artifacts string) (document, error) {
	state, err := read(filepath.Join(rounds(artifacts), "latest.json"))
	if err != nil {
		return nil, err
	}
	if state["phase"] != "structure" || state["judgments"] == nil {
		return nil, fmt.Errorf("the structure phase was not settled")
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return nil, err
	}
	outcome := outcomeOf(state, answers)
	proceed := outcome["outcome"] == "pass"
	if proceed {
		if err = write(filepath.Join(artifacts, "plan", "structure.json"), state); err != nil {
			return nil, err
		}
	}
	return document{"proceed": proceed}, nil
}
