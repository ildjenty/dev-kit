package status

import (
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestApprovedProposalPointsToItsPullRequest(t *testing.T) {
	for _, workflow := range []string{"refine", "retro", "optimize"} {
		b := Branch{Branch: workflow + "/topic", Workflow: workflow, Outcome: "pass", Limit: 3}
		got := next(b, record.Document{"outcome": "pass", "pr_url": "https://github.example/team/repo/pull/1"})
		testutil.Contains(t, got, "published as https://github.example/team/repo/pull/1")
		testutil.Contains(t, got, "merges that pull request")
		testutil.Equal(t, strings.Contains(got, "Publish "), false)
		missing := next(b, record.Document{"outcome": "pass"})
		testutil.Contains(t, missing, "no pull request is recorded")
	}
}
