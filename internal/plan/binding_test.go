package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func mutatePlanSkeleton(t *testing.T, repo, mutation string) {
	t.Helper()
	switch mutation {
	case "tracked", "staged", "committed":
		testutil.Put(t, repo, "src/entry.ext", "unjudged source\n")
		if mutation != "tracked" {
			testutil.Git(t, repo, "add", "src/entry.ext")
		}
		if mutation == "committed" {
			testutil.Git(t, repo, "commit", "-qm", "unjudged change")
		}
	case "untracked":
		testutil.Put(t, repo, "src/generated.ext", "unjudged generated source\n")
	case "deleted":
		testutil.OK(t, os.Remove(filepath.Join(repo, "src/entry.ext")))
	case "mode":
		testutil.OK(t, os.Chmod(filepath.Join(repo, "src/entry.ext"), 0755))
	case "none":
	default:
		t.Fatalf("unknown mutation: %s", mutation)
	}
}

func TestPlanSkeletonCheckProcess(t *testing.T) {
	mutation := os.Getenv("DEVKIT_PLAN_TEST_MUTATION")
	if mutation == "" {
		return
	}
	mutatePlanSkeleton(t, ".", mutation)
	if os.Getenv("DEVKIT_PLAN_TEST_FAIL") == "1" {
		os.Exit(1)
	}
}

func planBindingSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range roots {
		testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			out[path] = fmt.Sprintf("%o:%s", info.Mode(), data)
			return err
		}))
	}
	return out
}

func TestPlanCheckRejectsChangesLeftBySkeletonCheck(t *testing.T) {
	executable := must(os.Executable())
	for _, phase := range []string{"structure", "detail"} {
		for _, mutation := range []string{"tracked", "staged", "untracked", "deleted", "mode", "committed"} {
			t.Run(phase+"/"+mutation, func(t *testing.T) {
				ctx, a := repository(t, document{
					"checks":         []document{{"name": "skeleton", "command": fmt.Sprintf("%q -test.run=^TestPlanSkeletonCheckProcess$", executable), "timeout_seconds": 10}},
					"skeleton_check": []string{"skeleton"},
				})
				ctx.Dir = t.TempDir()
				inputFiles(t, a)
				if phase == "detail" {
					completeStructure(t, ctx, a)
				} else {
					testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
				}
				vars := map[string]string{"STATE_DIR": t.TempDir(), "WORKFLOW_ID": ctx.RunID, "ARTIFACTS_DIR": a, "PLAN_PHASE": phase}
				var out, stderr bytes.Buffer
				env := cli.Env{Cwd: ctx.Cwd, Stdout: &out, Stderr: &stderr, Environ: append(os.Environ(), "DEVKIT_PLAN_TEST_MUTATION="+mutation), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"plan", "check"}, env), 0)
				var checked document
				testutil.OK(t, json.Unmarshal(out.Bytes(), &checked))
				testutil.Equal(t, checked["valid"], false)
				testutil.Contains(t, fmt.Sprint(checked["errors"]), "skeleton changed")
				state := must(read(filepath.Join(rounds(a), "latest.json")))
				testutil.Equal(t, state["judges"], []string{})
				for _, name := range []string{"structure.json", "delivery.json", "skeleton.patch", "changes.json"} {
					_, err := os.Stat(filepath.Join(text(checked["materials"]), name))
					testutil.Equal(t, os.IsNotExist(err), true)
				}
				before := planBindingSnapshot(t, ctx.Cwd)
				head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
				must(settle(a, nil, "", "", nil))
				testutil.Equal(t, outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil)["outcome"], "stop")
				testutil.Equal(t, planBindingSnapshot(t, ctx.Cwd), before)
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
			})
		}
	}
}

func TestPlanCheckRetainsMachineFailureAfterMutation(t *testing.T) {
	executable := must(os.Executable())
	ctx, a := repository(t, document{
		"checks":         []document{{"name": "skeleton", "command": fmt.Sprintf("%q -test.run=^TestPlanSkeletonCheckProcess$", executable), "timeout_seconds": 10}},
		"skeleton_check": []string{"skeleton"},
	})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	env := append(os.Environ(), "DEVKIT_PLAN_TEST_MUTATION=tracked", "DEVKIT_PLAN_TEST_FAIL=1")
	checked := must(checkCommand(ctx, a, env, "", "structure"))
	testutil.Equal(t, checked["valid"], false)
	testutil.Contains(t, fmt.Sprint(checked["errors"]), "Skeleton checks did not pass")
	testutil.Contains(t, fmt.Sprint(checked["errors"]), "skeleton changed")
	checked = must(checkCommand(ctx, a, append(os.Environ(), "DEVKIT_PLAN_TEST_MUTATION=none"), "", "structure"))
	testutil.Equal(t, checked["valid"], true)
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "show", text(checked["skeleton_commit"])+":src/entry.ext"), "unjudged source")
	material := must(read(filepath.Join(text(checked["materials"]), "structure.json")))
	testutil.Equal(t, object(material["skeleton"])["commit"], checked["skeleton_commit"])
	testutil.Contains(t, string(must(os.ReadFile(filepath.Join(text(checked["materials"]), "skeleton.patch")))), "unjudged source")
}

func TestPlanFinishBindsPassToJudgedSkeleton(t *testing.T) {
	for _, mutation := range []string{"none", "tracked", "staged", "untracked", "deleted", "mode", "committed"} {
		for _, materialize := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/materialize=%t", mutation, materialize), func(t *testing.T) {
				ctx, a := repository(t, document{})
				var out, stderr bytes.Buffer
				vars := map[string]string{"STATE_DIR": t.TempDir(), "WORKFLOW_ID": ctx.RunID, "ARTIFACTS_DIR": a}
				env := cli.Env{Cwd: ctx.Cwd, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				ctx = must(record.Context(env))
				request(ctx.Dir)
				requirements(ctx.Dir, "r1")
				run(ctx.Dir, ctx.RunID)
				inputFiles(t, a)
				d := draft()
				if !materialize {
					entries(d["decisions"])[0]["decided_by"] = "provisional"
				}
				structure := structureDraft()
				structure["decisions"] = d["decisions"]
				testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structure))
				testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "structure"))["valid"], true)
				passed := string(must(json.Marshal(judgment(nil))))
				must(settle(a, map[string]string{"structure": passed}, "", "", nil))
				testutil.Equal(t, must(advance(ctx, a))["proceed"], true)
				testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), d))
				testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], true)
				must(settle(a, map[string]string{"delivery": passed}, "", "", nil))
				judged := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
				mutatePlanSkeleton(t, ctx.Cwd, mutation)
				before := planBindingSnapshot(t, ctx.Cwd, ctx.Dir, a)
				head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
				index := must(os.ReadFile(filepath.Join(ctx.Cwd, ".git/index")))
				code := cli.New("test", Commands()).Execute([]string{"plan", "finish"}, env)
				if mutation == "none" {
					testutil.Equal(t, code, 0)
					var result document
					testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
					testutil.Equal(t, result["outcome"], "pass")
					testutil.Equal(t, testutil.Git(t, ctx.Cwd, "diff", judged, "HEAD", "--", "src", ".devkit/config.json"), "")
					testutil.Equal(t, must(record.Latest(ctx.Dir, "plan"))["skeleton_commit"], result["skeleton_commit"])
					testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain"), "")
					return
				}
				testutil.Equal(t, code, 1)
				testutil.Contains(t, stderr.String(), "skeleton changed")
				testutil.Equal(t, out.String(), "")
				testutil.Equal(t, planBindingSnapshot(t, ctx.Cwd, ctx.Dir, a), before)
				testutil.Equal(t, must(os.ReadFile(filepath.Join(ctx.Cwd, ".git/index"))), index)
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
				testutil.Equal(t, len(must(decisions.List(ctx.Cwd)).Decisions), 0)
			})
		}
	}
}

func TestAgreeRowsPreservesCheckoutIndexAndHead(t *testing.T) {
	for _, amend := range []bool{false, true} {
		t.Run(fmt.Sprint(amend), func(t *testing.T) {
			ctx, _ := repository(t, document{})
			ctx.Dir = t.TempDir()
			requirements(ctx.Dir, "r1")
			run(ctx.Dir, ctx.RunID)
			if amend {
				testutil.Put(t, ctx.Cwd, "src/peer.ext", "judged signature\n")
				must(commitSkeleton(ctx, "test"))
			}
			base := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
			mutatePlanSkeleton(t, ctx.Cwd, "staged")
			mutatePlanSkeleton(t, ctx.Cwd, "untracked")
			index := testutil.Git(t, ctx.Cwd, "diff", "--cached")
			commit := must(agreeRows(ctx, draft()))
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "diff", base, commit, "--", "src"), "")
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "diff", "--cached"), index)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "ls-files", "--others", "--exclude-standard"), "src/generated.ext")
			testutil.Equal(t, len(must(decisions.List(ctx.Cwd)).Decisions), 0)
			testutil.Equal(t, commit, base)
		})
	}
}
