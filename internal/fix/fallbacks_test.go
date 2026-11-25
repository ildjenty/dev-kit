package fix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRoundEndRejectsInvalidCommitMessageBeforeStaging(t *testing.T) {
	for _, message := range []any{nil, "", " \n", 1, "feat: wrong prefix", "repair: first\nsecond", "repair: "} {
		t.Run("invalid", func(t *testing.T) {
			ctx, artifacts, _, head := setup(t, 3)
			testutil.Must(prepare(ctx, artifacts))
			testutil.Put(t, ctx.Cwd, "change.txt", "change\n")
			index := testutil.Git(t, ctx.Cwd, "write-tree")
			file := filepath.Join(artifacts, "fix/state.json")
			before := testutil.Must(os.ReadFile(file))
			_, err := roundEnd(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: message})
			testutil.Fails(t, err, "CODE.commit_message")
			testutil.Fails(t, err, "rewrite")
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "write-tree"), index)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(file))), string(before))
		})
	}
}

func TestRoundEndAllowsAbsentCommitMessageWhenNoCommitIsNeeded(t *testing.T) {
	ctx, artifacts, _, _ := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	gate := gateFixture()
	gate.Passed = false
	testutil.Equal(t, testutil.Must(roundEnd(ctx, artifacts, gate, nil))["passed"], false)
}
