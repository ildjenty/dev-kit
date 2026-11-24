package retro

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestWorkflowJudgeMatchesMachineSchemaAndIsFreshReadOnly(t *testing.T) {
	data := testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/retro/retro.yaml"))
	var workflow doc
	testutil.OK(t, yaml.Unmarshal(data, &workflow))
	testutil.Equal(t, len(obj(workflow["inputs"])), 3)
	nodes := items(workflow["nodes"])
	for _, node := range nodes {
		if node["id"] != "draft" {
			continue
		}
		loop := obj(node["loop_group"])
		testutil.Equal(t, loop["max_iterations"], 3)
		for _, child := range items(loop["nodes"]) {
			if child["id"] != "judge" {
				continue
			}
			testutil.Equal(t, child["provider"], "claude")
			testutil.Equal(t, child["context"], "fresh")
			testutil.Equal(t, child["when"], "$check.output.valid == true")
			testutil.Equal(t, child["mutates_checkout"], false)
			testutil.Equal(t, child["allowed_tools"], []string{"Read", "Grep", "Glob"})
			var schema doc
			testutil.OK(t, json.Unmarshal(testutil.Must(schemas.ReadFile("schemas/judgment.schema.json")), &schema))
			testutil.Equal(t, child["output_format"], schema)
			return
		}
	}
	t.Fatal("retro judge missing")
}
