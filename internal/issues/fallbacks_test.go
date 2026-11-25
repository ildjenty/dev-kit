package issues

import (
	"strconv"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestExternalIssueListsRequireArrays(t *testing.T) {
	for _, body := range []string{"null", "{}", "[null]", "[{}]", "[[null]]", "[]", "[[]]"} {
		t.Run("references/"+body, func(t *testing.T) {
			c := fake(t, relationStep(42, "dependencies/blocked_by", body))
			items, err := c.references("repos/{owner}/{repo}/issues/42/dependencies/blocked_by")
			if body == "[]" || body == "[[]]" {
				testutil.OK(t, err)
				testutil.Equal(t, items, []Reference{})
			} else {
				testutil.Fails(t, err, "issues/42/dependencies/blocked_by")
			}
		})
		t.Run("drafts/"+body, func(t *testing.T) {
			c := fake(t, ghStep{Args: append(apiArgs("repos/{owner}/{repo}/issues?state=all&per_page=100"), "--paginate", "--slurp"), Output: body})
			_, found, err := c.findDraft("marker", "unit")
			if body == "[]" || body == "[[]]" {
				testutil.OK(t, err)
				testutil.Equal(t, found, false)
			} else {
				testutil.Fails(t, err, "issue drafts")
			}
		})
	}
	for _, body := range []string{"null", "{}", "[null]", "[]"} {
		c := fake(t, ghStep{Args: []string{"issue", "list", "--state", "open", "--limit", strconv.Itoa(int(^uint(0) >> 1)), "--json", fields}, Output: body})
		items, err := c.ListOpen("unit")
		if body == "[]" {
			testutil.OK(t, err)
			testutil.Equal(t, items, []Issue{})
		} else {
			testutil.Fails(t, err, "gh issue list")
		}
	}
}
