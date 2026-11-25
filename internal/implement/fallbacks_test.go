package implement

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestEndRejectsConfigFailureAndUsesValidatedPassLimit(t *testing.T) {
	for _, input := range []string{"{", `{"limits":{"passes":0}}`, `{"limits":{"passes":1}}`, `{}`} {
		t.Run(input, func(t *testing.T) {
			ctx, artifacts := setup(t, true)
			testutil.Must(prepare(ctx, artifacts))
			file := filepath.Join(artifacts, "implement/state.json")
			before := testutil.Must(os.ReadFile(file))
			testutil.Put(t, ctx.Cwd, ".devkit/config.json", input)
			gate := gateFixture()
			gate.Passed = false
			result, err := end(ctx, artifacts, gate, &codeOutput{}, nil)
			if input == "{" || input == `{"limits":{"passes":0}}` {
				testutil.Fails(t, err, ".devkit/config.json")
				testutil.Equal(t, string(testutil.Must(os.ReadFile(file))), string(before))
				return
			}
			testutil.OK(t, err)
			testutil.Equal(t, result["finished"], input != `{}`)
		})
	}
}

func TestEndRejectsInvalidCommitMessageBeforeStaging(t *testing.T) {
	for _, message := range []any{nil, "", " \n", 1, "feat: wrong prefix", "repair: U1 cancellation", "repair: first\nsecond", "repair: "} {
		t.Run("invalid", func(t *testing.T) {
			ctx, artifacts := setup(t, true)
			testutil.Must(prepare(ctx, artifacts))
			testutil.Put(t, ctx.Cwd, "change.txt", "change\n")
			head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
			index := testutil.Git(t, ctx.Cwd, "write-tree")
			file := filepath.Join(artifacts, "implement/state.json")
			before := testutil.Must(os.ReadFile(file))
			_, err := end(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: message}, nil)
			testutil.Fails(t, err, "CODE.commit_message")
			testutil.Fails(t, err, "rewrite")
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "write-tree"), index)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(file))), string(before))
		})
	}
}

func TestEndAllowsAbsentCommitMessageWhenNoCommitIsNeeded(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	gate := gateFixture()
	gate.Passed = false
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, gate, nil, nil))["committed"], false)
}
