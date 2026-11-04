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

func TestPausedCommandsWithoutForwardAssets(t *testing.T) {
	for _, broken := range []string{"devkit.toml"} {
		t.Run(broken, func(t *testing.T) {
			root, _ := startWork(t, event.ClaudeCode)
			if err := os.WriteFile(filepath.Join(root, broken), []byte("Invalid configuration or schema"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitCommand(t, root, "add", broken)
			gitCommand(t, root, "commit", "-qm", "Commit invalid forward configuration")
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
