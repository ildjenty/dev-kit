package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRegistry(t *testing.T) {
	var out, errout bytes.Buffer
	env := Env{Cwd: "/repo", Stdout: &out, Stderr: &errout, Stdin: strings.NewReader("input"), LookupEnv: func(key string) (string, bool) { return "value", key == "KEY" }}
	r := New("v1", []Command{{Area: "sample", Name: "run", Usage: "devkit sample run <arg>", Run: func(args []string, e Env) error {
		testutil.Equal(t, args, []string{"a"})
		testutil.Equal(t, e.Cwd, "/repo")
		testutil.Equal(t, e.Getenv("KEY"), "value")
		return e.JSON(map[string]bool{"ok": true})
	}}})
	testutil.Equal(t, r.Execute([]string{"help"}, env), 0)
	testutil.Contains(t, out.String(), "devkit sample run <arg>")
	testutil.Contains(t, out.String(), "devkit version")
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"version"}, env), 0)
	testutil.Equal(t, out.String(), "v1 "+kitinfo.EmbeddedDigest()+"\n")
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"sample", "run", "a"}, env), 0)
	testutil.Equal(t, out.String(), "{\"ok\":true}\n")
	testutil.Equal(t, r.Execute([]string{"missing"}, env), 2)
	testutil.Contains(t, errout.String(), "Usage:")
	testutil.Equal(t, len(r.Commands()), 1)
}

func TestMissingCommandsRejectsUnknownSubcommandsDespiteAreaFallback(t *testing.T) {
	r := New("test", []Command{{Area: "plan"}, {Area: "plan", Name: "check"}, {Area: "status"}})
	testutil.Equal(t, r.MissingCommands([]string{"plan check", "plan missing", "unknown run", "status", "help", "version"}), []string{"devkit plan missing", "devkit unknown run"})
}
