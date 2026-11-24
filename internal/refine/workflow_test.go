package refine

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestOnePhaseJudgeUsesSameSchemaAndFreshReadOnlyContext(t *testing.T) {
	var workflow doc
	testutil.OK(t, yaml.Unmarshal(testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/refine/refine.yaml")), &workflow))
	loops, judges := 0, 0
	for _, node := range items(workflow["nodes"]) {
		loop := obj(node["loop_group"])
		if loop == nil {
			continue
		}
		loops++
		testutil.Equal(t, loop["max_iterations"], 3)
		for _, child := range items(loop["nodes"]) {
			if child["id"] != "judge-decide" {
				continue
			}
			judges++
			testutil.Equal(t, child["context"], "fresh")
			testutil.Equal(t, child["mutates_checkout"], false)
			testutil.Equal(t, child["allowed_tools"], []string{"Read", "Grep", "Glob"})
			var schema doc
			testutil.OK(t, json.Unmarshal(testutil.Must(schemas.ReadFile("schemas/judgment.schema.json")), &schema))
			testutil.Equal(t, child["output_format"], schema)
		}
	}
	testutil.Equal(t, loops, 1)
	testutil.Equal(t, judges, 1)
}
