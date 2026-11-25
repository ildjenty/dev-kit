package pr

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func refusals(branch, baseBranch, head string, facts doc, clean bool, judgment doc) []string {
	out := []string{}
	if branch == "HEAD" || branch == baseBranch {
		out = append(out, "refusing to open a pull request from "+branch)
	}
	if head != facts["head"] {
		out = append(out, fmt.Sprintf("HEAD moved from %s after the facts were taken", text(facts["head"])))
	}
	if !clean {
		out = append(out, "the working tree has changes that are not committed")
	}
	if judgment == nil || object(judgment["target"])["commit"] != head || !passed(judgment) {
		out = append(out, "the latest report judgment did not pass on HEAD")
	}
	return out
}
func pushKind(remote, head string, isAncestor func(string, string) bool, recorded map[string]bool) string {
	return gitx.PushKind(remote, head, isAncestor, recorded)
}
func recordedCommitsOf(results, judgments []doc) map[string]bool {
	return record.RecordedCommitsOf(results, judgments)
}
func openPR(args []string, env cli.Env) error {
	if err := env.Require("ARTIFACTS_DIR"); err != nil {
		return err
	}
	ctx, err := context(env)
	if err != nil {
		return err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return err
	}
	out := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr"))
	facts, err := read(filepath.Join(out, "facts.json"))
	if err != nil {
		return err
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return err
	}
	var report doc
	for _, j := range judgments {
		if j["kind"] == "report" && j["run_id"] == ctx.RunID {
			report = j
		}
	}
	clean, err := g.Clean()
	if err != nil {
		return err
	}
	if refused := refusals(ctx.Branch, ctx.BaseBranch, head, facts, clean, report); len(refused) > 0 {
		return &record.RecordError{Message: "the pull request was not opened", Errors: refused}
	}
	state, err := optional(filepath.Join(out, "report-state.json"))
	if err != nil {
		return err
	}
	if !truth(state["passed"]) {
		return &record.RecordError{Message: "the pull request was not opened: the report must be judged after assembly"}
	}
	titleBytes, err := os.ReadFile(filepath.Join(out, "pr-title.txt"))
	if err != nil {
		return err
	}
	bodyBytes, err := os.ReadFile(filepath.Join(out, "pr.md"))
	if err != nil {
		return err
	}
	if !slices.Equal(stringsOf(report["materials"]), reportMaterials(titleBytes, bodyBytes)) {
		return &record.RecordError{Message: "the pull request was not opened: the title or body differs from the report judgment"}
	}
	results, err := record.List(ctx.Dir, "result")
	if err != nil {
		return err
	}
	url, head, err := g.Publish(env, gitx.Publication{Branch: ctx.Branch, BaseBranch: ctx.BaseBranch, Title: strings.TrimSpace(string(titleBytes)), Body: string(bodyBytes), State: "open", Recorded: recordedCommitsOf(results, judgments)}, func() (string, error) {
		return commitBacklog(ctx, judgments, results)
	})
	if err != nil {
		return err
	}
	if err = save(filepath.Join(out, "outcome.json"), doc{"outcome": "pass"}); err != nil {
		return err
	}
	return env.JSON(doc{"url": url, "branch": ctx.Branch, "commit": head})
}
