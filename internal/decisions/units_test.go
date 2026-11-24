package decisions

import (
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestUnitConstraintsRejectInvalidLists(t *testing.T) {
	for _, field := range []string{"depends_on", "decisions"} {
		for _, invalid := range []any{nil, "", "D-aaaaaa", true, []any{1}} {
			data := Data{Units: []Item{{"id": "U-aaaaaa", "state": "open", "depends_on": []string{}, "decisions": []string{}}}}
			data.Units[0][field] = invalid
			_, err := UnitDecisions(data, "U-aaaaaa")
			testutil.Fails(t, err, field+" must be a list")
			if field == "depends_on" {
				testutil.Contains(t, strings.Join(UnitStartReasons(data, "U-aaaaaa"), "\n"), "depends_on must be a list")
			}
		}
	}
	data := Data{Units: []Item{{"id": "U-aaaaaa", "state": "open", "depends_on": []string{}, "decisions": []string{}}}}
	testutil.Equal(t, UnitStartReasons(data, "U-aaaaaa"), []string{})
	testutil.Equal(t, testutil.Must(UnitDecisions(data, "U-aaaaaa")), []Item{})
	data.Units[0]["depends_on"] = []string{"U-bbbbbb"}
	data.Units = append(data.Units, Item{"id": "U-bbbbbb", "state": "open"})
	testutil.Equal(t, UnitStartReasons(data, "U-aaaaaa"), []string{"unit U-aaaaaa waits for U-bbbbbb"})
	data.Units[0]["decisions"] = []string{"D-aaaaaa"}
	data.Decisions = []Item{{"id": "D-aaaaaa", "status": "proposed"}}
	_, err := UnitDecisions(data, "U-aaaaaa")
	testutil.Fails(t, err, "decision D-aaaaaa which is not agreed")
}

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
