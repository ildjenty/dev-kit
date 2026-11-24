package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVKIT_PR_TEST_TOOL") == "1" {
		name := filepath.Base(os.Args[0])
		if name == "git" || name == "gh" {
			f, err := os.OpenFile(os.Getenv("DEVKIT_PR_TEST_CALLS"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				os.Exit(2)
			}
			err = json.NewEncoder(f).Encode(append([]string{name}, os.Args[1:]...))
			f.Close()
			if err != nil {
				os.Exit(1)
			}
			if os.Getenv("DEVKIT_PR_TEST_PUBLISH") == "1" {
				if name == "gh" {
					if len(os.Args) > 2 && os.Args[2] == "list" {
						fmt.Print(os.Getenv("DEVKIT_PR_TEST_URL"))
					} else {
						body, err := io.ReadAll(os.Stdin)
						if err != nil || os.WriteFile(os.Getenv("DEVKIT_PR_TEST_BODY"), body, 0600) != nil {
							os.Exit(1)
						}
						fmt.Print("https://example.invalid/pr/1")
					}
					os.Exit(0)
				}
				if len(os.Args) > 1 && os.Args[1] == "ls-remote" && os.Getenv("DEVKIT_PR_TEST_MUTATE_BODY") != "" {
					if os.WriteFile(os.Getenv("DEVKIT_PR_TEST_MUTATE_BODY"), []byte("changed after validation\n"), 0644) != nil {
						os.Exit(1)
					}
				}
			} else if name == "gh" || len(os.Args) > 1 && (os.Args[1] == "commit" || os.Args[1] == "push") {
				os.Exit(1)
			}
			cmd := exec.Command(os.Getenv("DEVKIT_PR_TEST_GIT"), os.Args[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err = cmd.Run(); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestPROpenRejectsUnsafeBacklogAndInvalidPublicationState(t *testing.T) {
	for _, kind := range []string{"destination-link", "backlog-link", "devkit-link", "head-moved", "dirty", "report-failed", "report-missing"} {
		t.Run(kind, func(t *testing.T) {
			repo := testutil.Repo(t)
			outside := t.TempDir()
			file := ".devkit/backlog/feature%2Fx.json"
			target := "keep.txt"
			link := ""
			switch kind {
			case "destination-link":
				link = file
			case "backlog-link":
				link, target = ".devkit/backlog", "feature%2Fx.json"
			case "devkit-link":
				link, target = ".devkit", "backlog/feature%2Fx.json"
			}
			testutil.Put(t, outside, target, "keep external\n")
			if link != "" {
				testutil.OK(t, os.MkdirAll(filepath.Dir(filepath.Join(repo, link)), 0755))
				destination := outside
				if kind == "destination-link" {
					destination = filepath.Join(outside, target)
				}
				testutil.OK(t, os.Symlink(destination, filepath.Join(repo, link)))
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "tracked link")
			}
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			base := testutil.Git(t, repo, "rev-parse", "main")
			remote := t.TempDir()
			testutil.Git(t, remote, "init", "--bare", "-q")
			testutil.Git(t, repo, "remote", "add", "origin", remote)
			testutil.Git(t, repo, "push", "-q", "origin", "HEAD:refs/heads/feature/x")
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			report := judgment("pr1", "report", base, head, doc{"kind": "report", "criteria_commit": base, "materials": reportMaterials([]byte("Title\n"), []byte("Body\n"))})
			if kind == "report-failed" {
				report["verdict"] = "fail"
			}
			if kind != "report-missing" {
				must(record.Append(dir, "judgment", report))
			}
			testutil.Put(t, artifacts, "pr/facts.json", jsonText(doc{"head": head}))
			testutil.Put(t, artifacts, "pr/pr-title.txt", "Title\n")
			testutil.Put(t, artifacts, "pr/pr.md", "Body\n")
			testutil.Put(t, artifacts, "pr/report-state.json", `{"passed":true}`)
			if kind == "head-moved" {
				testutil.Git(t, repo, "commit", "-q", "--allow-empty", "-m", "moved")
				head = testutil.Git(t, repo, "rev-parse", "HEAD")
			}
			if kind == "dirty" {
				testutil.Put(t, repo, "dirty.txt", "keep dirty\n")
			}
			before := testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=all")
			index := testutil.Git(t, repo, "write-tree")
			remoteHead := testutil.Git(t, remote, "rev-parse", "refs/heads/feature/x")
			realGit := must(exec.LookPath("git"))
			bin := t.TempDir()
			executable := must(os.Executable())
			for _, name := range []string{"git", "gh"} {
				testutil.OK(t, os.Symlink(executable, filepath.Join(bin, name)))
			}
			calls := filepath.Join(t.TempDir(), "calls.jsonl")
			t.Setenv("DEVKIT_PR_TEST_TOOL", "1")
			t.Setenv("DEVKIT_PR_TEST_CALLS", calls)
			t.Setenv("DEVKIT_PR_TEST_GIT", realGit)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
			var stdout, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			if code := cli.New("test", Commands()).Execute([]string{"pr", "open"}, env); code == 0 {
				t.Fatal("pr open accepted a rejected publication")
			}
			if link != "" {
				testutil.Contains(t, stderr.String(), "backlog")
			} else {
				testutil.Contains(t, stderr.String(), "the pull request was not opened")
			}
			observed := must(os.ReadFile(calls))
			for _, line := range strings.Split(strings.TrimSpace(string(observed)), "\n") {
				var args []string
				testutil.OK(t, json.Unmarshal([]byte(line), &args))
				if args[0] == "gh" || len(args) > 1 && (args[1] == "commit" || args[1] == "push") {
					t.Fatalf("publication called %v after rejection", args)
				}
			}
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, repo, "write-tree"), index)
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=all"), before)
			testutil.Equal(t, testutil.Git(t, remote, "rev-parse", "refs/heads/feature/x"), remoteHead)
			testutil.Equal(t, string(must(os.ReadFile(filepath.Join(outside, target)))), "keep external\n")
			_, err := os.Stat(filepath.Join(artifacts, "pr/outcome.json"))
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestBacklogReplacementDoesNotOverwriteHardLinkTarget(t *testing.T) {
	repo, outside := t.TempDir(), t.TempDir()
	testutil.Put(t, outside, "keep.txt", "keep external\n")
	testutil.OK(t, os.MkdirAll(filepath.Join(repo, ".devkit/backlog"), 0755))
	file := ".devkit/backlog/feature%2Fx.json"
	testutil.OK(t, os.Link(filepath.Join(outside, "keep.txt"), filepath.Join(repo, file)))
	testutil.OK(t, saveBacklog(repo, file, backlog(nil, nil)))
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(outside, "keep.txt")))), "keep external\n")
	testutil.Equal(t, must(read(filepath.Join(repo, file))), backlog(nil, nil))
}

func TestBacklogSaveKeepsExternalFilesUnchangedDuringLinkReplacement(t *testing.T) {
	for _, ancestor := range []bool{false, true} {
		t.Run(map[bool]string{false: "destination", true: "ancestor"}[ancestor], func(t *testing.T) {
			repo, outside := t.TempDir(), t.TempDir()
			file := ".devkit/backlog/feature%2Fx.json"
			testutil.Put(t, outside, "feature%2Fx.json", "keep external\n")
			testutil.OK(t, os.MkdirAll(filepath.Join(repo, ".devkit/backlog"), 0755))
			path := filepath.Join(repo, file)
			target := filepath.Join(outside, "feature%2Fx.json")
			if ancestor {
				path, target = filepath.Dir(path), outside
			}
			started, done := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				close(started)
				for range 200 {
					os.RemoveAll(path)
					os.Symlink(target, path)
					os.Remove(path)
					if ancestor {
						os.Mkdir(path, 0755)
					}
				}
			}()
			<-started
			for range 200 {
				saveBacklog(repo, file, backlog(nil, nil))
			}
			<-done
			testutil.Equal(t, string(must(os.ReadFile(filepath.Join(outside, "feature%2Fx.json")))), "keep external\n")
			entries := must(os.ReadDir(outside))
			testutil.Equal(t, len(entries), 1)
		})
	}
}
