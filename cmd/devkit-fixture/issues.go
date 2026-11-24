package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
)

func fixtureGitHub(args []string, env cli.Env) error {
	if err := guard(env); err != nil {
		return err
	}
	if len(args) >= 3 && args[0] == "issue" && args[1] == "view" {
		number, err := strconv.Atoi(args[2])
		if err != nil || number == 45 {
			return fmt.Errorf("issue %s does not exist", args[2])
		}
		state, label := "OPEN", "devkit:unit"
		if number == 43 {
			state = "CLOSED"
		}
		if number == 46 {
			label = "other"
		}
		issue := doc{"number": number, "title": "Update fixture", "body": "Update fixture.txt", "state": state, "labels": []doc{{"name": label}}, "url": fmt.Sprintf("https://github.example/team/repo/issues/%d", number)}
		if number == 7 {
			issue["title"], issue["body"] = "Audience", "Who can use the result?"
		} else {
			issue["parent"] = doc{"number": 7, "title": "Audience", "state": "OPEN", "url": "https://github.example/team/repo/issues/7"}
		}
		return env.JSON(issue)
	}
	if len(args) >= 2 && args[0] == "api" {
		if strings.Contains(args[1], "/44/dependencies/blocked_by?") {
			return env.JSON([][]doc{{{"number": 42, "state": "open", "title": "Update fixture", "html_url": "https://github.example/team/repo/issues/42"}}})
		}
		return env.JSON([][]doc{{}})
	}
	return fmt.Errorf("unexpected fixture GitHub arguments: %v", args)
}
