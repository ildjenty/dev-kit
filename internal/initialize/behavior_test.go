package initialize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func invoke(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var out, stderr bytes.Buffer
	code := cli.New("test", Commands()).Execute(append([]string{"init"}, args...), cli.Env{Cwd: dir, Stdin: strings.NewReader("y\ny\ny\ny\ny\ny\n"), Stdout: &out, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("init %v: exit %d: %s\n%s", args, code, stderr.String(), out.String())
	}
	return out.String()
}

func installed(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	invoke(t, dir)
	return dir
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := testutil.Must(filepath.Rel(root, path))
		if entry.Type()&os.ModeSymlink != 0 {
			result[name] = "link:" + testutil.Must(os.Readlink(path))
		} else if entry.IsDir() {
			result[name] = "directory"
		} else {
			result[name] = string(testutil.Must(os.ReadFile(path)))
		}
		return nil
	}))
	return result
}

func TestFreshInstallAndIdenticalFilesAreSilent(t *testing.T) {
	dir := t.TempDir()
	out := invoke(t, dir, "repo")
	testutil.Equal(t, strings.Contains(out, "[y/N]"), false)
	root := filepath.Join(dir, "repo")
	testutil.OK(t, fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, path)))), string(testutil.Must(kit.Files.ReadFile(path))))
		return nil
	}))
	testutil.Contains(t, out, "run the devkit-optimize skill")
	testutil.Equal(t, strings.Split(invoke(t, dir, "repo"), "[required]")[0], "")
	for _, path := range []string{"docs", "embed.go", ".archon/workflows/dev/requirements/fixtures/executed"} {
		_, err := os.Stat(filepath.Join(root, path))
		testutil.Equal(t, os.IsNotExist(err), true)
	}
}

func TestEveryGroupYesAndNo(t *testing.T) {
	for _, tc := range []struct{ root, file string }{
		{".archon/workflows/dev", "requirements/requirements.yaml"},
		{".claude/skills/devkit-runs", "SKILL.md"},
		{".claude/skills/devkit-discuss", "SKILL.md"},
		{".devkit/standards", "authority.md"},
		{".devkit/config.json", ""},
		{".archon/config.yaml", ""},
	} {
		for _, answer := range []string{"y", "n"} {
			t.Run(tc.root+"/"+answer, func(t *testing.T) {
				dir := installed(t)
				path := tc.root
				if tc.file != "" {
					path += "/" + tc.file
				}
				want := string(testutil.Must(os.ReadFile(filepath.Join(dir, path))))
				custom := "custom"
				if tc.root == ".devkit/config.json" {
					custom = strings.Replace(want, `"docs": "docs"`, `"docs": "custom-docs"`, 1)
				}
				testutil.Put(t, dir, path, custom)
				if tc.file != "" {
					testutil.Put(t, dir, tc.root+"/old/nested.txt", "stale")
				}
				testutil.Put(t, dir, "outside.txt", "mine")
				var out bytes.Buffer
				err := run(nil, cli.Env{Cwd: dir, Stdin: strings.NewReader(answer + "\n"), Stdout: &out}, true)
				testutil.OK(t, err)
				testutil.Equal(t, strings.Count(out.String(), "[y/N]"), 1)
				testutil.Contains(t, out.String(), tc.root+": ")
				got := string(testutil.Must(os.ReadFile(filepath.Join(dir, path))))
				if answer == "y" {
					testutil.Equal(t, got, want)
					testutil.Contains(t, out.String(), "Wrote: "+path)
					if tc.file != "" {
						_, err := os.Lstat(filepath.Join(dir, tc.root, "old"))
						testutil.Equal(t, os.IsNotExist(err), true)
						testutil.Contains(t, out.String(), "Deleted: "+tc.root+"/old/nested.txt")
					}
				} else {
					testutil.Equal(t, got, custom)
					testutil.Contains(t, out.String(), "Left alone: "+path)
					if tc.file != "" {
						testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, tc.root, "old/nested.txt")))), "stale")
					}
				}
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, "outside.txt")))), "mine")
			})
		}
	}
}

func TestMissingFilesWrittenWithoutQuestions(t *testing.T) {
	dir := installed(t)
	for _, path := range []string{".archon/workflows/dev/README.md", ".claude/skills/devkit-runs/SKILL.md", ".claude/skills/devkit-discuss/SKILL.md", ".archon/config.yaml", ".devkit/config.json"} {
		testutil.OK(t, os.Remove(filepath.Join(dir, path)))
	}
	out := invoke(t, dir, "--no")
	testutil.Equal(t, strings.Contains(out, "[y/N]"), false)
	testutil.Contains(t, out, "Wrote: .archon/workflows/dev/README.md")
	testutil.Contains(t, out, "Wrote: .claude/skills/devkit-runs/SKILL.md")
	testutil.Contains(t, out, "Wrote: .claude/skills/devkit-discuss/SKILL.md")
	testutil.Contains(t, out, "Wrote: .archon/config.yaml")
	testutil.Contains(t, out, "Wrote: .devkit/config.json")
}

func TestOwnedGroupNoStillAddsMissingFiles(t *testing.T) {
	dir := installed(t)
	testutil.Put(t, dir, ".claude/skills/devkit-runs/SKILL.md", "custom")
	testutil.OK(t, os.Remove(filepath.Join(dir, ".claude/skills/devkit-runs/scripts/watch.sh")))
	out := invoke(t, dir, "--no")
	testutil.Contains(t, out, "replace 1 files and delete 0 files? [y/N] No")
	testutil.Contains(t, out, "Wrote: .claude/skills/devkit-runs/scripts/watch.sh")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/devkit-runs/SKILL.md")))), "custom")
}

func TestStandardsNoLeavesEntireConfiguredFolderUntouched(t *testing.T) {
	for _, onlyMissing := range []bool{true, false} {
		t.Run(fmt.Sprint(onlyMissing), func(t *testing.T) {
			dir := installed(t)
			testutil.OK(t, os.Rename(filepath.Join(dir, ".devkit/standards"), filepath.Join(dir, "policy")))
			testutil.Put(t, dir, ".devkit/config.json", `{"standards":"policy","language":"fr"}`)
			testutil.OK(t, os.Remove(filepath.Join(dir, "policy/authority.md")))
			if !onlyMissing {
				testutil.Put(t, dir, "policy/concerns.json", "mine")
				testutil.Put(t, dir, "policy/extra", "mine")
			}
			before := snapshot(t, filepath.Join(dir, "policy"))
			out := invoke(t, dir, "--no")
			testutil.Contains(t, out, "policy: replace it with the kit's standards? [y/N] No")
			testutil.Equal(t, snapshot(t, filepath.Join(dir, "policy")), before)
			out = invoke(t, dir, "--yes")
			testutil.Contains(t, out, "Wrote: policy/authority.md")
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, "policy/authority.md")))), string(testutil.Must(kit.Files.ReadFile(".devkit/standards/authority.md"))))
			_, err := os.Stat(filepath.Join(dir, ".devkit/standards"))
			testutil.Equal(t, os.IsNotExist(err), true)
			settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json")))))
			testutil.Equal(t, settings.Standards, "policy")
			testutil.Equal(t, strings.Split(invoke(t, dir), "[required]")[0], "")
		})
	}
}

func TestRenamedSkillsAndReuse(t *testing.T) {
	dir := t.TempDir()
	invoke(t, dir, "--skill", "devkit-runs=my-runs", "--skill=devkit-discuss=my-discuss")
	testutil.OK(t, fs.WalkDir(kit.Files, ".claude/skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		replacer := strings.NewReplacer("devkit-runs", "my-runs", "devkit-discuss", "my-discuss")
		got := string(testutil.Must(os.ReadFile(filepath.Join(dir, replacer.Replace(path)))))
		testutil.Equal(t, got, replacer.Replace(string(testutil.Must(kit.Files.ReadFile(path)))))
		testutil.Equal(t, strings.Contains(got, "devkit-runs") || strings.Contains(got, "devkit-discuss"), false)
		return nil
	}))
	runs := string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/my-runs/SKILL.md"))))
	testutil.Contains(t, runs, "name: my-runs")
	testutil.Contains(t, runs, ".claude/skills/my-runs/scripts/watch.sh")
	testutil.Contains(t, runs, "`my-discuss`")
	settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json")))))
	testutil.Equal(t, settings.Skills, map[string]string{"devkit-runs": "my-runs", "devkit-discuss": "my-discuss", "devkit-optimize": "devkit-optimize"})
	testutil.Equal(t, strings.Split(invoke(t, dir), "[required]")[0], "")
	testutil.OK(t, os.Remove(filepath.Join(dir, ".claude/skills/my-runs/scripts/watch.sh")))
	out := invoke(t, dir)
	testutil.Contains(t, out, "Wrote: .claude/skills/my-runs/scripts/watch.sh")
	_, err := os.Stat(filepath.Join(dir, ".claude/skills/devkit-runs"))
	testutil.Equal(t, os.IsNotExist(err), true)
	guide := string(testutil.Must(os.ReadFile(filepath.Join(dir, ".archon/workflows/dev/README.md"))))
	testutil.Contains(t, guide, "`my-runs`")
}

func TestFirstInstallNamesTheConfiguredOptimizeSkillAndInstallsIt(t *testing.T) {
	dir := t.TempDir()
	out := invoke(t, dir, "--skill", "devkit-optimize=fit-repo")
	testutil.Contains(t, out, "run the fit-repo skill")
	skill := string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/fit-repo/SKILL.md"))))
	testutil.Contains(t, skill, "name: fit-repo")
	_, err := os.Stat(filepath.Join(dir, ".claude/skills/devkit-optimize"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.Equal(t, strings.Contains(invoke(t, dir), "Next: in a session"), false)
}
func TestRenameNeverOffersAFolderDevkitDidNotInstallForDeletion(t *testing.T) {
	dir := t.TempDir()
	own := filepath.Join(dir, ".claude/skills/devkit-runs/SKILL.md")
	testutil.OK(t, os.MkdirAll(filepath.Dir(own), 0755))
	testutil.OK(t, os.WriteFile(own, []byte("the repository's own skill\n"), 0644))
	invoke(t, dir, "--no")
	out := invoke(t, dir, "--yes", "--skill", "devkit-runs=kit-runs")
	testutil.Contains(t, out, "Left alone: .claude/skills/devkit-runs holds files devkit did not install")
	testutil.Equal(t, strings.Contains(out, "Deleted: .claude/skills/devkit-runs"), false)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(own))), "the repository's own skill\n")
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/kit-runs/SKILL.md")))), "name: kit-runs")
}
func TestRenameOffersPreviousFolderForDeletionAndConfigNoUpdatesOnlySkills(t *testing.T) {
	for _, flag := range []string{"--yes", "--no"} {
		t.Run(flag, func(t *testing.T) {
			dir := installed(t)
			before := "{\n\t\"language\" : \"fr\",\n\t\"checks\": [],\n\t\"skills\": {\"devkit-runs\":\"devkit-runs\", \"devkit-discuss\":\"devkit-discuss\"}\n}\n"
			testutil.Put(t, dir, ".devkit/config.json", before)
			testutil.Put(t, dir, ".claude/skills/unrelated/SKILL.md", "mine")
			out := invoke(t, dir, flag, "--skill", "devkit-runs=delivery")
			testutil.Contains(t, out, ".claude/skills/devkit-runs: replace 0 files and delete 2 files? [y/N]")
			after := string(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json"))))
			if flag == "--no" {
				want := strings.Replace(before, `{"devkit-runs":"devkit-runs", "devkit-discuss":"devkit-discuss"}`, `{"devkit-discuss":"devkit-discuss","devkit-optimize":"devkit-optimize","devkit-runs":"delivery"}`, 1)
				testutil.Equal(t, after, want)
				testutil.Contains(t, out, "Left alone: .claude/skills/devkit-runs/SKILL.md")
			} else {
				_, err := os.Stat(filepath.Join(dir, ".claude/skills/devkit-runs"))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
			testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/delivery/SKILL.md")))), "name: delivery")
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/unrelated/SKILL.md")))), "mine")
			out = invoke(t, dir, "--no")
			testutil.Equal(t, strings.Contains(out, ".claude/skills/devkit-runs/"), false)
			settings := testutil.Must(config.Parse([]byte(after)))
			testutil.Equal(t, settings.Skills["devkit-runs"], "delivery")
		})
	}
}

func TestSkillNamesCanSwapWithoutDeletingActiveFolders(t *testing.T) {
	dir := installed(t)
	out := invoke(t, dir, "--yes", "--skill", "devkit-runs=devkit-discuss", "--skill", "devkit-discuss=devkit-runs")
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/devkit-discuss/SKILL.md")))), "name: devkit-discuss")
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".claude/skills/devkit-discuss/SKILL.md")))), ".claude/skills/devkit-discuss/scripts/watch.sh")
	testutil.Equal(t, strings.Contains(out, "Deleted: .claude/skills/devkit-discuss/SKILL.md"), false)
	testutil.Equal(t, strings.Split(invoke(t, dir), "[required]")[0], "")
}

func TestNonterminalAndFlags(t *testing.T) {
	for _, flags := range [][]string{nil, {"--no"}, {"--yes"}} {
		t.Run(fmt.Sprint(flags), func(t *testing.T) {
			dir := installed(t)
			path := ".claude/skills/devkit-runs/SKILL.md"
			testutil.Put(t, dir, path, "mine")
			out := invoke(t, dir, flags...)
			if len(flags) == 0 {
				testutil.Contains(t, out, "Stdin is not a terminal; all questions default to No.")
			}
			got := string(testutil.Must(os.ReadFile(filepath.Join(dir, path))))
			if len(flags) > 0 && flags[0] == "--yes" {
				testutil.Equal(t, got, string(testutil.Must(kit.Files.ReadFile(path))))
			} else {
				testutil.Equal(t, got, "mine")
			}
		})
	}
	file := testutil.Must(os.Open(os.DevNull))
	defer file.Close()
	testutil.Equal(t, terminal(file), false)
}

func TestInteractiveDefaultNoAndSeparateAnswers(t *testing.T) {
	for _, answers := range []string{"\ny\n", "n\nYES\n", ""} {
		t.Run(fmt.Sprintf("%q", answers), func(t *testing.T) {
			dir := installed(t)
			runs, discuss := ".claude/skills/devkit-runs/SKILL.md", ".claude/skills/devkit-discuss/SKILL.md"
			testutil.Put(t, dir, runs, "runs")
			testutil.Put(t, dir, discuss, "discuss")
			var out bytes.Buffer
			testutil.OK(t, run(nil, cli.Env{Cwd: dir, Stdin: strings.NewReader(answers), Stdout: &out}, true))
			testutil.Equal(t, strings.Count(out.String(), "[y/N]"), 2)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, runs)))), "runs")
			if answers != "" {
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, discuss)))), string(testutil.Must(kit.Files.ReadFile(discuss))))
			}
		})
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	for _, flags := range [][]string{{"--dry-run"}, {"--dry-run", "--yes"}, {"--dry-run", "--no"}} {
		t.Run(fmt.Sprint(flags), func(t *testing.T) {
			dir := installed(t)
			testutil.Put(t, dir, ".archon/workflows/dev/README.md", "mine")
			testutil.Put(t, dir, ".archon/workflows/dev/extra", "extra")
			testutil.OK(t, os.Remove(filepath.Join(dir, ".devkit/standards/authority.md")))
			before := snapshot(t, dir)
			out := invoke(t, dir, append(flags, "--skill", "devkit-runs=delivery")...)
			testutil.Contains(t, out, "replace 1 files and delete 1 files? [y/N] (dry-run)")
			testutil.Contains(t, out, "replace it with the kit's standards? [y/N] (dry-run)")
			testutil.Contains(t, out, "overwrite this file? [y/N] (dry-run)")
			testutil.Equal(t, snapshot(t, dir), before)
		})
	}
	dir := t.TempDir()
	invoke(t, dir, "--dry-run", "missing")
	testutil.Equal(t, snapshot(t, dir), map[string]string{".": "directory"})
}

func TestSymbolicLinksYesAndNo(t *testing.T) {
	for _, path := range []string{".archon", ".archon/workflows/dev", ".archon/workflows/dev/README.md", ".claude/skills/devkit-runs/scripts", ".claude/skills/devkit-discuss/SKILL.md", ".devkit", ".devkit/standards", ".devkit/standards/authority.md", ".devkit/config.json", ".archon/config.yaml"} {
		for _, flag := range []string{"--yes", "--no", "--dry-run"} {
			t.Run(path+flag, func(t *testing.T) {
				dir := installed(t)
				outside := t.TempDir()
				target := filepath.Join(outside, "target")
				testutil.OK(t, os.Rename(filepath.Join(dir, path), target))
				testutil.OK(t, os.Symlink(target, filepath.Join(dir, path)))
				before := snapshot(t, outside)
				out := invoke(t, dir, flag)
				testutil.Contains(t, out, "Warning: symbolic link: "+path)
				testutil.Contains(t, out, "Replace symbolic link "+path+" itself")
				testutil.Equal(t, snapshot(t, outside), before)
				info := testutil.Must(os.Lstat(filepath.Join(dir, path)))
				testutil.Equal(t, info.Mode()&os.ModeSymlink != 0, flag != "--yes")
			})
		}
	}
}

func TestDanglingAndStaleLinks(t *testing.T) {
	for _, flag := range []string{"--yes", "--no"} {
		t.Run(flag, func(t *testing.T) {
			dir := installed(t)
			target := filepath.Join(t.TempDir(), "absent")
			path := ".archon/workflows/dev/obsolete"
			testutil.OK(t, os.Symlink(target, filepath.Join(dir, path)))
			testutil.OK(t, os.Remove(filepath.Join(dir, ".claude/skills/devkit-runs/SKILL.md")))
			testutil.OK(t, os.Symlink(target, filepath.Join(dir, ".claude/skills/devkit-runs/SKILL.md")))
			invoke(t, dir, flag)
			_, err := os.Lstat(filepath.Join(dir, path))
			testutil.Equal(t, os.IsNotExist(err), flag == "--yes")
			_, err = os.Lstat(target)
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestStandardsLinkNoLeavesMissingFilesUntouched(t *testing.T) {
	dir := installed(t)
	testutil.OK(t, os.Remove(filepath.Join(dir, ".devkit/standards/authority.md")))
	path := filepath.Join(dir, ".devkit/standards/concerns.json")
	outside := t.TempDir()
	testutil.OK(t, os.Rename(path, filepath.Join(outside, "concerns.json")))
	testutil.OK(t, os.Symlink(filepath.Join(outside, "concerns.json"), path))
	before := snapshot(t, filepath.Join(dir, ".devkit/standards"))
	var out bytes.Buffer
	testutil.OK(t, run(nil, cli.Env{Cwd: dir, Stdin: strings.NewReader("n\ny\n"), Stdout: &out}, true))
	testutil.Equal(t, snapshot(t, filepath.Join(dir, ".devkit/standards")), before)
}

func TestInvalidArgumentsAndConfigWriteNothing(t *testing.T) {
	for _, args := range [][]string{{"--force"}, {"--yes", "--no"}, {"--skill"}, {"--skill", "unknown=x"}, {"--skill", "devkit-runs=Bad"}, {"--skill", "devkit-runs=../outside"}, {"--skill", "devkit-runs="}, {"--skill", "devkit-runs=devkit-discuss"}, {"one", "two"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			dir := t.TempDir()
			var out, stderr bytes.Buffer
			code := cli.New("test", Commands()).Execute(append([]string{"init"}, args...), cli.Env{Cwd: dir, Stdout: &out, Stderr: &stderr})
			if code == 0 {
				t.Fatal("expected rejection")
			}
			testutil.Equal(t, snapshot(t, dir), map[string]string{".": "directory"})
		})
	}
	dir := t.TempDir()
	testutil.Put(t, dir, ".devkit/config.json", `{"standards":"../outside"}`)
	before := snapshot(t, dir)
	var out bytes.Buffer
	if err := run([]string{"--yes"}, cli.Env{Cwd: dir, Stdout: &out}, false); err == nil {
		t.Fatal("expected invalid destination error")
	}
	testutil.Equal(t, snapshot(t, dir), before)
}

type observingReader func([]byte) (int, error)

func (r observingReader) Read(p []byte) (int, error) { return r(p) }

func TestAllComparisonAndQuestionsPrecedeWrites(t *testing.T) {
	dir := installed(t)
	testutil.Put(t, dir, ".claude/skills/devkit-runs/SKILL.md", "mine")
	testutil.Put(t, dir, ".archon/config.yaml", "mine")
	testutil.OK(t, os.Remove(filepath.Join(dir, ".archon/workflows/dev/README.md")))
	before := snapshot(t, dir)
	var out bytes.Buffer
	reads := 0
	reader := observingReader(func(p []byte) (int, error) {
		testutil.Equal(t, snapshot(t, dir), before)
		testutil.Contains(t, out.String(), "Different: .archon/config.yaml")
		reads++
		if reads > 2 {
			return 0, io.EOF
		}
		return copy(p, "y\n"), nil
	})
	testutil.OK(t, run(nil, cli.Env{Cwd: dir, Stdout: &out, Stdin: reader}, true))
	testutil.Equal(t, reads, 2)
}

func TestWithSkillsPreservesOtherKeys(t *testing.T) {
	for _, input := range []string{`{}`, "{ \"language\":\"fr\" }\n", "{\n  \"skills\": {\"devkit-runs\":\"old\"},\n  \"language\": \"fr\"\n}\n"} {
		data := testutil.Must(withSetting([]byte(input), "skills", []byte(`{"devkit-discuss":"talk","devkit-runs":"run"}`)))
		var before, after map[string]json.RawMessage
		testutil.OK(t, json.Unmarshal([]byte(input), &before))
		testutil.OK(t, json.Unmarshal(data, &after))
		delete(before, "skills")
		delete(after, "skills")
		testutil.Equal(t, after, before)
	}
}

func TestRefusedStaleLinkDoesNotFailParentDeletion(t *testing.T) {
	dir := installed(t)
	testutil.Put(t, dir, ".archon/workflows/dev/old/file", "stale")
	link := ".archon/workflows/dev/old/link"
	testutil.OK(t, os.Symlink("missing", filepath.Join(dir, link)))
	var out bytes.Buffer
	testutil.OK(t, run(nil, cli.Env{Cwd: dir, Stdin: strings.NewReader("n\ny\n"), Stdout: &out}, true))
	testutil.Contains(t, out.String(), "Left alone: "+link)
	testutil.Equal(t, testutil.Must(os.Readlink(filepath.Join(dir, link))), "missing")
	_, err := os.Stat(filepath.Join(dir, ".archon/workflows/dev/old/file"))
	testutil.Equal(t, os.IsNotExist(err), true)
}

func TestConfigNoWithExplicitSkillAndApprovedLink(t *testing.T) {
	dir := installed(t)
	target := filepath.Join(t.TempDir(), "config.json")
	testutil.Put(t, filepath.Dir(target), "config.json", `{"language":"fr"}`)
	testutil.OK(t, os.Remove(filepath.Join(dir, ".devkit/config.json")))
	testutil.OK(t, os.Symlink(target, filepath.Join(dir, ".devkit/config.json")))
	var out bytes.Buffer
	testutil.OK(t, run([]string{"--skill", "devkit-runs=delivery"}, cli.Env{Cwd: dir, Stdin: strings.NewReader("y\nn\nn\nn\nn\n"), Stdout: &out}, true))
	got := testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json")))
	settings := testutil.Must(config.Parse(got))
	testutil.Equal(t, settings.Skills["devkit-runs"], "delivery")
	testutil.Equal(t, settings.Language, "fr")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(target))), `{"language":"fr"}`)
}

func TestDeclinedGroupKeepsSharedAncestorLink(t *testing.T) {
	dir := installed(t)
	target := filepath.Join(t.TempDir(), "devkit")
	testutil.OK(t, os.Rename(filepath.Join(dir, ".devkit"), target))
	testutil.OK(t, os.Symlink(target, filepath.Join(dir, ".devkit")))
	before := snapshot(t, target)
	var out bytes.Buffer
	testutil.OK(t, run(nil, cli.Env{Cwd: dir, Stdin: strings.NewReader("y\nn\ny\n"), Stdout: &out}, true))
	testutil.Equal(t, testutil.Must(os.Readlink(filepath.Join(dir, ".devkit"))), target)
	testutil.Equal(t, snapshot(t, target), before)
}

func TestMalformedConfigCanBeKeptOrReplaced(t *testing.T) {
	for _, raw := range []string{"invalid", "null", "[]"} {
		for _, flag := range []string{"--yes", "--no"} {
			t.Run(raw+flag, func(t *testing.T) {
				dir := installed(t)
				testutil.Put(t, dir, ".devkit/config.json", raw)
				out := invoke(t, dir, flag)
				testutil.Contains(t, out, "Warning: .devkit/config.json is not a valid JSON object")
				testutil.Contains(t, out, ".devkit/config.json: overwrite this file? [y/N]")
				got := string(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json"))))
				if flag == "--yes" {
					testutil.Equal(t, got, string(testutil.Must(kit.Files.ReadFile(".devkit/config.json"))))
				} else {
					testutil.Equal(t, got, raw)
				}
			})
		}
	}
	dir := installed(t)
	testutil.Put(t, dir, ".devkit/config.json", "invalid")
	before := snapshot(t, dir)
	var out bytes.Buffer
	err := run([]string{"--no", "--skill", "devkit-runs=delivery"}, cli.Env{Cwd: dir, Stdout: &out}, false)
	testutil.Fails(t, err, "cannot update only skills")
	testutil.Equal(t, snapshot(t, dir), before)
	invoke(t, dir, "--yes", "--skill", "devkit-runs=delivery")
	settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/config.json")))))
	testutil.Equal(t, settings.Skills["devkit-runs"], "delivery")
}
