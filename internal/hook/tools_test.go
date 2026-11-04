package hook

import (
	"reflect"
	"testing"
)

func TestClaudeAgentToolLists(t *testing.T) {
	for _, value := range []string{"Read, Glob, Grep", `"Read, Glob, Grep"`, `["Read", "Glob", "Grep"]`, "[Read, Glob, Grep]"} {
		tools, err := agentTools(value)
		if err != nil || !reflect.DeepEqual(tools, []string{"Read", "Glob", "Grep"}) {
			t.Fatalf("tools %q = %v, %v", value, tools, err)
		}
	}
	for _, value := range []string{`"Read, Bash"`, `["Read", "Bash"]`} {
		tools, err := agentTools(value)
		if err != nil || !reflect.DeepEqual(tools, []string{"Read", "Bash"}) {
			t.Fatalf("a write-capable tool was hidden: %q: %v, %v", value, tools, err)
		}
	}
	for _, value := range []string{"", `["Read"`, `"Read", "Bash"`} {
		if _, err := agentTools(value); err == nil {
			t.Fatalf("accepted an ambiguous tool list: %q", value)
		}
	}
}
