package initialize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "devkit-init-tools-*")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"git", "archon", "devkit", "claude", "codex", "gh"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			panic(err)
		}
	}
	os.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	os.Setenv("ARCHON_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
