package record

import (
	"fmt"
	"strconv"

	"github.com/ildjenty/dev-kit/internal/cli"
)

func RequestIssue(dir string) (int, error) {
	requests, err := List(dir, "request")
	if err != nil {
		return 0, err
	}
	issue := 0
	for _, request := range requests {
		if n, ok := request["issue"].(float64); ok && n > 0 {
			issue = int(n)
		}
	}
	return issue, nil
}

func InputIssue(env cli.Env, dir string) (int, error) {
	value := env.Getenv("INPUTS_ISSUE")
	if value == "" {
		return RequestIssue(dir)
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("issue must be a positive issue number: %q", value)
	}
	return number, nil
}
