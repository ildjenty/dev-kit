package proposal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPublicationUsesApprovedPRTextAndUpdatesExistingPR(t *testing.T) {
	for _, language := range []string{"en", "ja-JP"} {
		t.Run(language, func(t *testing.T) {
			cwd := testutil.Repo(t)
			testutil.Put(t, cwd, ".devkit/config.json", `{"language":"`+language+`","decisions":"choices","eval":"examples"}`)
			testutil.Git(t, cwd, "add", ".")
			testutil.Git(t, cwd, "commit", "-qm", "Configure proposal")
			testutil.Git(t, cwd, "branch", "base")
			testutil.Git(t, cwd, "checkout", "-qb", "refine/topic")
			testutil.Put(t, cwd, "AGENTS.md", "Employees only\n")
			testutil.Git(t, cwd, "add", ".")
			testutil.Git(t, cwd, "commit", "-qm", "Propose scope")
			testutil.PublicationRemote(t, cwd)
			log, _ := testutil.IssueCreationCLI(t)
			ctx := record.ContextData{Cwd: cwd, Dir: t.TempDir(), Branch: "refine/topic", BaseBranch: "base"}
			path := filepath.Join(ctx.Dir, "issue-creation/1.json")
			draft := map[string]any{
				"pr_title": "Record employee access", "pr_summary": "Put the employee scope on the base. Keep future work within the agreed audience.\n",
				"summary":   strings.Repeat("Internal summary ", 40),
				"parent":    map[string]any{"title": "Audience"},
				"decisions": []map[string]any{{"key": "N1", "title": "Employees only"}},
				"bindings":  []map[string]any{{"path": "AGENTS.md", "decision": "N1", "reason": "Constrain future delivery to employees"}},
				"units":     []map[string]any{{"key": "U1", "title": "Sign in", "blocked_by": []string{"TN1"}}},
				"topics":    []map[string]any{{"key": "TN1", "title": "Choose identity", "blocked_by": []string{}}},
			}
			content := map[string]any{"draft": draft, "mapping": map[string]string{"N1": "D-abcdef"}, "approval_text": "approve=5; versions; internal failure details"}
			env := cli.Env{Cwd: cwd}
			progress := testutil.Must(PublishProposal(ctx, env, content, path))
			read := func(name string) string { return string(testutil.Must(os.ReadFile(filepath.Join(log, name)))) }
			testutil.Equal(t, read("pr-title"), draft["pr_title"])
			testutil.Equal(t, strings.HasPrefix(read("pr-body"), draft["pr_summary"].(string)+"\n\n"), true)
			progress.Parent = &issues.Created{Number: 101, URL: "https://github.example/team/repo/issues/101"}
			progress.Created = map[string]issues.Created{"U1": {Number: 102, URL: "https://github.example/team/repo/issues/102"}, "TN1": {Number: 103, URL: "https://github.example/team/repo/issues/103"}}
			testutil.OK(t, issues.SaveCreation(path, progress))
			testutil.OK(t, UpdateProposal(ctx, env, content, progress))
			body := read("pr-body")
			testutil.Contains(t, body, "- `AGENTS.md` — D-abcdef Employees only: Constrain future delivery to employees")
			testutil.Contains(t, body, "[D-abcdef](https://github.example/team/repo/blob/refine%2Ftopic/choices/D-abcdef.md) Employees only")
			testutil.Contains(t, body, "- [#101 Audience](https://github.example/team/repo/issues/101)\n  - [#102 Sign in](https://github.example/team/repo/issues/102)")
			testutil.Contains(t, body, "[#103](https://github.example/team/repo/issues/103)")
			testutil.Contains(t, body, "[#103 Choose identity](https://github.example/team/repo/issues/103)")
			for _, unwanted := range []string{"approve=5", "versions", "internal failure details", "Internal summary"} {
				testutil.Equal(t, strings.Contains(body, unwanted), false)
			}
			if language == "en" {
				testutil.Contains(t, body, "## What to review\n")
				testutil.Contains(t, body, "## Decision records\n\nThese records are a log for people and do not bind delivery.")
				testutil.Contains(t, body, "## Issues created\n\nUnits become ready when this PR merges, subject to blocked-by dependencies.")
				testutil.Contains(t, body, " — blocked by: [#103]")
			} else {
				testutil.Contains(t, body, "## レビュー対象\n")
				testutil.Contains(t, body, "## 判断ログ\n\n判断ログは人のための記録であり、開発作業を拘束しません。")
				testutil.Contains(t, body, "## 作成した Issue\n\n作業単位は、この PR のマージ後、blocked-by の依存条件を満たすと開始可能になります。")
				testutil.Contains(t, body, " — 依存先: [#103]")
			}
			draft["pr_title"] = "Record the revised employee scope"
			draft["pr_summary"] = "Put the revised employee scope on the base. Preserve the revised delivery constraints."
			content["topic"] = map[string]any{"number": 7, "title": "Existing audience"}
			progress.Parent = &issues.Created{Number: 7}
			testutil.OK(t, issues.SaveCreation(path, progress))
			testutil.Must(PublishProposal(ctx, env, content, path))
			testutil.Equal(t, read("pr-title"), draft["pr_title"])
			testutil.Equal(t, strings.HasPrefix(read("pr-body"), draft["pr_summary"].(string)+"\n\n"), true)
			testutil.Contains(t, read("pr-body"), "- [#7 Existing audience](https://github.example/team/repo/issues/7)")
			testutil.Equal(t, strings.Count(read("calls"), "pr create"), 1)
			testutil.Equal(t, strings.Count(read("calls"), "pr edit"), 2)
		})
	}
}

func TestRetroBodyListsApprovedCorrectionsEscapesAndCases(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			cfg := config.Defaults("")
			cfg.Language = language
			cfg.Eval = "examples"
			content := map[string]any{"approval_text": "approval history", "draft": map[string]any{
				"pr_title": "Prevent missed failures", "pr_summary": "Add prevention rules and a regression case to the base. This prevents the traced failure from recurring.",
				"standards": []map[string]any{{"path": "policy/contract.md", "reason": "Require caller compatibility", "evidence": "The entering change broke callers", "entries": []string{"escape-1"}}},
				"checks":    []map[string]any{{"name": "compatibility", "reason": "Detect incompatible calls", "entries": []string{"escape-1"}}},
				"escapes":   []map[string]any{{"entry": "escape-1", "stage": "quality", "quality": "contract", "reason": "The caller was overlooked", "evidence": "The entering change removed an argument"}},
				"cases":     []map[string]any{{"entry": "escape-1", "name": "missing-argument"}},
			}}
			_, body, err := proposalText(content, issues.Creation{}, "retro/failure", "https://github.example/team/repo", cfg)
			testutil.OK(t, err)
			testutil.Contains(t, body, "- `policy/contract.md` — Require caller compatibility; The entering change broke callers (escape-1)")
			testutil.Contains(t, body, "- `.devkit/config.json` — `compatibility`: Detect incompatible calls (escape-1)")
			testutil.Contains(t, body, "- `escape-1` — quality / contract: The caller was overlooked; The entering change removed an argument")
			testutil.Contains(t, body, "[missing-argument](https://github.example/team/repo/tree/retro%2Ffailure/examples/judge/missing-argument) — `escape-1`")
			testutil.Equal(t, strings.Contains(body, "approval history"), false)
			if language == "en" {
				testutil.Contains(t, body, "## Escapes\n")
				testutil.Contains(t, body, "## Eval cases\n")
			} else {
				testutil.Contains(t, body, "## 流出した欠陥\n")
				testutil.Contains(t, body, "## 評価ケース\n")
			}
		})
	}
}
