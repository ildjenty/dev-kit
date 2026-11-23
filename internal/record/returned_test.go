package record

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReturnedKeepsEveryUnconsumedDiagnosisAndDropsInvalidOnes(t *testing.T) {
	dir := afterPlan(t)
	for _, id := range []string{"j1", "j2"} {
		run(dir, id, "judge", nil)
		end(dir, "judge", id, Document{"outcome": "back", "back": Document{"to": "plan", "premise": "Ordering is preserved", "observation": id + ": ordering failed"}})
	}
	run(dir, "p2", "plan", nil)
	ctx := ContextData{Dir: dir, RunID: "p2"}
	testutil.Equal(t, len(must(Returned(ctx, "plan"))), 2)
	end(dir, "plan", "p2", Document{"outcome": "stop", "stop": Document{"state": "More evidence is needed", "unresolved": []string{"Ordering"}, "premise": "Ordering is preserved"}})
	run(dir, "p3", "plan", nil)
	ctx.RunID = "p3"
	testutil.Equal(t, len(must(Returned(ctx, "plan"))), 2)
	plan(dir, "p3")
	end(dir, "plan", "p3", Document{"outcome": "pass"})
	run(dir, "p4", "plan", nil)
	ctx.RunID = "p4"
	testutil.Equal(t, must(Returned(ctx, "plan")), []Document{})
	run(dir, "j3", "judge", nil)
	end(dir, "judge", "j3", Document{"outcome": "back", "back": Document{"to": "plan", "premise": "New premise", "observation": "New observation"}})
	run(dir, "r2", "requirements", nil)
	run(dir, "p5", "plan", nil)
	ctx.RunID = "p5"
	testutil.Equal(t, must(Returned(ctx, "plan")), []Document{})
}
