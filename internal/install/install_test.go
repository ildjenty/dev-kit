package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/assets"
	"github.com/ildjenty/dev-kit/internal/config"
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
	for _, args := range [][]string{
		{"config", "user.name", "Devkit test"},
		{"config", "user.email", "devkit@example.test"},
		{"config", "commit.gpgsign", "false"},
		{"config", "core.hooksPath", t.TempDir()},
	} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
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
	if !bytes.Contains(readFile(t, root, "devkit.toml"), []byte(`kit_version = "v3"`)) {
		t.Fatal("init did not write kit version v3")
	}
	wantGuidance := "The bundled default and fix templates require command:test. If it is not registered, the first work creates the tests and registers the command under [commands.test] in devkit.toml, and a human confirms the registration."
	if !slices.Contains(result.Guidance, wantGuidance) {
		t.Fatalf("missing first-work test guidance: %v", result.Guidance)
	}
	for _, pair := range [][2]string{
		{"principles/harness.md", ".devkit/principles/harness.md"},
		{"stages/implement.md", ".devkit/stages/implement.md"},
		{"templates/brief/work.md.tmpl", ".devkit/templates/brief/work.md.tmpl"},
		{"plans/default.json", ".devkit/plans/default.json"},
		{"principles/review/contract.md", ".devkit/principles/review/contract.md"},
		{"rules/rules.md", ".devkit/rules.md"},
		{"skills/devkit/SKILL.md", ".agents/skills/devkit/SKILL.md"},
	} {
		want, err := fs.ReadFile(assets.FS(), pair[0])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, readFile(t, root, pair[1])) {
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
	for _, check := range report.Checks {
		if check.Name == ".devkit/requirements.md" || check.Name == ".devkit/structure.json" {
			t.Fatalf("doctor required a product that work creates: %+v", check)
		}
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
	writeFile(t, root, ".devkit/rules.md", "Repository-owned rules.\n")
	report, err = Doctor(ctx, root, "current-session")
	if err != nil || !report.Healthy() {
		t.Fatalf("repository-owned norm edit was rejected: %+v, %v", report, err)
	}
	for _, check := range report.Checks {
		if check.Name == "kit_copy" {
			t.Fatal("doctor still checks a kit copy")
		}
	}
	for _, name := range []string{".devkit/kit", ".devkit/checks"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("init created a non-distributable path: %s: %v", name, err)
		}
	}
	if bytes.Contains(readFile(t, root, "devkit.toml"), []byte("[paths]")) {
		t.Fatal("init wrote path defaults explicitly")
	}
}

func TestInitPreservesExistingFilesAndSettings(t *testing.T) {
	root := repository(t)
	preserved := map[string]string{
		"devkit.toml":                           "kit_version = 'v3'\n[router]\nhuman_confirmation = ['dependency_changed']\n",
		".agents/skills/devkit/SKILL.md":        "Custom skill.\n",
		".devkit/plans/default.json":            `{"custom":true}`,
		".devkit/principles/review/contract.md": "Custom criteria.\n",
		".devkit/rules.md":                      "Custom rules.\n",
		".claude/agents/devkit-judge.md":        "Custom judge.\n",
		".codex/agents/devkit-planner.toml":     "name = 'custom-planner'\n",
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
		if name == "devkit.toml" {
			configuration, err := config.Parse(readFile(t, root, name))
			if err != nil || !slices.Equal(configuration.Router.HumanConfirmation, []string{"dependency_changed"}) {
				t.Fatalf("existing devkit settings were lost: %+v, %v", configuration, err)
			}
			continue
		}
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
	if string(readFile(t, root, ".gitignore")) != "existing-ignore\n.devkit/run/\n.devkit/worktrees/\n" {
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
		for _, name := range []string{"devkit.toml", ".devkit/kit", ".devkit/principles", ".claude/settings.json", ".agents/skills/devkit/SKILL.md", ".gitignore"} {
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
	if err := store.AppendRecord(workID, string(ledger.KindWork), ledger.Work{ID: workID, TaskRef: "task.md", Branch: "main", BaseCommit: "base", KitVersion: "v1"}); err != nil {
		t.Fatal(err)
	}
	report, err = Doctor(ctx, root, "current-session")
	check := checkNamed(t, report, "work_version/"+workID)
	if err != nil || check.OK || !strings.Contains(check.Message, "kit version") || !strings.Contains(check.Message, `"v1"`) || !strings.Contains(check.Message, `"v3"`) {
		t.Fatalf("incompatible Work was accepted: %+v, %v", report, err)
	}
	writeFile(t, root, "devkit.toml", "kit_version = 'v1'\n")
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
			if change.Path == ".gitignore" && string(change.Before) == before && string(change.After) == before+".devkit/run/\n.devkit/worktrees/\n" {
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
	if err := writeConfig(path, []byte("Original settings.\n")); err != nil {
		t.Fatal(err)
	}
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
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeConfig(directory, []byte("Cannot replace a directory.\n")); err == nil {
		t.Fatal("configuration publication unexpectedly replaced a directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 3 || entries[0].Name() != "directory" || entries[1].Name() != "previous.json" || entries[2].Name() != "settings.json" {
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
		{"incomplete-work", "work", `{"kit_version":"v3"}`, `record field "id" is required`},
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
	const observation = `{"host":"codex","session_id":"current-session","observed_at":"2026-11-04T00:00:00Z"}`
	writeFile(t, root, prefix+"20261104T000000.000000000Z-SessionStart.json", observation)
	latest := prefix + "20261104T000001.000000000Z-SessionStart.json"
	for _, tc := range []struct {
		name string
		body string
		ok   bool
	}{
		{"malformed", `{`, false},
		{"different-session", `{"host":"codex","session_id":"other-session","observed_at":"2026-11-04T00:00:00Z"}`, false},
		{"missing-host", `{"session_id":"current-session","observed_at":"2026-11-04T00:00:00Z"}`, false},
		{"invalid-host", `{"host":123,"session_id":"current-session","observed_at":"2026-11-04T00:00:00Z"}`, false},
		{"missing-time", `{"host":"codex","session_id":"current-session"}`, false},
		{"invalid-time", `{"host":"codex","session_id":"current-session","observed_at":"invalid"}`, false},
		{"null-time", `{"host":"codex","session_id":"current-session","observed_at":null}`, false},
		{"invalid-work-id", `{"host":"codex","session_id":"current-session","observed_at":"2026-11-04T00:00:00Z","work_id":123}`, false},
		{"unbound", observation, true},
		{"bound", `{"host":"codex","session_id":"current-session","observed_at":"2026-11-04T00:00:00Z","work_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeFile(t, root, latest, tc.body)
			report := Report{Root: root}
			report.checkSession(sessionID)
			if check := checkNamed(t, report, "hook_session"); check.OK != tc.ok {
				t.Fatalf("latest observation was not validated: %s: %+v", tc.body, check)
			}
		})
	}
}

func TestInitMergesDenyDefaults(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			root := repository(t)
			if existing {
				writeFile(t, root, "devkit.toml", "kit_version = \""+kit.Version+"\"\n[deny]\nrules = [\"CustomTool\"]\njudge = [\"CustomWrite\"]\n")
			}
			if _, err := Init(context.Background(), root, nil); err != nil {
				t.Fatal(err)
			}
			configuration, err := config.Parse(readFile(t, root, "devkit.toml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range []string{"Bash(git push *)", "Agent(fork)"} {
				if !slices.Contains(configuration.Deny.Rules, rule) {
					t.Errorf("missing default rule %q", rule)
				}
			}
			for _, tool := range []string{"apply_patch", "Edit", "Write", "NotebookEdit"} {
				if !slices.Contains(configuration.Deny.Judge, tool) {
					t.Errorf("missing judge tool %q", tool)
				}
			}
			if existing && (!slices.Contains(configuration.Deny.Rules, "CustomTool") || !slices.Contains(configuration.Deny.Judge, "CustomWrite")) {
				t.Fatal("existing deny settings were replaced")
			}
			before := readFile(t, root, "devkit.toml")
			result, err := Init(context.Background(), root, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Changes) != 0 || !bytes.Equal(before, readFile(t, root, "devkit.toml")) {
				t.Fatal("repeated init changed settings")
			}
		})
	}
}

func TestUninstallReversesInit(t *testing.T) {
	root := repository(t)
	settings := "{\"model\":\"x\",\"permissions\":{\"allow\":[\"Bash(ls)\"]}}\n"
	writeFile(t, root, ".claude/settings.json", settings)
	writeFile(t, root, ".gitignore", "node_modules/\n")
	if _, err := Init(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".devkit/rules.md", "# My rules\n")
	confirmed := false
	result, err := Uninstall(context.Background(), root, func([]Change) error {
		confirmed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed || len(result.Changes) == 0 {
		t.Fatalf("uninstall did not confirm its changes: %+v", result)
	}
	if !slices.Equal(result.Preserved, []string{".devkit/rules.md"}) {
		t.Fatalf("preserved = %v", result.Preserved)
	}
	var got, want map[string]any
	if err := json.Unmarshal(readFile(t, root, ".claude/settings.json"), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(settings), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("settings were not restored: %v", got)
	}
	if got := string(readFile(t, root, ".gitignore")); got != "node_modules/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	for _, name := range []string{"devkit.toml", ".devkit/kit", ".devkit/run", ".devkit/plans", ".devkit/principles", ".devkit/stages", ".devkit/templates", ".agents", ".codex", ".claude/skills", ".claude/agents"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains after uninstall: %v", name, err)
		}
	}
	again, err := Uninstall(context.Background(), root, nil)
	if err != nil || len(again.Changes) != 0 {
		t.Fatalf("second uninstall is not idempotent: %+v, %v", again.Changes, err)
	}
}

func TestUninstallKeepsUserConfig(t *testing.T) {
	root := repository(t)
	if _, err := Init(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	config := string(readFile(t, root, "devkit.toml"))
	writeFile(t, root, "devkit.toml", config+"\n[router]\nhuman_confirmation = [\"dependency_changed\"]\n")
	result, err := Uninstall(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Preserved, "devkit.toml") {
		t.Fatalf("devkit.toml with user settings was not preserved: %v", result.Preserved)
	}
	if _, err := os.Stat(filepath.Join(root, "devkit.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestInitCommitsOnlyWrittenPaths(t *testing.T) {
	for _, existingHead := range []bool{false, true} {
		t.Run(fmt.Sprint(existingHead), func(t *testing.T) {
			ctx := context.Background()
			root := repository(t)
			git := func(args ...string) string {
				t.Helper()
				output, err := runGit(ctx, root, args...)
				if err != nil {
					t.Fatal(err)
				}
				return strings.TrimSpace(string(output))
			}
			if existingHead {
				writeFile(t, root, "README.md", "Existing repository.\n")
				git("add", "README.md")
				git("commit", "-m", "Existing commit")
			}
			writeFile(t, root, "staged.txt", "Unrelated staged content.\n")
			writeFile(t, root, "untracked.txt", "Unrelated untracked content.\n")
			writeFile(t, root, ".devkit/principles/harness.md", "Repository-owned principle.\n")
			git("add", "staged.txt")
			result, err := Init(ctx, root, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := git("log", "-1", "--format=%s"); got != "devkit: init" {
				t.Fatalf("unexpected init commit message: %q", got)
			}
			paths := strings.Split(git("diff-tree", "--root", "--no-commit-id", "--name-only", "-r", "HEAD"), "\n")
			want := make([]string, 0, len(result.Changes))
			for _, change := range result.Changes {
				want = append(want, change.Path)
			}
			slices.Sort(want)
			if !slices.Equal(paths, want) {
				t.Fatalf("init committed paths it did not write: got %v, want %v", paths, want)
			}
			if got := git("diff", "--cached", "--name-only"); got != "staged.txt" {
				t.Fatalf("unrelated staged content changed: %q", got)
			}
			before := git("rev-parse", "HEAD")
			if _, err := Init(ctx, root, nil); err != nil {
				t.Fatal(err)
			}
			if after := git("rev-parse", "HEAD"); before != after {
				t.Fatal("unchanged init created another commit")
			}
		})
	}
}

func TestInitNoCommitAndDoctorCommitState(t *testing.T) {
	ctx := context.Background()
	root := repository(t)
	if _, err := Init(ctx, root, nil, InitOptions{NoCommit: true}); err != nil {
		t.Fatal(err)
	}
	report, err := Doctor(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if checkNamed(t, report, "HEAD").OK || checkNamed(t, report, "init_committed").OK {
		t.Fatal("doctor accepted initialization without a commit")
	}
	if _, err := runGit(ctx, root, "add", "--", "devkit.toml", ".devkit/plans", ".claude/agents", ".codex/agents", ".agents/skills"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(ctx, root, "commit", "-m", "Commit initialization files"); err != nil {
		t.Fatal(err)
	}
	report, err = Doctor(ctx, root, "")
	if err != nil || !checkNamed(t, report, "HEAD").OK || !checkNamed(t, report, "init_committed").OK {
		t.Fatalf("doctor rejected committed initialization files: %+v, %v", report, err)
	}
	for _, name := range []string{"devkit.toml", ".devkit/plans/default.json", ".devkit/plans/custom.json", ".claude/agents/devkit-judge.md", ".codex/agents/devkit-worker.toml"} {
		t.Run(name, func(t *testing.T) {
			before, err := readOptional(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, root, name, string(before)+"\n")
			report, err := Doctor(ctx, root, "")
			if err != nil || checkNamed(t, report, "init_committed").OK {
				t.Fatalf("doctor accepted uncommitted initialization file %s: %+v, %v", name, report, err)
			}
			if before == nil {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, root, name, string(before))
			}
		})
	}
}

func TestConfiguredNormPathsAndUninstall(t *testing.T) {
	ctx := context.Background()
	root := repository(t)
	writeFile(t, root, "devkit.toml", `kit_version = "v3"
[sources.principles]
path = "norms/principles"
[sources.stages]
path = "norms/stages"
[sources.plans]
path = "norms/plans"
[sources.templates]
path = "norms/templates"
[sources.rules]
path = "norms/rules.md"
[sources.requirements]
path = "records/requirements.md"
[sources.structure]
path = "records/structure.json"
`)
	writeFile(t, root, "norms/principles/harness.md", "Custom principle.\n")
	if _, err := Init(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"principles/testing.md", "norms/principles/testing.md"},
		{"stages/implement.md", "norms/stages/implement.md"},
		{"principles/review/contract.md", "norms/principles/review/contract.md"},
		{"plans/default.json", "norms/plans/default.json"},
		{"templates/brief/work.md.tmpl", "norms/templates/brief/work.md.tmpl"},
		{"rules/rules.md", "norms/rules.md"},
	} {
		want, err := fs.ReadFile(assets.FS(), pair[0])
		if err != nil || !bytes.Equal(want, readFile(t, root, pair[1])) {
			t.Fatalf("configured norm differs: %s: %v", pair[1], err)
		}
	}
	for _, name := range []string{".devkit/principles", ".devkit/stages", ".devkit/checks", ".devkit/plans", ".devkit/templates", ".devkit/rules.md", "records"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("init created an unconfigured or stage-owned path: %s: %v", name, err)
		}
	}
	report, err := Doctor(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"norms/principles", "norms/stages", "norms/plans", "norms/templates", "norms/rules.md", "HEAD", "init_committed"} {
		if check := checkNamed(t, report, name); !check.OK {
			t.Fatalf("configured norm was rejected: %+v", check)
		}
	}
	writeFile(t, root, "norms/plans/local.json", "{}\n")
	report, err = Doctor(ctx, root, "")
	if err != nil || checkNamed(t, report, "init_committed").OK {
		t.Fatalf("untracked configured plan was accepted: %+v, %v", report, err)
	}
	for _, name := range []string{"records/requirements.md", "records/structure.json"} {
		writeFile(t, root, name, "Stage-owned content.\n")
	}
	before, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Uninstall(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Preserved, "norms/principles/harness.md") || !slices.Contains(result.Preserved, "devkit.toml") {
		t.Fatalf("repository-owned norms or config were not preserved: %v", result.Preserved)
	}
	for _, name := range []string{"norms/stages", "norms/templates", "norms/rules.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("unmodified norm remains: %s: %v", name, err)
		}
	}
	for _, name := range []string{"records/requirements.md", "records/structure.json"} {
		if got := string(readFile(t, root, name)); got != "Stage-owned content.\n" {
			t.Errorf("stage-owned content changed: %s", name)
		}
	}
	after, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("uninstall changed HEAD: %v", err)
	}
}
