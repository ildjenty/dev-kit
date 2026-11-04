package install

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestExternalAndReadOnlySourcesRemainUntouched(t *testing.T) {
	root := repository(t)
	vault := t.TempDir()
	writeFile(t, vault, "local.json", "External plan contents.\n")
	writeFile(t, root, "read-only/rules.md", "Read-only rules.\n")
	configuration := fmt.Sprintf("kit_version = 'v2'\n[sources.plans]\npath = %q\n[sources.rules]\npath = 'read-only/rules.md'\nwritable = false\n[sources.extra]\npath = 'missing-extra'\nnorm = true\n", vault)
	writeFile(t, root, "devkit.toml", configuration)
	if _, err := Init(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(vault)
	if err != nil || len(entries) != 1 || entries[0].Name() != "local.json" {
		t.Fatalf("init changed the external source: %+v, %v", entries, err)
	}
	report, err := Doctor(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !checkNamed(t, report, vault).OK || checkNamed(t, report, "missing-extra").OK || !checkNamed(t, report, "init_committed").OK {
		t.Fatalf("source existence or commit checks differ: %+v", report)
	}
	if _, err := Uninstall(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(readFile(t, vault, "local.json")); got != "External plan contents.\n" {
		t.Fatalf("external plan changed: %q", got)
	}
	if got := string(readFile(t, root, "read-only/rules.md")); got != "Read-only rules.\n" {
		t.Fatalf("read-only rules changed: %q", got)
	}
}
