package initialize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestDuplicateConfigKeysRejectPartialUpdatesBeforeQuestionsAndWrites(t *testing.T) {
	for _, tc := range []struct {
		name, raw, key string
	}{
		{"provider", `{"provider":"claude","provider":"claude"}`, "provider"},
		{"sessions", `{"sessions":["claude"],"sessions":["codex"]}`, "sessions"},
		{"skills", `{"skills":{},"skills":{"devkit-runs":"old"}}`, "skills"},
		{"language", `{"language":"en","language":"ja"}`, "language"},
		{"nested skill", `{"skills":{"devkit-runs":"old","devkit-runs":"old"}}`, "devkit-runs"},
		{"nested unrelated setting", `{"limits":{"fixes":1,"fixes":2}}`, "fixes"},
		{"object in array", `{"checks":[{"name":"first","name":"second"}]}`, "name"},
		{"unknown key", `{"custom":{"nested":[{"label":1,"label":2}]}}`, "label"},
		{"escaped key", `{"provider":"claude","\u0070rovider":"claude"}`, "provider"},
		{"shadowed object", `{"custom":{"hidden":1,"hidden":2},"custom":{}}`, "hidden"},
	} {
		for _, flag := range []string{"--provider=codex", "--provider=claude", "--sessions=claude,codex", "--language=fr", "--skill=devkit-runs=delivery"} {
			for _, mode := range []string{"interactive", "--yes", "--no", "--dry-run"} {
				t.Run(tc.name+"/"+flag+"/"+mode, func(t *testing.T) {
					root := t.TempDir()
					testutil.Put(t, root, ".devkit/config.json", tc.raw+"\n")
					testutil.Put(t, root, kitinfo.StampPath, "existing stamp")
					testutil.Put(t, root, ".devkit/decisions/D-abcdef.md", "existing decision")
					testutil.Put(t, root, ".archon/workflows/dev/custom/custom.yaml", "existing workflow")
					before := snapshot(t, root)
					var out, stderr bytes.Buffer
					input := observingReader(func([]byte) (int, error) {
						t.Fatal("read stdin before rejecting duplicate key")
						return 0, nil
					})
					env := cli.Env{Cwd: root, Stdin: input, Stdout: &out, Stderr: &stderr}
					if mode == "interactive" {
						err := run([]string{flag}, env, true)
						testutil.Fails(t, err, fmt.Sprintf("duplicated key %q", tc.key))
						testutil.Fails(t, err, "remove the duplicate")
					} else {
						code := cli.New("test", Commands()).Execute([]string{"init", mode, flag}, env)
						testutil.Equal(t, code, 1)
						testutil.Contains(t, stderr.String(), ".devkit/config.json")
						testutil.Contains(t, stderr.String(), fmt.Sprintf("duplicated key %q", tc.key))
						testutil.Contains(t, stderr.String(), "remove the duplicate")
					}
					testutil.Equal(t, out.String(), "")
					testutil.Equal(t, snapshot(t, root), before)
				})
			}
		}
	}
}

func TestPartialUpdatesWithoutDuplicateKeysPreserveFormatting(t *testing.T) {
	before := "{\n\t\"provider\" : \"claude\",\n\t\"sessions\" : [ \"claude\", \"codex\" ],\n\t\"language\" : \"en\",\n\t\"skills\" : {\"devkit-discuss\":\"devkit-discuss\",\"devkit-optimize\":\"devkit-optimize\",\"devkit-runs\":\"old\"},\n\t\"custom\" : [{\"same\":1e1000},{\"same\":2}],\n\t\"checks\" : []\n}\n"
	for _, tc := range []struct {
		name, old, replacement string
		flags                  []string
	}{
		{"provider", `"claude"`, `"codex"`, []string{"--provider=codex"}},
		{"sessions", `[ "claude", "codex" ]`, `["codex"]`, []string{"--sessions=codex"}},
		{"language", `"en"`, `"fr"`, []string{"--language=fr"}},
		{"skills", `"old"`, `"delivery"`, []string{"--skill=devkit-runs=delivery"}},
		{"combined", "", "", []string{"--provider=codex", "--sessions=claude", "--language=fr", "--skill=devkit-runs=delivery"}},
	} {
		for _, dry := range []bool{false, true} {
			t.Run(fmt.Sprint(tc.name, "/dry=", dry), func(t *testing.T) {
				root := installed(t)
				testutil.Put(t, root, ".devkit/config.json", before)
				unchanged := snapshot(t, root)
				args := append([]string{"--no"}, tc.flags...)
				if dry {
					args = append(args, "--dry-run")
				}
				out := invoke(t, root, args...)
				if dry {
					testutil.Contains(t, out, "Dry run: nothing written or deleted.")
					testutil.Equal(t, snapshot(t, root), unchanged)
					return
				}
				want := strings.Replace(before, tc.old, tc.replacement, 1)
				if tc.name == "combined" {
					want = strings.NewReplacer(`"provider" : "claude"`, `"provider" : "codex"`, `[ "claude", "codex" ]`, `["claude"]`, `"en"`, `"fr"`, `"old"`, `"delivery"`).Replace(before)
				}
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))), want)
				if tc.name == "provider" {
					var stored struct {
						Provider string   `json:"provider"`
						Sessions []string `json:"sessions"`
					}
					testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json"))), &stored))
					testutil.Equal(t, stored.Provider, "codex")
					testutil.Equal(t, stored.Sessions, []string{"claude", "codex"})
				}
			})
		}
	}
}

func TestDuplicateKeyCheckKeepsFullReplacementBehavior(t *testing.T) {
	for _, raw := range []string{`{"provider":"claude","provider":"claude"}`, `{"provider":"claude","provider":"claude",}`} {
		for _, mode := range []string{"--yes", "--no", "--dry-run"} {
			t.Run(raw+mode, func(t *testing.T) {
				root := installed(t)
				testutil.Put(t, root, ".devkit/config.json", raw)
				before := snapshot(t, root)
				out := invoke(t, root, mode)
				if mode != "--yes" {
					testutil.Equal(t, snapshot(t, root), before)
					return
				}
				testutil.Contains(t, out, ".devkit/config.json: overwrite this file?")
				settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
				testutil.Equal(t, settings.Provider, "claude")
			})
		}
	}
	for _, mode := range []string{"--yes", "--no", "--dry-run"} {
		t.Run("malformed with explicit provider/"+mode, func(t *testing.T) {
			root := installed(t)
			testutil.Put(t, root, ".devkit/config.json", `{"provider":"claude","provider":"claude",}`)
			before := snapshot(t, root)
			var out, stderr bytes.Buffer
			code := cli.New("test", Commands()).Execute([]string{"init", mode, "--provider=codex"}, cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr})
			testutil.Contains(t, out.String(), "Warning: .devkit/config.json is not a valid JSON object")
			if mode == "--no" {
				testutil.Equal(t, code, 1)
				testutil.Contains(t, stderr.String(), "cannot update only skills/provider/sessions/language")
			} else {
				testutil.Equal(t, code, 0)
			}
			if mode != "--yes" {
				testutil.Equal(t, snapshot(t, root), before)
				return
			}
			settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
			testutil.Equal(t, settings.Provider, "codex")
			testutil.Equal(t, settings.Sessions, []string{"claude", "codex"})
		})
	}
}
