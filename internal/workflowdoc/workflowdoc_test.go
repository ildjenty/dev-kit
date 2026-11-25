package workflowdoc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func TestWorkflowDiagramsMatchTheWorkflows(t *testing.T) {
	want := testutil.Must(Render(kit.Files, ".archon/workflows/dev"))
	file := filepath.Join("..", "..", "docs", "workflows.md")
	if os.Getenv("DEVKIT_UPDATE_DOCS") == "1" {
		testutil.OK(t, os.WriteFile(file, []byte(want), 0644))
	}
	got, err := os.ReadFile(file)
	testutil.OK(t, err)
	if string(got) != want {
		t.Fatalf("docs/workflows.md is out of date with the workflows; run DEVKIT_UPDATE_DOCS=1 mise exec -- go test ./internal/workflowdoc")
	}
}

func TestEveryWorkflowIsDrawn(t *testing.T) {
	entries := testutil.Must(kit.Files.ReadDir(".archon/workflows/dev"))
	drawn := map[string]bool{}
	for _, name := range Order {
		drawn[name] = true
	}
	for _, e := range entries {
		if e.IsDir() && e.Name()[0] != '.' && !drawn[e.Name()] {
			t.Errorf("workflow %s is not in Order", e.Name())
		}
	}
}
