package judgeeval

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

type failingOutput struct {
	match string
	err   error
}

func (w failingOutput) Write(b []byte) (int, error) {
	if strings.Contains(string(b), w.match) {
		return 0, w.err
	}
	return len(b), nil
}

func TestEvaluationKeepsOutputErrors(t *testing.T) {
	for _, match := range []string{limitation, "Results:", "Case:", "Routing source:", "Case total:", "FOUND ", "MISSED ", "NOT-EXPECTED REPORTED ", "OTHER ", "Total:", "Total contract:", "Result JSON:", "ERROR (not scored):", "Not scored because of errors:"} {
		t.Run(match, func(t *testing.T) {
			root := testutil.Repo(t)
			installTestPack(t, root)
			integrationCase(t, filepath.Join(root, "cases"), false)
			testutil.Put(t, root, ".devkit/config.json", `{"language":"fr"}`)
			env, _, _, _ := fakeEnv(t, root, false)
			if strings.Contains(match, "scored") {
				env = withEnv(env, map[string]string{"DEVKIT_EVAL_EXIT": "contract"})
			}
			failure := errors.New("evaluation output unavailable")
			env.Stdout = failingOutput{match: match, err: failure}
			err := run([]string{"--dir", "cases", "--out", filepath.Join(t.TempDir(), "results")}, env)
			if !errors.Is(err, failure) {
				t.Fatalf("got %v", err)
			}
		})
	}
}
