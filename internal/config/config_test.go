package config

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"
[commands]
test = { executable = "./scripts/test", args = ["--all"], timeout_seconds = 120 }
[judges]
review = { executable = "./scripts/judge", args = [], timeout_seconds = 60, output_schema = ".devkit/kit/v1/schemas/judgment.json" }
[deny]
rules = ["Bash(git push *)", "Agent(fork)", "mcp__service__write"]
judge = ["apply_patch", "mcp__service__write"]
[router]
human_confirmation = ["billing_changed"]
[external.comment]
executable = "./scripts/comment"
args = ["--json"]
timeout_seconds = 30
occurrence_field = "request_id"
query = { executable = "./scripts/find-comment", args = [], timeout_seconds = 10 }
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.KitVersion != "v1" || c.Commands["test"].Timeout() != 120*time.Second || c.Commands["test"].Args[0] != "--all" {
		t.Fatalf("configuration does not match: %+v", c)
	}
	if c.Judges["review"].Executable != "./scripts/judge" || c.Judges["review"].OutputSchema == "" {
		t.Fatalf("judge command does not match: %+v", c.Judges)
	}
	op := c.External["comment"]
	if op.Query == nil || op.Query.Timeout() != 10*time.Second || op.OccurrenceField != "request_id" {
		t.Fatalf("external operation does not match: %+v", op)
	}
	if len(c.Deny.Rules) != 3 || len(c.Deny.Judge) != 2 || c.Deny.Judge[0] != "apply_patch" || c.Router.HumanConfirmation[0] != "billing_changed" {
		t.Fatalf("rules do not match: %+v", c)
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"kit_missing", "", "kit_version"},
		{"syntax", "kit_version = [", "parse"},
		{"unknown", "kit_version = 'v1'\nmodel = 'x'", "unknown"},
		{"executable", "kit_version = 'v1'\n[commands.test]\ntimeout_seconds = 1", "executable"},
		{"timeout", "kit_version = 'v1'\n[commands.test]\nexecutable = 'test'\ntimeout_seconds = 0", "timeout_seconds"},
		{"timeout_overflow", "kit_version = 'v1'\n[commands.test]\nexecutable = 'test'\ntimeout_seconds = 9223372036854775807", "timeout_seconds"},
		{"schema", "kit_version = 'v1'\n[judges.review]\nexecutable = 'judge'\ntimeout_seconds = 1", "output_schema"},
		{"query", "kit_version = 'v1'\n[external.post]\nexecutable = 'post'\ntimeout_seconds = 1\nquery = { executable = 'query' }", "query.timeout_seconds"},
		{"deny", "kit_version = 'v1'\n[deny]\nrules = ['Bash(git push)']", "deny.rules"},
		{"agent_extra_close", "kit_version = 'v1'\n[deny]\nrules = ['Agent(fork))']", "deny.rules"},
		{"agent_extra_open", "kit_version = 'v1'\n[deny]\nrules = ['Agent((fork)']", "deny.rules"},
		{"agent_nested", "kit_version = 'v1'\n[deny]\nrules = ['Agent((fork))']", "deny.rules"},
		{"judge_empty", "kit_version = 'v1'\n[deny]\njudge = ['']", "deny.judge"},
		{"judge_command", "kit_version = 'v1'\n[deny]\njudge = ['Bash(git push *)']", "deny.judge"},
		{"judge_whitespace", "kit_version = 'v1'\n[deny]\njudge = ['apply patch']", "deny.judge"},
		{"router", "kit_version = 'v1'\n[router]\nhuman_confirmation = ['']", "human_confirmation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, expected field = %s", err, tt.want)
			}
		})
	}
}
