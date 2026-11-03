package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
)

func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "--quiet", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	return root
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkNamed(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q: %+v", name, report.Checks)
	return Check{}
}

func TestInitAndDoctor(t *testing.T) {
	ctx := context.Background()
	root := repository(t)
	writeFile(t, root, "AGENTS.md", "Existing repository instructions.\n")
	writeFile(t, root, "CLAUDE.md", "Existing Claude instructions.\n")
	cmd := exec.Command("git", "-C", root, "config", "core.hooksPath", "custom-hooks")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	confirmed := false
	result, err := Init(ctx, root, func(changes []Change) error {
		confirmed = true
		for _, name := range []string{"devkit.toml", ".claude/settings.json", ".codex/hooks.json", ".codex/config.toml", ".codex/rules/devkit.rules"} {
			found := false
			for _, change := range changes {
				if change.Path == name && len(change.After) != 0 {
					found = true
				}
			}
			if !found {
				t.Errorf("missing proposed change %s", name)
			}
			if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s was written before confirmation: %v", name, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed || len(result.Changes) == 0 || len(result.Guidance) == 0 {
		t.Fatalf("missing installation result: %+v", result)
	}
	for _, pair := range [][2]string{
		{"plans/default.json", ".devkit/plans/default.json"},
		{"checks/contract.md", ".devkit/checks/contract.md"},
		{"examples/contract/spec.json", ".devkit/checks/contract/spec.json"},
		{"rules/rules.md", ".devkit/rules.md"},
		{"skills/devkit/SKILL.md", ".agents/skills/devkit/SKILL.md"},
	} {
		if !bytes.Equal(readFile(t, root, ".devkit/kit/"+kit.Version+"/"+pair[0]), readFile(t, root, pair[1])) {
			t.Errorf("copied asset differs: %s", pair[1])
		}
	}
	if string(readFile(t, root, "AGENTS.md")) != "Existing repository instructions.\n" || string(readFile(t, root, "CLAUDE.md")) != "Existing Claude instructions.\n" {
		t.Fatal("repository instructions changed")
	}
	output, err := exec.Command("git", "-C", root, "config", "core.hooksPath").Output()
	if err != nil || string(output) != "custom-hooks\n" {
		t.Fatalf("git hook configuration changed: %q, %v", output, err)
	}
	report, err := Doctor(ctx, root, "current-session")
	if err != nil {
		t.Fatal(err)
	}
	check := checkNamed(t, report, "hook_session")
	if check.OK || !strings.Contains(check.Message, "/hooks") {
		t.Fatalf("missing hook trust guidance: %+v", check)
	}
	writeFile(t, root, ".devkit/run/sessions/current-session/2026-11-04T00:00:00Z-SessionStart.json", `{"session_id":"current-session","host":"codex","observed_at":"2026-11-04T00:00:00Z"}`)
	report, err = Doctor(ctx, root, "current-session")
	if err != nil || !report.Healthy() {
		t.Fatalf("installation is unhealthy: %+v, %v", report, err)
	}
	result, err = Init(ctx, root, func([]Change) error {
		t.Error("unchanged installation requested confirmation")
		return nil
	})
	if err != nil || len(result.Changes) != 0 {
		t.Fatalf("second init is not idempotent: %+v, %v", result.Changes, err)
	}
	writeFile(t, root, ".devkit/kit/"+kit.Version+"/rules/rules.md", "Modified fixed asset.\n")
	report, err = Doctor(ctx, root, "current-session")
	if err != nil || checkNamed(t, report, "kit_copy").OK {
		t.Fatalf("modified kit was accepted: %+v, %v", report, err)
	}
}

func TestInitPreservesExistingFilesAndSettings(t *testing.T) {
	root := repository(t)
	preserved := map[string]string{
		"devkit.toml":                       "kit_version = 'v1'\n[router]\nhuman_confirmation = ['custom']\n",
		".agents/skills/devkit/SKILL.md":    "Custom skill.\n",
		".devkit/plans/default.json":        `{"custom":true}`,
		".devkit/checks/contract.md":        "Custom aspect.\n",
		".devkit/rules.md":                  "Custom rules.\n",
		".claude/agents/devkit-judge.md":    "Custom judge.\n",
		".codex/agents/devkit-planner.toml": "name = 'custom-planner'\n",
	}
	for name, body := range preserved {
		writeFile(t, root, name, body)
	}
	writeFile(t, root, ".claude/settings.json", `{"custom_number":9007199254740993,"permissions":{"allow":["Read"],"deny":["Bash(git push *)"]},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"custom-stop"}]}]}}`)
	writeFile(t, root, ".codex/config.toml", "model = 'custom-model'\n[features]\ncustom = true\n")
	writeFile(t, root, ".codex/rules/devkit.rules", "# Existing rule.\nprefix_rule(pattern=[\"git\"], decision=\"prompt\")")
	writeFile(t, root, ".gitignore", "existing-ignore")
	if _, err := Init(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	for name, body := range preserved {
		if string(readFile(t, root, name)) != body {
			t.Errorf("existing file was overwritten: %s", name)
		}
	}
	settings := readFile(t, root, ".claude/settings.json")
	if !bytes.Contains(settings, []byte("9007199254740993")) || !bytes.Contains(settings, []byte("custom-stop")) || !bytes.Contains(settings, []byte("Bash(git push *)")) {
		t.Fatalf("existing JSON settings were lost: %s", settings)
	}
	var codex map[string]any
	if err := toml.Unmarshal(readFile(t, root, ".codex/config.toml"), &codex); err != nil {
		t.Fatal(err)
	}
	if codex["model"] != "custom-model" || codex["sandbox_mode"] != "workspace-write" || codex["features"].(map[string]any)["custom"] != true {
		t.Fatalf("TOML settings were not merged: %+v", codex)
	}
	if string(readFile(t, root, ".gitignore")) != "existing-ignore\n.devkit/run/\n" {
		t.Fatal("gitignore was not appended")
	}
	if !strings.HasPrefix(string(readFile(t, root, ".codex/rules/devkit.rules")), "# Existing rule.\n") {
		t.Fatal("existing Codex rules were lost")
	}
	result, err := Init(context.Background(), root, nil)
	if err != nil || len(result.Changes) != 0 {
		t.Fatalf("merged settings are not idempotent: %+v, %v", result.Changes, err)
	}
}

func TestInitDeclinedAndConflictingSettings(t *testing.T) {
	t.Run("declined", func(t *testing.T) {
		root := repository(t)
		declined := errors.New("installation declined")
		_, err := Init(context.Background(), root, func([]Change) error { return declined })
		if !errors.Is(err, declined) {
			t.Fatalf("confirmation error was lost: %v", err)
		}
		for _, name := range []string{"devkit.toml", ".claude/settings.json", ".agents/skills/devkit/SKILL.md", ".gitignore"} {
			if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("declined change was applied: %s: %v", name, err)
			}
		}
	})
	t.Run("conflict", func(t *testing.T) {
		root := repository(t)
		body := "sandbox_mode = 'read-only'\n"
		writeFile(t, root, ".codex/config.toml", body)
		_, err := Init(context.Background(), root, nil)
		if err == nil || !strings.Contains(err.Error(), "sandbox_mode") {
			t.Fatalf("conflict was not reported: %v", err)
		}
		if string(readFile(t, root, ".codex/config.toml")) != body {
			t.Fatal("conflicting setting was overwritten")
		}
	})
}

func TestHookMatcherDoesNotHideRequiredRegistration(t *testing.T) {
	before := []byte(`{"hooks":{"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"devkit hook PreToolUse"}]}]}}`)
	fragment := []byte(`{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","command":"devkit hook PreToolUse"}]}]}}`)
	after, err := mergeJSON(before, fragment)
	if err != nil || bytes.Equal(before, after) {
		t.Fatalf("required matcher was not installed: %s, %v", after, err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(after, &settings); err != nil || len(settings.Hooks["PreToolUse"]) != 2 {
		t.Fatalf("existing matcher was not preserved: %s, %v", after, err)
	}
	for _, matcher := range []string{"Read", ".*"} {
		found := false
		for _, registration := range settings.Hooks["PreToolUse"] {
			for _, hook := range registration.Hooks {
				if registration.Matcher == matcher && hook.Type == "command" && hook.Command == "devkit hook PreToolUse" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("missing registration for matcher %q: %s", matcher, after)
		}
	}
}

func TestDoctorVersionsAndSessionScope(t *testing.T) {
	root := repository(t)
	ctx := context.Background()
	if _, err := Init(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".devkit/run/sessions/other-session/time-SessionStart.json", `{}`)
	report, err := Doctor(ctx, root, "current-session")
	if err != nil || checkNamed(t, report, "hook_session").OK {
		t.Fatalf("another session substituted for current observation: %+v, %v", report, err)
	}
	const workID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	store := fsstore.New(filepath.Join(root, ".devkit/run/work", workID))
	if err := store.AppendRecord(workID, string(ledger.KindWork), ledger.Work{ID: workID, TaskRef: "task.md", Branch: "main", BaseCommit: "base", KitVersion: "v0"}); err != nil {
		t.Fatal(err)
	}
	report, err = Doctor(ctx, root, "current-session")
	if err != nil || checkNamed(t, report, "work_version/"+workID).OK || !strings.Contains(checkNamed(t, report, "work_version/"+workID).Message, "unreadable kit version") {
		t.Fatalf("incompatible Work was accepted: %+v, %v", report, err)
	}
	writeFile(t, root, "devkit.toml", "kit_version = 'v0'\n")
	report, err = Doctor(ctx, root, "current-session")
	if err != nil || checkNamed(t, report, "kit_version").OK {
		t.Fatalf("incompatible configuration was accepted: %+v, %v", report, err)
	}
	if _, err := Init(ctx, root, nil); err == nil {
		t.Fatal("init accepted a different kit version")
	}
}

func TestInitRepairsNegatedIgnore(t *testing.T) {
	root := repository(t)
	ctx := context.Background()
	if _, err := Init(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	const before = ".devkit/run/\n!.devkit/run/\n"
	writeFile(t, root, ".gitignore", before)
	report, err := Doctor(ctx, root, "")
	if err != nil || checkNamed(t, report, "gitignore").OK {
		t.Fatalf("negated ignore was accepted: %+v, %v", report, err)
	}
	confirmed := false
	if _, err := Init(ctx, root, func(changes []Change) error {
		for _, change := range changes {
			if change.Path == ".gitignore" && string(change.Before) == before && string(change.After) == before+".devkit/run/\n" {
				confirmed = true
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Fatal("ignore repair was not proposed")
	}
	cmd := exec.Command("git", "-C", root, "check-ignore", "--quiet", "--", ".devkit/run/work/record.json")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runtime records are not ignored: %v: %s", err, output)
	}
	report, err = Doctor(ctx, root, "")
	if err != nil || !checkNamed(t, report, "gitignore").OK {
		t.Fatalf("repaired ignore was rejected: %+v, %v", report, err)
	}
	result, err := Init(ctx, root, nil)
	if err != nil || len(result.Changes) != 0 {
		t.Fatalf("ignore repair is not idempotent: %+v, %v", result.Changes, err)
	}
}

func TestWriteConfigPublishesAndCleansUp(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	writeFile(t, root, "settings.json", "Original settings.\n")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(root, "previous.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeConfig(path, []byte("Updated settings.\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(readFile(t, root, "settings.json")) != "Updated settings.\n" || info.Mode().Perm() != 0o600 {
		t.Fatal("configuration contents or permissions were not preserved")
	}
	if string(readFile(t, root, "previous.json")) != "Original settings.\n" {
		t.Fatal("existing configuration was rewritten in place")
	}
	if err := writeConfig(root, []byte("Cannot replace a directory.\n")); err == nil {
		t.Fatal("configuration publication unexpectedly replaced a directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 || entries[0].Name() != "previous.json" || entries[1].Name() != "settings.json" {
		t.Fatalf("temporary files remain: %+v, %v", entries, err)
	}
}

func TestDoctorValidatesLedgerRecords(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   string
		body   string
		reason string
	}{
		{"valid", "", "", ""},
		{"incomplete-work", "work", `{"kit_version":"v1"}`, `record field "id" is required`},
		{"unknown-kind", "unknown", `{}`, "unknown record kind"},
		{"invalid-lesson", "lesson", `{"content":"","evidence":[]}`, "required record field is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := repository(t)
			ctx := context.Background()
			if _, err := Init(ctx, root, nil); err != nil {
				t.Fatal(err)
			}
			const workID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			const recordID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
			store := fsstore.New(filepath.Join(root, ".devkit/run/work", workID))
			if err := store.AppendRecord(workID, "work", ledger.Work{ID: workID, TaskRef: "task.md", Branch: "main", BaseCommit: "base", KitVersion: kit.Version}); err != nil {
				t.Fatal(err)
			}
			if tc.kind != "" {
				if err := store.AppendRecord(recordID, tc.kind, json.RawMessage(tc.body)); err != nil {
					t.Fatal(err)
				}
			}
			report, err := Doctor(ctx, root, "")
			if err != nil {
				t.Fatal(err)
			}
			check := checkNamed(t, report, "work_version/"+workID)
			if tc.reason == "" {
				if !check.OK {
					t.Fatalf("valid records were rejected: %+v", check)
				}
			} else if check.OK || !strings.Contains(check.Message, recordID+".json") || !strings.Contains(check.Message, tc.reason) {
				t.Fatalf("unreadable record was not identified: %+v", check)
			}
		})
	}
}

func TestDoctorUsesLatestSessionStart(t *testing.T) {
	root := t.TempDir()
	const sessionID = "current-session"
	const prefix = ".devkit/run/sessions/" + sessionID + "/"
	const observation = `{"session_id":"current-session"}`
	writeFile(t, root, prefix+"20261104T000000.000000000Z-SessionStart.json", observation)
	latest := prefix + "20261104T000001.000000000Z-SessionStart.json"
	for _, body := range []string{`{`, `{"session_id":"other-session"}`, observation} {
		writeFile(t, root, latest, body)
		report := Report{Root: root}
		report.checkSession(sessionID)
		if check := checkNamed(t, report, "hook_session"); check.OK != (body == observation) {
			t.Fatalf("latest observation was not validated: %s: %+v", body, check)
		}
	}
}
