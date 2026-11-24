package escape

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestCommandRecordsOwnerTextAndResolvesLocalCommits(t *testing.T) {
	for _, entered := range []string{"HEAD", "short", "27", "https://github.com/acme/repo/pull/27", strings.Repeat("b", 40)} {
		t.Run(entered, func(t *testing.T) {
			root := testutil.Repo(t)
			head := testutil.Git(t, root, "rev-parse", "HEAD")
			ref, want := entered, entered
			if entered == "HEAD" {
				want = head
			}
			if entered == "short" {
				ref, want = head[:8], head
			}
			text := "The owner noticed a broken response after merge.\nQuotes: ' \" ` $() ${HOME}\n"
			defect := filepath.Join(t.TempDir(), "owner.txt")
			testutil.OK(t, os.WriteFile(defect, []byte(text), 0644))
			sub := filepath.Join(root, "sub")
			testutil.OK(t, os.Mkdir(sub, 0755))
			var out, errOut bytes.Buffer
			env := cli.Env{Cwd: sub, Stdout: &out, Stderr: &errOut}
			code := cli.New("test", Commands()).Execute([]string{"backlog", "escape", "--defect-file", defect, "--entered", ref, "--fixed", "HEAD"}, env)
			if code != 0 {
				t.Fatal(errOut.String())
			}
			paths := testutil.Must(filepath.Glob(filepath.Join(root, ".devkit/backlog/escapes/*.json")))
			testutil.Equal(t, len(paths), 1)
			testutil.Equal(t, ID.MatchString(strings.TrimSuffix(filepath.Base(paths[0]), ".json")), true)
			entry := testutil.Must(Validate(testutil.Must(os.ReadFile(paths[0]))))
			testutil.Equal(t, entry.Defect, text)
			testutil.Equal(t, entry.Entered, want)
			testutil.Equal(t, entry.Fixed, head)
			testutil.Must(time.Parse(time.RFC3339, entry.RecordedAt))
			testutil.Contains(t, out.String(), paths[0])
			testutil.Contains(t, out.String(), "Commit this record to reach the base branch")
			testutil.Equal(t, testutil.Git(t, root, "rev-parse", "HEAD"), head)
		})
	}
}

func TestCommandRejectsInvalidFlagsReferencesAndArgumentTextWithoutWriting(t *testing.T) {
	for _, args := range [][]string{
		{}, {"owner text"}, {"--defect", "owner text", "--entered", "HEAD"},
		{"--defect-file", "owner.txt"}, {"--defect-file", "missing", "--entered", "HEAD"},
		{"--defect-file", "owner.txt", "--entered", "unknown"},
		{"--defect-file", "owner.txt", "--entered", "-HEAD"},
		{"--defect-file", "owner.txt", "--entered", "0"},
		{"--defect-file", "owner.txt", "--entered", "https://example.com/acme/repo/pull/1"},
		{"--defect-file", "owner.txt", "--entered", "HEAD", "--fixed", ""},
		{"--defect-file", "owner.txt", "--entered", "HEAD", "--fixed", "unknown"},
		{"--defect-file", "owner.txt", "--entered", "HEAD", "owner text"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := testutil.Repo(t)
			testutil.Put(t, root, "owner.txt", "Description")
			var out, errOut bytes.Buffer
			testutil.Equal(t, cli.New("test", Commands()).Execute(append([]string{"backlog", "escape"}, args...), cli.Env{Cwd: root, Stdout: &out, Stderr: &errOut}) != 0, true)
			testutil.Equal(t, out.Len(), 0)
			testutil.Equal(t, len(testutil.Must(filepath.Glob(filepath.Join(root, ".devkit/backlog/escapes/*.json")))), 0)
		})
	}
	for _, text := range []string{"", " \n\t", string([]byte{0xff})} {
		root := testutil.Repo(t)
		testutil.Put(t, root, "owner.txt", text)
		var out bytes.Buffer
		testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"backlog", "escape", "--defect-file", "owner.txt", "--entered", "HEAD"}, cli.Env{Cwd: root, Stdout: &out, Stderr: &out}), 1)
		testutil.Contains(t, out.String(), "non-empty UTF-8")
	}
}

func TestOptionalFixedIsOmittedAndIDsDoNotOverwrite(t *testing.T) {
	root := testutil.Repo(t)
	text := filepath.Join(t.TempDir(), "defect.txt")
	testutil.OK(t, os.WriteFile(text, []byte("Defect"), 0644))
	var out bytes.Buffer
	for i := 0; i < 2; i++ {
		testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"backlog", "escape", "--defect-file", text, "--entered", "1"}, cli.Env{Cwd: root, Stdout: &out, Stderr: &out}), 0)
	}
	paths := testutil.Must(filepath.Glob(filepath.Join(root, ".devkit/backlog/escapes/*.json")))
	testutil.Equal(t, len(paths), 2)
	for _, path := range paths {
		data := testutil.Must(os.ReadFile(path))
		testutil.Must(Validate(data))
		testutil.Equal(t, bytes.Contains(data, []byte(`"fixed"`)), false)
	}
}

func TestEntryValidationIsStrict(t *testing.T) {
	for _, data := range []string{
		`{}`, `null`, `{"defect":"x","entered":"1"}`,
		`{"defect":" ","entered":"1","recorded_at":"2026-11-24T00:00:00Z"}`,
		`{"defect":"x","entered":1,"recorded_at":"2026-11-24T00:00:00Z"}`,
		`{"defect":"x","entered":"1","recorded_at":"today"}`,
		`{"defect":"x","entered":"1","recorded_at":"2026-11-24T00:00:00Z","fixed":null}`,
		`{"defect":"x","entered":"1","recorded_at":"2026-11-24T00:00:00Z","other":true}`,
		`{"Defect":"x","entered":"1","recorded_at":"2026-11-24T00:00:00Z"}`,
		`{"defect":"x","entered":"1","recorded_at":"2026-11-24T00:00:00Z"} {}`,
	} {
		if _, err := Validate([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	data := testutil.Must(json.Marshal(Entry{Defect: "x", Entered: "12", RecordedAt: "2026-11-24T00:00:00Z"}))
	testutil.Must(Validate(data))
}
