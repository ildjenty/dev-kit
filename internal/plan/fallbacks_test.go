package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestFinishRequiresEverySettledRound(t *testing.T) {
	for _, missing := range []string{"first", "all"} {
		t.Run(missing, func(t *testing.T) {
			ctx, a := repository(t, document{})
			ctx.Dir, ctx.RunID, ctx.Branch = t.TempDir(), "p1", "feature/x"
			request(ctx.Dir)
			requirements(ctx.Dir, "r1")
			run(ctx.Dir, ctx.RunID)
			inputFiles(t, a)
			testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
			passed := string(must(json.Marshal(judgment(nil))))
			for n := 1; n <= 2; n++ {
				must(checkCommand(ctx, a, nil, "", "structure"))
				must(settle(a, map[string]string{"structure": passed}, "", "", nil))
			}
			testutil.Equal(t, must(advance(ctx, a))["proceed"], true)
			testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
			must(checkCommand(ctx, a, nil, "", "detail"))
			must(settle(a, map[string]string{"delivery": passed}, "", "", nil))
			path := filepath.Join(rounds(a), "1", "settled.json")
			testutil.OK(t, os.Remove(path))
			if missing == "all" {
				testutil.OK(t, os.Remove(filepath.Join(rounds(a), "2", "settled.json")))
				testutil.OK(t, os.Remove(filepath.Join(rounds(a), "3", "settled.json")))
			}
			_, err := finish(ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c")})
			testutil.Fails(t, err, path)
			testutil.Equal(t, must(record.List(ctx.Dir, "result")), []record.Document{})
			testutil.Equal(t, must(record.List(ctx.Dir, "judgment")), []record.Document{})
		})
	}
}
