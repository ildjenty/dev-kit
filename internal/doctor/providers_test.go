package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestWorkflowProvidersUseYAMLNodesAndBothExtensions(t *testing.T) {
	for _, extension := range []string{"yaml", "yml"} {
		for _, provider := range []string{"codex", `"codex"`, "'codex'", "codex # comment"} {
			t.Run(extension+provider, func(t *testing.T) {
				env, bin, out := environment(t, "claude")
				testutil.OK(t, os.Remove(filepath.Join(bin, "plugins/archon-provider-claude")))
				testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/deep/custom."+extension, "nodes:\n  - id: group\n    loop_group:\n      nodes:\n        - id: ai\n          provider: "+provider+"\n")
				providers := testutil.Must(usedProviders(env.Cwd, "claude"))
				testutil.Equal(t, providers, []string{"claude", "codex"})
				registry := cli.New("test", append(Commands(), env.Registry.Commands()...))
				testutil.Equal(t, registry.Execute([]string{"doctor"}, env), 0)
				testutil.Contains(t, out.String(), "[required] Archon provider codex: OK")
				out.Reset()
				executable(t, bin, "archon", "printf 'Installed providers: claude\\n'")
				testutil.Equal(t, registry.Execute([]string{"doctor"}, env), 1)
				testutil.Contains(t, out.String(), "[required] Archon provider codex: MISSING")
			})
		}
	}
}

func TestWorkflowProvidersIgnoreNonNodeFieldsAndFixtures(t *testing.T) {
	env, _, out := environment(t, "claude")
	testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/custom.yaml", `description: |
  provider: nonexistent
nodes:
  - id: ai
    provider: &provider "claude"
    output_format:
      properties:
        provider: {type: string}
  - id: alias
    provider: *provider
  - {id: inline, provider: 'codex'}
`)
	testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/fixtures/custom.stubs.yml", "invalid: [")
	testutil.Equal(t, testutil.Must(usedProviders(env.Cwd, "claude")), []string{"claude", "codex"})
	testutil.Equal(t, Report(env), true)
	testutil.Equal(t, strings.Contains(out.String(), "nonexistent"), false)
}

func TestWorkflowProviderReadAndParseErrorsFailDoctorWithoutWrites(t *testing.T) {
	for _, failure := range []string{"syntax", "type", "unreadable", "walk"} {
		t.Run(failure, func(t *testing.T) {
			env, _, out := environment(t, "claude")
			path := filepath.Join(env.Cwd, kitinfo.Workflows, "custom/custom.yml")
			switch failure {
			case "syntax":
				testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/custom.yml", "nodes: [")
			case "type":
				testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/custom.yml", "nodes:\n  - provider: {invalid: value}\n")
			case "unreadable":
				testutil.OK(t, os.MkdirAll(filepath.Dir(path), 0755))
				testutil.OK(t, os.Symlink(filepath.Join(env.Cwd, "absent"), path))
			case "walk":
				testutil.OK(t, os.RemoveAll(filepath.Join(env.Cwd, kitinfo.Workflows)))
			}
			_, err := usedProviders(env.Cwd, "claude")
			if err == nil {
				t.Fatal("expected workflow provider discovery error")
			}
			configPath := filepath.Join(env.Cwd, ".devkit/config.json")
			before := testutil.Must(os.ReadFile(configPath))
			registry := cli.New("test", append(Commands(), env.Registry.Commands()...))
			testutil.Equal(t, registry.Execute([]string{"doctor"}, env), 1)
			testutil.Contains(t, out.String(), "[required] workflow providers: MISSING — Cannot read workflow providers:")
			testutil.Equal(t, testutil.Must(os.ReadFile(configPath)), before)
			if failure == "syntax" || failure == "type" {
				testutil.Contains(t, out.String(), path)
			}
		})
	}
}
