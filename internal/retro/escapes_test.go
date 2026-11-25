package retro

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

const escapePath = ".devkit/backlog/escapes/abcdef.json"
const escapeID = escapePath + "#escape/0"
const casePath = "custom-eval/judge/escaped-contract"

func escapeFixture(t *testing.T) *fixture {
	r := setup(t, nil)
	r.json(".devkit/config.json", doc{"language": "en", "eval": "custom-eval"})
	r.json(escapePath, doc{"defect": "After merge, the owner noticed that Value returns the wrong result.", "entered": r.git("rev-parse", "HEAD"), "recorded_at": "2026-11-24T00:00:00Z"})
	r.commit()
	r.git("branch", "-f", "main", "HEAD")
	r.begin()
	return r
}

func (r *fixture) escapeDraft() doc {
	d := r.draft()
	d["units"] = []doc{}
	d["escapes"] = []doc{{"entry": escapeID, "stage": "quality", "quality": "contract", "reason": "The judge missed the changed return value", "evidence": "git show of entered: Value changed from 1 to 2", "judge_should_catch": true}}
	d["cases"] = []doc{{"entry": escapeID, "name": "escaped-contract"}}
	r.put(casePath+"/base/value.go", "package value\nfunc Value() int { return 1 }\n")
	r.put(casePath+"/change.patch", "--- a/value.go\n+++ b/value.go\n@@ -1,2 +1,2 @@\n package value\n-func Value() int { return 1 }\n+func Value() int { return 2 }\n")
	r.json(casePath+"/case.json", doc{"description": "A changed return value breaks callers", "expected": []doc{{"id": "wrong-value", "quality": "contract", "file": "value.go", "summary": "Callers require 1 but now receive 2"}, {"id": "missed-finding", "quality": "contract", "file": "value.go", "summary": "A second expected finding remains missed"}}})
	return d
}

func TestPrepareIncludesEscapeContentAndConsumptionPath(t *testing.T) {
	r := escapeFixture(t)
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	entry := items(input["entries"])[0]
	testutil.Equal(t, entry["id"], escapeID)
	testutil.Equal(t, entry["kind"], "escape")
	testutil.Equal(t, entry["path"], escapePath)
	testutil.Contains(t, str(obj(entry["content"])["defect"]), "After merge")
	testutil.Equal(t, obj(entry["content"])["entered"], r.git("rev-parse", "HEAD^"))
	testutil.Equal(t, obj(input["backlog"])[escapePath], string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, escapePath)))))
	testutil.Equal(t, caseFolder(input), "custom-eval/judge")
	for _, invalid := range []string{`{}`, `{"defect":"x","entered":"1","recorded_at":"invalid"}`} {
		r.put(escapePath, invalid)
		_, _, e := readBacklog(r.cwd)
		if e == nil {
			t.Fatal("accepted invalid escape")
		}
	}
}

func TestEscapeChecksRequireCoverageTraceAndValidCases(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*fixture, doc)
	}{
		{"unused", "neither used nor dismissed", func(r *fixture, d doc) {
			d["cases"] = []doc{}
			items(d["escapes"])[0]["judge_should_catch"] = false
			items(d["escapes"])[0]["stage"] = "check"
			testutil.OK(t, os.RemoveAll(filepath.Join(r.cwd, casePath)))
		}},
		{"missing trace", "requires a stage trace", func(r *fixture, d doc) { d["escapes"] = []doc{} }},
		{"missing case", "requires an evaluation case", func(r *fixture, d doc) {
			d["cases"] = []doc{}
			d["units"] = r.draft()["units"]
			testutil.OK(t, os.RemoveAll(filepath.Join(r.cwd, casePath)))
		}},
		{"invalid json", "case.json", func(r *fixture, d doc) { r.put(casePath+"/case.json", `{}`) }},
		{"invalid patch", "apply --index", func(r *fixture, d doc) { r.put(casePath+"/change.patch", "not a patch") }},
		{"absent finding file", "absent from base and change", func(r *fixture, d doc) {
			r.json(casePath+"/case.json", doc{"description": "Defect", "expected": []doc{{"id": "x", "quality": "contract", "file": "absent.go", "summary": "Missing"}}})
		}},
		{"empty expected", "must have an expected finding", func(r *fixture, d doc) {
			r.json(casePath+"/case.json", doc{"description": "Defect", "expected": []doc{}})
		}},
		{"wrong quality", "must expect the traced quality", func(r *fixture, d doc) { items(d["escapes"])[0]["quality"] = "data" }},
		{"unsafe name", "invalid or duplicate", func(r *fixture, d doc) { items(d["cases"])[0]["name"] = "../escape" }},
		{"duplicate case", "one used escape", func(r *fixture, d doc) { d["cases"] = append(items(d["cases"]), items(d["cases"])[0]) }},
		{"untraced case", "no escape trace", func(r *fixture, d doc) { r.put("custom-eval/judge/untraced/case.json", "{}") }},
		{"extra case file", "must be case.json", func(r *fixture, d doc) { r.put(casePath+"/extra.txt", "Extra") }},
		{"ignored base file", "must not be ignored", func(r *fixture, d doc) { r.put(casePath+"/base/.gitignore", "value.go\n") }},
		{"empty base", "survive Git commits", func(r *fixture, d doc) { testutil.OK(t, os.Remove(filepath.Join(r.cwd, casePath, "base/value.go"))) }},
		{"request gap", "cannot require a judge case", func(r *fixture, d doc) { items(d["escapes"])[0]["stage"] = "none" }},
		{"routing without correction", "traced concerns.json edit", func(r *fixture, d doc) { items(d["escapes"])[0]["stage"] = "routing" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := escapeFixture(t)
			d := r.escapeDraft()
			r.write(d)
			tc.change(r, d)
			testutil.Contains(t, r.errors(d), tc.want)
		})
	}
}

func TestRoutingTraceRequiresItsQualitySentenceToChange(t *testing.T) {
	r := escapeFixture(t)
	d := r.escapeDraft()
	items(d["escapes"])[0]["stage"] = "routing"
	path := ".devkit/standards/concerns.json"
	d["standards"] = []doc{{"path": path, "entries": []string{escapeID}, "reason": "Route changed return values", "evidence": "The entering diff was missed"}}
	mapping := testutil.Must(readJSON(filepath.Join(r.cwd, path)))
	r.put(path, string(testutil.Must(json.MarshalIndent(mapping, "", "    "))))
	r.write(d)
	testutil.Contains(t, r.errors(d), "changed routing sentence")
	obj(obj(mapping["qualities"])["contract"])["routing"] = "Changes to return values or caller-visible responses."
	r.json(path, mapping)
	testutil.Equal(t, r.errors(d), "")
}

func TestAdditionOnlyEscapeCasePreservesItsBaseDirectory(t *testing.T) {
	r := escapeFixture(t)
	d := r.escapeDraft()
	testutil.OK(t, os.Remove(filepath.Join(r.cwd, casePath, "base/value.go")))
	r.put(casePath+"/base/.gitkeep", "")
	r.put(casePath+"/change.patch", "--- /dev/null\n+++ b/value.go\n@@ -0,0 +1,2 @@\n+package value\n+func Value() int { return 2 }\n")
	r.write(d)
	testutil.Equal(t, r.errors(d), "")
}

func TestDismissedEscapeNeedsTraceButDoesNotCallProvider(t *testing.T) {
	r := escapeFixture(t)
	d := r.draft()
	d["units"] = []doc{}
	d["dismissed"] = []doc{{"entries": []string{escapeID}, "reason": "A genuine gap in the original request"}}
	d["escapes"] = []doc{{"entry": escapeID, "stage": "none", "quality": "", "reason": "No requirement covered this behavior", "evidence": "Neither local git nor gh can read the entering change", "judge_should_catch": false}}
	r.write(d)
	testutil.Equal(t, r.errors(d), "")
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(verify(r.ctx, r.artifacts, envVars(map[string]string{"DEVKIT_CLAUDE": "/missing/provider"})))["needed"], false)
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "question")
}

func fakeEscapeProvider(t *testing.T, r *fixture, failure string) cli.Env {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "provider")
	exe := testutil.Must(os.Executable())
	testutil.Put(t, filepath.Dir(fake), filepath.Base(fake), "#!/bin/sh\nexec '"+strings.ReplaceAll(exe, "'", "'\\''")+"' -test.run=TestEscapeProviderHelper -- \"$@\"\n")
	testutil.OK(t, os.Chmod(fake, 0700))
	vars := map[string]string{"DEVKIT_ESCAPE_HELPER": "1", "DEVKIT_ESCAPE_FAILURE": failure, "DEVKIT_CODEX": fake, "DEVKIT_CLAUDE": fake, "TYPESAFE_API_KEY": ""}
	env := cli.Env{Cwd: r.cwd, Environ: os.Environ(), Stdout: io.Discard, Stderr: io.Discard}
	for key, value := range vars {
		env.Environ = append(env.Environ, key+"="+value)
	}
	env.LookupEnv = func(key string) (string, bool) {
		if value, ok := vars[key]; ok {
			return value, true
		}
		return os.LookupEnv(key)
	}
	return env
}

func TestEscapeVerificationAndApprovalWithFakeProviders(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, failure := range []string{"", "contract", "matching"} {
			t.Run(provider+"/"+failure, func(t *testing.T) {
				r := escapeFixture(t)
				r.json(".devkit/config.json", doc{"language": "en", "eval": "custom-eval", "provider": provider})
				r.commit()
				r.git("branch", "-f", "main", "HEAD")
				testutil.Must(prepare(r.ctx, r.artifacts))
				d := r.escapeDraft()
				items(d["escapes"])[0]["stage"] = "routing"
				path := ".devkit/standards/authority.md"
				original := string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, path))))
				r.put(path, original+"\nProposed escape standard marker.\n")
				d["standards"] = []doc{{"path": path, "entries": []string{escapeID}, "reason": "Clarify the observed failure", "evidence": "Entered change"}}
				mapPath := ".devkit/standards/concerns.json"
				mapping := testutil.Must(readJSON(filepath.Join(r.cwd, mapPath)))
				obj(obj(mapping["qualities"])["contract"])["routing"] = "Proposed routing marker: changes to return values."
				r.json(mapPath, mapping)
				d["standards"] = append(items(d["standards"]), doc{"path": mapPath, "entries": []string{escapeID}, "reason": "Route the missed change", "evidence": "Entered diff"})
				r.write(d)
				testutil.Equal(t, r.errors(d), "")
				testutil.Must(r.review(judgment()))
				head := r.git("rev-parse", "HEAD")
				env := fakeEscapeProvider(t, r, failure)
				testutil.Equal(t, testutil.Must(verify(r.ctx, r.artifacts, env))["needed"], true)
				verification := testutil.Must(readJSON(file(r.artifacts, "verification.json")))
				result := obj(verification["result"])
				item := items(result["cases"])[0]
				text := "wrong-value: found; missed-finding: missed."
				if failure == "" {
					testutil.Equal(t, num(obj(result["totals"])["found"]), 1)
					testutil.Equal(t, num(obj(result["totals"])["missed"]), 1)
				} else {
					testutil.Contains(t, str(item["error"]), failure+":")
					text = "Provider verification failed (not scored): " + str(item["error"])
				}
				testutil.OK(t, writeJSON(file(r.artifacts, "verification-text.json"), doc{"text": text}))
				testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
				out := testutil.Must(finish(r.ctx, r.artifacts))
				testutil.Equal(t, out["outcome"], "question")
				testutil.Equal(t, str(items(out["questions"])[0]["text"]), readFile(t, file(r.artifacts, "approval.txt")))
				details := testutil.Must(readJSON(filepath.Join(r.ctx.Dir, "proposals/1.details.json")))
				testutil.Equal(t, obj(details["content"])["verification_report"], text)
				testutil.Equal(t, obj(details["content"])["verification"], verification)
				testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, path)))), "Proposed escape standard marker.")
				testutil.Equal(t, r.git("status", "--porcelain"), "")
				_, e := os.Stat(filepath.Join(r.cwd, casePath, "case.json"))
				testutil.OK(t, e)
				r.ctx.RunID = "r2"
				s, e := start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
				testutil.OK(t, e)
				testutil.Equal(t, s.Stopped, false)
				testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
				_, e = os.Stat(filepath.Join(r.cwd, escapePath))
				testutil.Equal(t, os.IsNotExist(e), true)
				for _, p := range []string{escapePath, casePath + "/base/value.go", casePath + "/change.patch", casePath + "/case.json", path} {
					testutil.Contains(t, r.git("show", "--format=", "--name-only", "HEAD"), p)
				}
				testutil.Equal(t, r.git("status", "--porcelain"), "")
			})
		}
	}
}

func TestEscapeCannotSkipVerificationOrChangeReviewedCase(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			r := escapeFixture(t)
			r.write(r.escapeDraft())
			testutil.Must(r.review(judgment()))
			want := "must verify the reviewed draft"
			if changed {
				r.put(casePath+"/base/extra.go", "package value\n")
				want = "checkout changed after the judgment"
			}
			out := testutil.Must(finish(r.ctx, r.artifacts))
			testutil.Equal(t, out["outcome"], "stop")
			testutil.Contains(t, strings.Join(stringsOf(out["reasons"]), "\n"), want)
			if changed {
				testutil.Contains(t, r.git("status", "--porcelain"), "extra.go")
			} else {
				testutil.Equal(t, r.git("status", "--porcelain"), "")
			}
		})
	}
}

func TestEscapeProviderHelper(t *testing.T) {
	if os.Getenv("DEVKIT_ESCAPE_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	prompt := string(testutil.Must(io.ReadAll(os.Stdin)))
	answerPath := ""
	if args[0] == "exec" {
		answerPath = args[8]
	}
	kind := os.Getenv("QUALITY")
	answer := doc{}
	if strings.HasPrefix(prompt, "Match evaluation findings.") {
		kind = "matching"
		answer = doc{"expected": doc{"wrong-value": "reported-1", "missed-finding": ""}, "not_expected": doc{}}
	} else if strings.Contains(prompt, "When Jev is unavailable") {
		kind = "route"
		artifact := os.Getenv("ARTIFACTS_DIR")
		target := testutil.Must(readJSON(filepath.Join(artifact, "judge/target.json")))
		mapping := testutil.Must(readJSON(filepath.Join(artifact, "criteria", str(target["standards"]), "concerns.json")))
		testutil.Contains(t, str(obj(obj(mapping["qualities"])["contract"])["routing"]), "Proposed routing marker")
		answer = doc{"qualities": []doc{{"quality": "contract", "file": "value.go", "reason": "Changed return value"}}}
	} else {
		artifact := os.Getenv("ARTIFACTS_DIR")
		target := testutil.Must(readJSON(filepath.Join(artifact, "judge/target.json")))
		standard := filepath.Join(artifact, "criteria", str(target["standards"]), "authority.md")
		testutil.Contains(t, string(testutil.Must(os.ReadFile(standard))), "Proposed escape standard marker.")
		findings := []doc{}
		if kind == "contract" {
			findings = append(findings, doc{"aspect": "contract", "file": "value.go", "line": "2", "blocking": true, "needs_decision": false, "deferred": false, "mechanical": false, "failure_scenario": "Wrong return value", "evidence": "return 2", "fix": "Return 1"})
		}
		verdict := "pass"
		if len(findings) > 0 {
			verdict = "fail"
		}
		answer = doc{"criteria": []doc{{"aspect": kind, "verdict": verdict, "checked": "Read the change", "not_checked": ""}}, "findings": findings}
	}
	if kind == os.Getenv("DEVKIT_ESCAPE_FAILURE") {
		fmt.Fprintln(os.Stderr, "Fake provider failure")
		os.Exit(1)
	}
	if answerPath != "" {
		testutil.OK(t, os.WriteFile(answerPath, testutil.Must(json.Marshal(answer)), 0600))
	} else {
		testutil.OK(t, json.NewEncoder(os.Stdout).Encode(doc{"type": "result", "subtype": "success", "is_error": false, "structured_output": answer}))
	}
	os.Exit(0)
}

func TestVerifyWithoutEscapesDoesNotCallProvider(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.write(r.draft())
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(verify(r.ctx, r.artifacts, envVars(nil)))["needed"], false)
	_, e := os.Stat(file(r.artifacts, "evaluation"))
	testutil.Equal(t, os.IsNotExist(e), true)
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "proposal"))), 1)
}
