package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func IssueCLI(t testing.TB, state, label, blocker string, source ...string) string {
	t.Helper()
	dir := t.TempDir()
	issue := fmt.Sprintf(`{"number":42,"title":"Allow sign-in","body":"Allow employee sign-in.\nKeep the submitted request text.","state":%q,"url":"https://github.example/team/repo/issues/42","labels":[{"name":%q}],"parent":{"number":7,"title":"Audience","state":"OPEN","url":"https://github.example/team/repo/issues/7"}}`, state, label)
	if len(source) > 0 {
		issue = strings.Replace(issue, "Keep the submitted request text.", `Keep the submitted request text.\n\nSource: #80\n\n<!-- devkit:draft `+strings.Repeat("a", 64)+`\nsource_pr: 80 -->`, 1)
	}
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
	if len(source) > 0 {
		mergedAt := "null"
		if source[0] == "MERGED" {
			mergedAt = `"2026-11-25T01:00:00Z"`
		}
		pr := fmt.Sprintf(`{"number":80,"title":"Approved proposal","state":%q,"url":"https://github.example/team/repo/pull/80","mergedAt":%s}`, source[0], mergedAt)
		script = strings.Replace(script, `  "api "*)`, "  \"pr view 80\") cat <<'PR_JSON'\n"+pr+"\nPR_JSON\n    ;;\n"+`  "api "*)`, 1)
	}
	if state == "missing" {
		script = "#!/bin/sh\nexit 1\n"
	}
	Put(t, dir, "gh", script)
	path := filepath.Join(dir, "gh")
	OK(t, os.Chmod(path, 0755))
	t.Setenv("DEVKIT_GH", path)
	return path
}

func IssueCreationCLI(t testing.TB) (string, string) {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
state='` + dir + `'
printf '%s\n' "$*" >> "$state/calls"
case "$1 $2" in
 "label create") printf '%s\n' "$3" >> "$state/labels" ;;
 "pr list") test ! -f "$state/pr" || cat "$state/pr" ;;
 "pr create")
  branch=$(git symbolic-ref HEAD)
  test "$(git rev-parse HEAD)" = "$(git ls-remote origin "$branch" | cut -f1)" || exit 98
  cat >> "$state/pr-bodies"
  printf 'https://github.example/team/repo/pull/80' > "$state/pr"
  test ! -f "$state/pr-lost" || exit 1
  cat "$state/pr" ;;
 "pr edit") cat >> "$state/pr-bodies" ;;
 "issue create")
  test -f "$state/labels" || exit 96
  body=$(cat)
  printf '%s\n' "$body" >> "$state/bodies"
  count=0
  test ! -f "$state/count" || count=$(cat "$state/count")
  count=$((count+1))
  test ! -f "$state/fail" || test "$(cat "$state/fail")" != "$count" || exit 1
  printf '%s' "$count" > "$state/count"
  printf 'https://github.example/team/repo/issues/%s\n' "$((100+count))"
  ;;
 "issue close"|"issue comment") cat >/dev/null; printf '{}' ;;
 "issue view")
  printf '{"number":7,"title":"Audience","body":"Who may sign in?","state":"OPEN","labels":[{"name":"devkit:topic"}],"url":"https://github.example/team/repo/issues/7"}' ;;
 "api "*)
  case "$*" in
   *--method*) printf '{}' ;;
   *per_page*) printf '[[]]' ;;
   *) printf '{"id":9001}' ;;
  esac ;;
 *) exit 97 ;;
esac
`
	Put(t, dir, "gh", script)
	path := filepath.Join(dir, "gh")
	OK(t, os.Chmod(path, 0755))
	t.Setenv("DEVKIT_GH", path)
	return dir, path
}

func PublicationRemote(t testing.TB, cwd string) string {
	t.Helper()
	remote := t.TempDir()
	Git(t, remote, "init", "-q", "--bare")
	Git(t, cwd, "remote", "add", "origin", remote)
	return remote
}
