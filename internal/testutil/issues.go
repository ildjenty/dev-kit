package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func IssueCLI(t testing.TB, state, label, blocker string) string {
	t.Helper()
	dir := t.TempDir()
	issue := fmt.Sprintf(`{"number":42,"title":"Allow sign-in","body":"Allow employee sign-in.\nKeep the submitted request text.","state":%q,"url":"https://github.example/team/repo/issues/42","labels":[{"name":%q}],"parent":{"number":7,"title":"Audience","state":"OPEN","url":"https://github.example/team/repo/issues/7"}}`, state, label)
	if state == "missing" {
		issue = ""
	}
	blocked := `[[]]`
	if blocker != "" {
		blocked = fmt.Sprintf(`[[{"number":8,"title":"Prepare access","state":%q,"html_url":"https://github.example/team/repo/issues/8"}]]`, blocker)
	}
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2 $3" in
  "issue view 42")
    test -n '%s' || exit 1
    cat <<'ISSUE_JSON'
%s
ISSUE_JSON
    ;;
  "issue view 7")
    cat <<'TOPIC_JSON'
{"number":7,"title":"Audience","body":"Who may sign in?\nEmployees only.","state":"OPEN","url":"https://github.example/team/repo/issues/7","labels":[{"name":"devkit:topic"}]}
TOPIC_JSON
    ;;
  *dependencies/blocked_by*)
    cat <<'BLOCKERS_JSON'
%s
BLOCKERS_JSON
    ;;
  "api "*) printf '[[]]' ;;
  *) exit 97 ;;
esac
`, state, issue, blocked)
	if state == "missing" {
		script = "#!/bin/sh\nexit 1\n"
	}
	Put(t, dir, "gh", script)
	path := filepath.Join(dir, "gh")
	OK(t, os.Chmod(path, 0755))
	t.Setenv("DEVKIT_GH", path)
	return path
}
