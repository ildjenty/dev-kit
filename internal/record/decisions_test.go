package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/cli"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func citationRecord(t *testing.T, dir, kind string, n int, doc Document) {
	t.Helper()
	data := must(json.Marshal(doc))
	testutil.Put(t, dir, filepath.Join(folders[kind], fmt.Sprintf("%d.json", n)), string(data))
}

func supersededBase(t *testing.T) string {
	t.Helper()
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "---\nid: D-abcdef\ntitle: Old\nstatus: superseded\n---\nOld body\n")
	testutil.Put(t, repo, ".devkit/decisions/D-fedcba.md", "---\nid: D-fedcba\ntitle: New\nstatus: agreed\nsupersedes: [D-abcdef]\n---\nNew body\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "supersession")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	return repo
}

func TestBranchDecisionCitationsKeepStagesAndCitingItems(t *testing.T) {
	repo, dir := testutil.Repo(t), t.TempDir()
	citationRecord(t, dir, "request", 1, Document{"unit": "U-abcdef"})
	testutil.Put(t, repo, ".devkit/config.json", `{"units":"work"}`)
	testutil.Put(t, repo, "work/U-abcdef.md", "---\ndecisions: [D-abcdef]\n---\nD-999999 in unit body is not a decision source.\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "unit")
	citationRecord(t, dir, "requirements", 1, Document{"constraints": []string{"Old D-999999"}})
	citationRecord(t, dir, "requirements", 2, Document{"goal": "Keep D-123456", "completion_conditions": []Document{{"id": "C1", "text": "Uses D-abcdef"}}, "constraints": []string{"Source D-abcdef"}, "open_points": []Document{{"id": "O1", "text": "D-654321?"}}})
	citationRecord(t, dir, "plan", 1, Document{"decisions": []Document{{"id": "P0", "ref": "D-999999"}}})
	citationRecord(t, dir, "plan", 2, Document{"decisions": []Document{{"id": "P1", "ref": "D-fedcba", "chosen": "D-abcdef and D-abcdef"}, {"id": "P2", "evidence": []Document{{"text": "D-fedcba"}}}}, "increments": []Document{{"id": "U1", "summary": "D-999999"}}})
	expected := []decisions.Citation{
		{ID: "D-123456", Stage: "requirements", Items: []string{"goal"}},
		{ID: "D-654321", Stage: "requirements", Items: []string{"O1"}},
		{ID: "D-abcdef", Stage: "plan", Items: []string{"P1"}},
		{ID: "D-abcdef", Stage: "requirements", Items: []string{"U-abcdef", "C1", "constraints[1]"}},
		{ID: "D-fedcba", Stage: "plan", Items: []string{"P1", "P2"}},
	}
	testutil.Equal(t, must(BranchDecisionCitations(dir, repo, "")), expected)
	testutil.Put(t, repo, "work/U-abcdef.md", "---\ndecisions: [D-999999]\n---\nChanged checkout\n")
	testutil.Equal(t, must(BranchDecisionCitations(dir, repo, "feature/x")), expected)
}

func TestSupersessionTargetsEarliestCitingStageAndReturnedText(t *testing.T) {
	repo := supersededBase(t)
	for _, source := range []string{"unit", "requirements", "plan", "none"} {
		t.Run(source, func(t *testing.T) {
			dir := newRecord(t)
			request(dir)
			switch source {
			case "unit":
				citationRecord(t, dir, "request", 2, Document{"unit": "U-abcdef"})
				testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\ndecisions: [D-abcdef]\n---\nUnit\n")
			case "requirements":
				citationRecord(t, dir, "requirements", 1, Document{"constraints": []string{"D-abcdef"}})
			}
			if source != "none" {
				citationRecord(t, dir, "plan", 1, Document{"decisions": []Document{{"id": "P1", "ref": "D-abcdef"}}})
			}
			target := "requirements"
			if source == "plan" {
				target = "plan"
			}
			if source == "none" {
				target = ""
			}
			check := must(CheckBranchDecisions(dir, repo, "", "main"))
			testutil.Equal(t, check.Target, target)
			for _, workflow := range []string{"requirements", "plan", "implement", "judge", "fix", "pr", "sync", "refine", "retro"} {
				reasons := must(StartConditions(dir, workflow, ConditionOptions{Cwd: repo, BaseBranch: "main"}))
				hasSupersession := strings.Contains(strings.Join(reasons, "\n"), "was superseded")
				want := target != "" && workflow != "requirements" && workflow != "sync" && workflow != "refine" && workflow != "retro" && (target != "plan" || workflow != "plan")
				testutil.Equal(t, hasSupersession, want)
			}
			for _, workflow := range []string{"requirements", "plan"} {
				returned := must(Returned(ContextData{Cwd: repo, Dir: dir, BaseBranch: "main"}, workflow))
				if workflow != target {
					testutil.Equal(t, len(returned), 0)
					continue
				}
				testutil.Equal(t, len(returned), 1)
				data := string(must(json.Marshal(returned[0])))
				for _, text := range []string{"D-abcdef", "Old body", "Old", "D-fedcba", "New body", "New", "P1"} {
					testutil.Contains(t, data, text)
				}
				if source == "unit" {
					testutil.Contains(t, data, "U-abcdef")
				}
				if source == "requirements" {
					testutil.Contains(t, data, "constraints[1]")
				}
			}
		})
	}
}

func TestSupersessionPreventsStartWithoutWritingRun(t *testing.T) {
	repo, dir := supersededBase(t), afterPlan(t)
	citationRecord(t, dir, "requirements", 2, Document{"constraints": []string{"D-abcdef"}})
	out := must(Start(dir, "implement", StartOptions{Cwd: repo, BaseBranch: "main", RunID: "i1", BaseCommit: sha("c")}))
	testutil.Equal(t, out.Stopped, true)
	testutil.Contains(t, strings.Join(out.Remaining, "\n"), "run sync on this branch, then start requirements")
	testutil.Equal(t, len(must(List(dir, "run"))), 2)
	out = must(Start(dir, "requirements", StartOptions{Cwd: repo, BaseBranch: "main", RunID: "r2", BaseCommit: sha("c")}))
	testutil.Equal(t, out.Stopped, false)
}

func TestUnitStartRequiresAgreedCitationAndNamesMissingOrDoneUnit(t *testing.T) {
	for _, state := range []string{"agreed", "proposed", "superseded", "missing", "done"} {
		t.Run(state, func(t *testing.T) {
			repo, stateDir := supersededBase(t), t.TempDir()
			if state != "missing" {
				testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\nid: U-abcdef\ntitle: Unit\ndecisions: [D-fedcba]\ndepends_on: []\n---\nGoal\n")
			}
			if state == "proposed" {
				testutil.Put(t, repo, ".devkit/decisions/D-fedcba.md", "---\nid: D-fedcba\nstatus: proposed\nsupersedes: [D-abcdef]\n---\nNot agreed\n")
			}
			if state == "superseded" {
				testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\nid: U-abcdef\ntitle: Unit\ndecisions: [D-abcdef]\ndepends_on: []\n---\nGoal\n")
			}
			if state == "done" {
				testutil.Put(t, repo, ".devkit/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"other"}`)
			}
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "unit", "--allow-empty")
			dir := RecordDir(stateDir, "feature/x")
			citationRecord(t, dir, "request", 1, Document{"unit": "U-abcdef"})
			var out, stderr bytes.Buffer
			env := commandEnv(repo, map[string]string{"STATE_DIR": stateDir, "WORKFLOW_ID": "r1"}, &out, &stderr)
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
			var got StartOutput
			testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
			testutil.Equal(t, got.Stopped, state != "agreed")
			reason := map[string]string{"proposed": "decision D-fedcba which is not agreed", "superseded": "decision D-abcdef which is not agreed", "missing": "unit U-abcdef does not exist", "done": "unit U-abcdef is already done"}[state]
			if reason != "" {
				testutil.Contains(t, strings.Join(got.Remaining, "\n"), reason)
			}
		})
	}
}

func TestStartsEnforceSupersessionOnlyAfterTarget(t *testing.T) {
	repo := supersededBase(t)
	for _, target := range []string{"requirements", "plan", "none"} {
		for _, workflow := range []string{"requirements", "plan", "implement", "judge", "fix", "pr"} {
			t.Run(target+"/"+workflow, func(t *testing.T) {
				dir := afterPlan(t)
				if workflow == "fix" || workflow == "pr" {
					begin(dir, "judge", "j1")
					extra := Document{}
					outcome := "pass"
					if workflow == "fix" {
						extra = Document{"verdict": "fail", "findings": []Document{finding(true)}}
						outcome = "stop"
					}
					judgment(dir, "j1", "scope", extra)
					if outcome == "pass" {
						end(dir, "judge", "j1", Document{"outcome": "pass"})
					} else {
						end(dir, "judge", "j1", stopped())
					}
				}
				if target == "requirements" {
					req := must(Latest(dir, "requirements"))
					req["constraints"] = []string{"D-abcdef"}
					citationRecord(t, dir, "requirements", 1, req)
				}
				if target == "plan" {
					doc := must(Latest(dir, "plan"))
					doc["decisions"] = []Document{{"id": "P1", "ref": "D-abcdef"}}
					citationRecord(t, dir, "plan", 1, doc)
				}
				out := must(Start(dir, workflow, StartOptions{Cwd: repo, BaseBranch: "main", RunID: "new", Commit: sha("d"), BaseCommit: sha("c"), ChangeID: func() (*string, error) { s := strings.Repeat("a", 64); return &s, nil }}))
				want := target != "none" && workflow != "requirements" && (target != "plan" || workflow != "plan")
				testutil.Equal(t, out.Stopped, want)
				if want {
					testutil.Contains(t, strings.Join(out.Remaining, "\n"), "was superseded on main")
				}
			})
		}
	}
}

func TestMissingBaseDoesNotAddSupersessionRefusal(t *testing.T) {
	repo, dir := supersededBase(t), afterPlan(t)
	req := must(Latest(dir, "requirements"))
	req["constraints"] = []string{"D-abcdef"}
	citationRecord(t, dir, "requirements", 1, req)
	check := must(CheckBranchDecisions(dir, repo, "", "absent"))
	testutil.Contains(t, check.Note, "decision check skipped")
	testutil.Equal(t, check.Target, "")
	testutil.Equal(t, must(StartConditions(dir, "implement", ConditionOptions{Cwd: repo, BaseBranch: "absent"})), []string{})
}
