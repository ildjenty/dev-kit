package initialize

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestFreshInstallDetectsLocaleLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, all, messages, lang, want, source string
	}{
		{"Japanese", "", "", "ja_JP.UTF-8", "ja", "LANG"},
		{"English", "", "", "en_US", "en", "LANG"},
		{"Portuguese", "", "", "pt_BR", "pt", "LANG"},
		{"modifier", "", "", "DE@euro", "de", "LANG"},
		{"encoding", "", "", "FR.UTF-8", "fr", "LANG"},
		{"C", "", "", "C", "en", "LANG"},
		{"POSIX", "", "", "POSIX", "en", "LANG"},
		{"empty", "", "", "", "en", "default"},
		{"LC_MESSAGES", "", "fr_FR.UTF-8", "ja_JP.UTF-8", "fr", "LC_MESSAGES"},
		{"LC_ALL", "ja_JP.UTF-8", "fr_FR.UTF-8", "en_US", "ja", "LC_ALL"},
		{"C takes precedence", "C", "ja_JP.UTF-8", "ja_JP.UTF-8", "en", "LC_ALL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			values := map[string]string{"LC_ALL": tc.all, "LC_MESSAGES": tc.messages, "LANG": tc.lang}
			var out bytes.Buffer
			testutil.OK(t, run(nil, cli.Env{Cwd: root, Stdout: &out, LookupEnv: func(key string) (string, bool) {
				value, ok := values[key]
				return value, ok
			}}, false))
			settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
			testutil.Equal(t, settings.Language, tc.want)
			testutil.Contains(t, out.String(), "Language: "+tc.want+" (from "+tc.source+")")
			testutil.Contains(t, out.String(), "devkit-optimize can fit it to this repository")
		})
	}
}

func TestLanguageFlagOnFreshInstall(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	for _, language := range []string{"en", "ja", "pt-BR"} {
		for _, args := range [][]string{{"--language", language}, {"--language=" + language}} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				root := t.TempDir()
				out := invoke(t, root, args...)
				settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
				testutil.Equal(t, settings.Language, language)
				testutil.Contains(t, out, "Language: "+language+" (from --language)")
			})
		}
	}
}

func TestLanguageFlagOnReinstall(t *testing.T) {
	for _, flag := range []string{"--yes", "--no"} {
		for _, args := range [][]string{{"--language", "pt-BR"}, {"--language=pt-BR"}} {
			t.Run(flag+strings.Join(args, " "), func(t *testing.T) {
				root := installed(t)
				before := "{\n  \"language\" : \"ja\",\n  \"docs\": \"manual\",\n  \"checks\": []\n}\n"
				testutil.Put(t, root, ".devkit/config.json", before)
				invoke(t, root, append([]string{flag}, args...)...)
				raw := testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))
				testutil.Equal(t, testutil.Must(config.Parse(raw)).Language, "pt-BR")
				if flag == "--no" {
					testutil.Equal(t, string(raw), strings.Replace(before, `"ja"`, `"pt-BR"`, 1))
				}
			})
		}
	}
}

func TestReinstallKeepsLanguageWithoutFlag(t *testing.T) {
	for _, flag := range []string{"--yes", "--no"} {
		t.Run(flag, func(t *testing.T) {
			root := installed(t)
			before := "{\n  \"language\" : \"ja\",\n  \"docs\": \"manual\",\n  \"checks\": []\n}\n"
			testutil.Put(t, root, ".devkit/config.json", before)
			t.Setenv("LC_ALL", "fr_FR.UTF-8")
			invoke(t, root, flag)
			raw := testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))
			testutil.Equal(t, testutil.Must(config.Parse(raw)).Language, "ja")
			if flag == "--no" {
				testutil.Equal(t, string(raw), before)
			}
		})
	}
}

func TestInvalidLanguageFlagsWriteNothing(t *testing.T) {
	for _, args := range [][]string{{"--language"}, {"--language", ""}, {"--language="}, {"--language", " \t\n"}} {
		root := t.TempDir()
		var out bytes.Buffer
		if err := run(args, cli.Env{Cwd: root, Stdout: &out}, false); err == nil {
			t.Errorf("accepted %v", args)
		}
		testutil.Equal(t, snapshot(t, root), map[string]string{".": "directory"})
	}
}

func TestLanguageDryRunWritesNothing(t *testing.T) {
	root := installed(t)
	before := snapshot(t, root)
	out := invoke(t, root, "--dry-run", "--language=ja")
	testutil.Contains(t, out, "Language: ja (from --language)")
	testutil.Equal(t, snapshot(t, root), before)
}
