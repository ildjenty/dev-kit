package requirements

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRoundCLIRejectsBrokenIndexWithoutChangingHistory(t *testing.T) {
	for _, failure := range []string{"malformed", "read-error", "missing-index", "stale-index"} {
		t.Run(failure, func(t *testing.T) {
			artifacts := t.TempDir()
			roundInput(t, artifacts)
			must(round(artifacts, requirements(), judgment()))
			first := filepath.Join(rounds(artifacts), "1.json")
			original := must(os.ReadFile(first))
			latest := filepath.Join(rounds(artifacts), "latest.json")
			switch failure {
			case "malformed":
				testutil.OK(t, os.WriteFile(latest, []byte("{"), 0644))
			case "read-error":
				testutil.OK(t, os.Remove(latest))
				testutil.OK(t, os.Mkdir(latest, 0755))
			case "missing-index":
				testutil.OK(t, os.Remove(latest))
			case "stale-index":
				entry := must(read(first))
				entry["round"] = 2
				testutil.OK(t, write(filepath.Join(rounds(artifacts), "2.json"), entry))
			}
			before := must(os.ReadDir(rounds(artifacts)))
			files := map[string]string{}
			for _, entry := range before {
				if !entry.IsDir() {
					files[entry.Name()] = string(must(os.ReadFile(filepath.Join(rounds(artifacts), entry.Name()))))
				}
			}
			draft := requirements()
			draft["goal"] = "A different request"
			vars := map[string]string{"ARTIFACTS_DIR": artifacts, "INPUTS_REQUIREMENTS": string(must(json.Marshal(draft))), "INPUTS_JUDGMENT": string(must(json.Marshal(judgment())))}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: artifacts, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"requirements", "round"}, env), 1)
			testutil.Equal(t, out.String(), "")
			if stderr.Len() == 0 {
				t.Fatal("round failure was not reported")
			}
			testutil.Equal(t, string(must(os.ReadFile(first))), string(original))
			testutil.Equal(t, len(must(os.ReadDir(rounds(artifacts)))), len(before))
			for name, content := range files {
				testutil.Equal(t, string(must(os.ReadFile(filepath.Join(rounds(artifacts), name)))), content)
			}
		})
	}
}

func TestRoundIndexWriteFailureKeepsHistoryAndRejectsRetry(t *testing.T) {
	artifacts := t.TempDir()
	roundInput(t, artifacts)
	must(round(artifacts, requirements(), judgment()))
	latest := filepath.Join(rounds(artifacts), "latest.json")
	before := must(os.ReadFile(latest))
	testutil.OK(t, os.Chmod(latest, 0444))
	t.Cleanup(func() { os.Chmod(latest, 0644) })
	if f, err := os.OpenFile(latest, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("filesystem permits writing a read-only index")
	}
	_, err := round(artifacts, requirements(), judgment())
	if !os.IsPermission(err) {
		t.Fatalf("expected index write permission error, got %v", err)
	}
	second := filepath.Join(rounds(artifacts), "2.json")
	saved := must(os.ReadFile(second))
	testutil.Equal(t, number(must(read(second))["round"]), 2)
	testutil.Equal(t, string(must(os.ReadFile(latest))), string(before))
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(rounds(artifacts), "1.json")))), string(before))
	testutil.OK(t, os.Chmod(latest, 0644))
	draft := requirements()
	draft["goal"] = "Must not overwrite the unindexed round"
	_, err = round(artifacts, draft, judgment())
	if !os.IsExist(err) {
		t.Fatalf("expected existing round error, got %v", err)
	}
	testutil.Equal(t, string(must(os.ReadFile(second))), string(saved))
	testutil.Equal(t, string(must(os.ReadFile(latest))), string(before))
}

func TestRoundCreatesFirstAndNextWithoutChangingEarlierHistory(t *testing.T) {
	artifacts := t.TempDir()
	roundInput(t, artifacts)
	testutil.Equal(t, must(round(artifacts, requirements(), judgment()))["round"], 1)
	first := must(os.ReadFile(filepath.Join(rounds(artifacts), "1.json")))
	draft := requirements()
	draft["goal"] = "The next round"
	testutil.Equal(t, must(round(artifacts, draft, judgment()))["round"], 2)
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(rounds(artifacts), "1.json")))), string(first))
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(rounds(artifacts), "2.json")))), string(must(os.ReadFile(filepath.Join(rounds(artifacts), "latest.json")))))
}
