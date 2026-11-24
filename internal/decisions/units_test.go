package decisions

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestUnitFrontMatterDoesNotTurnEmptyScalarsIntoLists(t *testing.T) {
	for _, field := range []string{"decisions", "depends_on"} {
		for _, scalar := range []string{"", `""`, "null", "false", "D-aaaaaa"} {
			unit := testutil.Must(Parse("---\n" + field + ": " + scalar + "\nid: U-aaaaaa\n---\nGoal"))
			if _, ok := list(unit[field]); ok {
				t.Fatalf("%s: %q became a list", field, scalar)
			}
		}
	}
}
