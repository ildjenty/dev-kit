package refine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestBindingChecksRunOnBase(t *testing.T) {
	for _, tc := range []struct {
		name, command, preparation, want string
		paths                            []string
		timeout                          float64
	}{
		{name: "passes after preparation regardless of empty paths", command: "test -f prepared && test \"$REFINE_CHECK_VALUE\" = supplied", paths: []string{}},
		{name: "passes regardless of unmatched paths", command: "true", paths: []string{"missing/**"}},
		{name: "failure", command: "echo base-failure; exit 1", want: "check added on base: fail"},
		{name: "skipped", command: "echo unavailable; exit 77", want: "check added on base: skip"},
		{name: "environment failure", command: "exit 75", want: "check added on base: error"},
		{name: "timeout", command: "sleep 10", timeout: 0.01, want: "check added on base: error"},
		{name: "missing command", command: "./missing-command", want: "check added on base: fail"},
		{name: "proposal checkout cannot supply the command", command: "./branch-only", want: "check added on base: fail"},
		{name: "preparation failure", command: "true", preparation: "exit 1", want: "base preparation:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := config.Check{Name: "existing", Command: "exit 1", TimeoutSeconds: 2}
			r := setup(t, doc{"checks": []config.Check{existing}})
			preparation := tc.preparation
			if preparation == "" {
				preparation = "touch prepared"
			}
			r.put(".archon/config.yaml", "worktree:\n  postCreate: ['"+preparation+"']\n")
			r.commit()
			r.git("branch", "-f", "main", "HEAD")
			if tc.name == "proposal checkout cannot supply the command" {
				r.put("branch-only", "#!/bin/sh\nexit 0\n")
				testutil.OK(t, os.Chmod(filepath.Join(r.cwd, "branch-only"), 0755))
				r.commit()
			}
			r.begin()
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 2
			}
			added := config.Check{Name: "added", Command: tc.command, TimeoutSeconds: timeout, Paths: tc.paths}
			r.put(".devkit/config.json", string(testutil.Must(json.Marshal(doc{"language": "en", "checks": []config.Check{existing, added}}))))
			d := draft()
			d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Enforce automated verification"}}
			r.writeDraft(d)
			head := r.git("rev-parse", "HEAD")
			env := cli.Env{Environ: append(os.Environ(), "REFINE_CHECK_VALUE=supplied")}
			checked := testutil.Must(check(r.ctx, r.artifacts, env))
			testutil.Equal(t, checked["valid"], tc.want == "")
			report := testutil.Must(readJSON(file(r.artifacts, "verification.json")))
			testutil.Equal(t, report, testutil.Must(readJSON(file(r.artifacts, "verification/1.json"))))
			if tc.want != "" {
				testutil.Contains(t, strings.Join(stringsOf(checked["errors"]), "\n"), tc.want)
				testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
				testutil.Equal(t, len(r.latest("proposal")), 0)
			} else {
				testutil.Equal(t, obj(r.latest("proposal")["content"])["verification"], report)
				testutil.Equal(t, r.git("status", "--porcelain"), "")
			}
			results := items(report["base_checks"])
			if tc.preparation == "" {
				testutil.Equal(t, len(results), 1)
				testutil.Equal(t, results[0]["name"], "added")
				log := readText(t, str(results[0]["log"]))
				if tc.name == "failure" {
					testutil.Contains(t, log, "base-failure")
				}
				if tc.name == "timeout" {
					testutil.Contains(t, log, "Timed out")
				}
			} else {
				testutil.Equal(t, len(results), 0)
				testutil.Contains(t, readText(t, file(r.artifacts, "checks/1/1-prepare-1.log")), "exit 1")
			}
			testutil.Equal(t, strings.Count(r.git("worktree", "list", "--porcelain"), "worktree "), 1)
			_, err := os.Stat(filepath.Join(r.cwd, "prepared"))
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestBindingChecksValidateConfigBeforeRunning(t *testing.T) {
	for _, settings := range []string{
		`{"language":"en","checks":"invalid"}`,
		`{"language":"en","checks":[{"name":"invalid","command":"true","timeout_seconds":0}]}`,
		`{"language":"fr","checks":[{"name":"invalid","command":"true","timeout_seconds":2}]}`,
	} {
		t.Run(settings, func(t *testing.T) {
			r := setup(t, nil)
			r.begin()
			r.put(".devkit/config.json", settings)
			d := draft()
			d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Register verification"}}
			r.writeDraft(d)
			head := r.git("rev-parse", "HEAD")
			testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))["valid"], false)
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
			_, err := os.Stat(file(r.artifacts, "verification.json"))
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestBindingChecksDetectChangesAndSkipUnchangedChecks(t *testing.T) {
	for _, command := range []string{"exit 1", "true"} {
		t.Run(command, func(t *testing.T) {
			r := setup(t, doc{"checks": []config.Check{{Name: "existing", Command: "exit 1", TimeoutSeconds: 2}}})
			r.put(".archon/config.yaml", "worktree:\n  postCreate: ['exit 1']\n")
			r.commit()
			r.begin()
			r.put(".devkit/config.json", string(testutil.Must(json.Marshal(doc{"language": "en", "checks": []config.Check{{Name: "existing", Command: command, TimeoutSeconds: 2}}}))))
			d := draft()
			d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Register verification"}}
			r.writeDraft(d)
			checked := testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))
			testutil.Equal(t, checked["valid"], command == "exit 1")
			if command == "true" {
				testutil.Contains(t, strings.Join(stringsOf(checked["errors"]), "\n"), "base preparation:")
			} else {
				_, err := os.Stat(file(r.artifacts, "verification.json"))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
		})
	}
}

func TestBindingCheckFailureSurvivesRoundAndRevision(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Register verification"}}
	r.put(".devkit/config.json", `{"language":"en","checks":[{"name":"added","command":"echo first-failure; exit 1","timeout_seconds":2}]}`)
	r.writeDraft(d)
	env := cli.Env{Environ: os.Environ()}
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, env))["valid"], false)
	first := testutil.Must(readJSON(file(r.artifacts, "verification/1.json")))
	log := str(items(first["base_checks"])[0]["log"])
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, judgment()))["valid"], false)
	entry := testutil.Must(readJSON(file(r.artifacts, "rounds/latest.json")))
	testutil.Contains(t, strings.Join(stringsOf(entry["errors"]), "\n"), "check added on base: fail")
	r.put(".devkit/config.json", `{"language":"en","checks":[{"name":"added","command":"true","timeout_seconds":2}]}`)
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, env))["valid"], true)
	testutil.Contains(t, readText(t, log), "first-failure")
	testutil.Equal(t, items(testutil.Must(readJSON(file(r.artifacts, "verification/2.json")))["base_checks"])[0]["status"], "pass")
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, env))["valid"], true)
	testutil.Equal(t, len(items(testutil.Must(readJSON(file(r.artifacts, "verification.json")))["base_checks"])), 1)
	before := testutil.Must(readJSON(file(r.artifacts, "baseline.json")))
	r.begin()
	r.writeDraft(d)
	r.artifact("approval.txt", "Reply approve=2 or answers")
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, env))["valid"], true)
	testutil.Equal(t, len(items(testutil.Must(readJSON(file(r.artifacts, "verification.json")))["base_checks"])), 1)
	after := testutil.Must(readJSON(file(r.artifacts, "baseline.json")))
	if before["commit"] == after["commit"] {
		t.Fatal("expected a new proposal commit after prepare")
	}
}

func TestBindingCheckCanRegisterWithoutBaseConfig(t *testing.T) {
	r := setup(t, nil)
	r.git("rm", ".devkit/config.json")
	r.commit()
	r.git("branch", "-f", "main", "HEAD")
	r.begin()
	r.put(".devkit/config.json", `{"checks":[{"name":"added","command":"true","timeout_seconds":2}]}`)
	d := draft()
	d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Register verification"}}
	r.writeDraft(d)
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))["valid"], true)
	testutil.Equal(t, items(testutil.Must(readJSON(file(r.artifacts, "verification.json")))["base_checks"])[0]["status"], "pass")
}
