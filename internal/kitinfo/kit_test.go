package kitinfo

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func TestDigestUsesOnlyPathsAndContents(t *testing.T) {
	first := fstest.MapFS{"b": {Data: []byte("two"), Mode: 0600}, "a": {Data: []byte("one"), ModTime: time.Now()}}
	second := fstest.MapFS{"a": {Data: []byte("one"), Mode: 0755}, "b": {Data: []byte("two")}}
	original := testutil.Must(Digest(first))
	testutil.Equal(t, original, testutil.Must(Digest(second)))
	testutil.Equal(t, len(original), 64)
	second["a"].Data = []byte("changed")
	testutil.Equal(t, original == testutil.Must(Digest(second)), false)
	second["a"].Data = []byte("one")
	second["c"] = second["a"]
	delete(second, "a")
	testutil.Equal(t, original == testutil.Must(Digest(second)), false)
	testutil.Equal(t, EmbeddedDigest(), testutil.Must(Digest(kit.Files)))
	copied := fstest.MapFS{}
	testutil.OK(t, fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		copied[path] = &fstest.MapFile{Data: testutil.Must(fs.ReadFile(kit.Files, path)), Mode: 0600}
		return nil
	}))
	testutil.Equal(t, EmbeddedDigest(), testutil.Must(Digest(copied)))
}

func TestInvocationParsingSharesWorkflowAndSkillRules(t *testing.T) {
	files := fstest.MapFS{
		"dev/plan/plan.yaml":                {Data: []byte("bash: devkit record start plan\nother: devkit gone run\nnext: devkit plan facts2")},
		"dev/plan/commands/write.md":        {Data: []byte("Run `devkit plan check` and devkit gone run.")},
		"dev/plan/fixtures/main.stubs.yaml": {Data: []byte("devkit fixture before")},
		"dev/plan/README.md":                {Data: []byte("devkit prose ignored")},
		"dev/other/other.yml":               {Data: []byte("devkit other start")},
		"skills/run/SKILL.md":               {Data: []byte("devkit doctor; devkit version")},
		"skills/run/scripts/watch.sh":       {Data: []byte("devkit status \"$branch\"")},
	}
	testutil.Equal(t, testutil.Must(Invocations(files, "dev/plan", false)), []string{"gone run", "plan check", "plan facts2", "record start"})
	testutil.Equal(t, testutil.Must(Invocations(files, "skills/run", true)), []string{"doctor", "status", "version"})
	_, err := Invocations(files, "absent", false)
	testutil.Equal(t, err != nil, true)
}
