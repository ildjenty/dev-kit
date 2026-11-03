package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/host/event"
)

func TestFrontmatterValue(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"sonnet # model selection", "sonnet"},
		{"'sonnet' # model selection", "sonnet"},
		{`"high" # effort selection`, "high"},
		{`"value#literal" # trailing comment`, "value#literal"},
		{"'value # literal' # trailing comment", "value # literal"},
		{"'it''s quoted' # comment", "it's quoted"},
		{`"quoted\"value" # comment`, `quoted"value`},
		{"value#literal", "value#literal"},
		{"# no value", ""},
		{" high\t# effort selection", "high"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := frontmatterValue(tc.source)
			if err != nil || got != tc.want {
				t.Fatalf("parsed %q as %q, %v; want %q", tc.source, got, err, tc.want)
			}
		})
	}
}

func TestPausedCommandsWithoutForwardAssets(t *testing.T) {
	for _, broken := range []string{"devkit.toml", ".devkit/kit/v1/schemas/progress.json"} {
		t.Run(broken, func(t *testing.T) {
			root, _ := startWork(t, event.ClaudeCode)
			if err := os.WriteFile(filepath.Join(root, broken), []byte("Invalid configuration or schema"), 0o644); err != nil {
				t.Fatal(err)
			}
			command(t, "", "status")
			command(t, "", "pause")
			if got := command(t, "", "next"); !strings.Contains(got, `"paused": true`) {
				t.Fatalf("paused next did not return the pause overlay: %s", got)
			}
			command(t, "", "resume")
			var output bytes.Buffer
			if err := run(context.Background(), []string{"next"}, strings.NewReader(""), &output); err == nil {
				t.Fatal("forward next ignored invalid assets")
			}
		})
	}
}
