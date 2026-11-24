package issues

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
)

func Commands() []cli.Command {
	return []cli.Command{
		{Area: "issues", Name: "ready", Usage: "devkit issues ready [--json]", Run: func(args []string, env cli.Env) error {
			if len(args) > 1 || len(args) == 1 && args[0] != "--json" {
				return cli.Usage("devkit issues ready [--json]")
			}
			settings, err := config.Load(env.Cwd)
			if err != nil {
				return err
			}
			ready, err := (Client{env}).Ready(settings.Issues.Labels)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				return env.JSON(ready)
			}
			for _, group := range []struct {
				kind   string
				issues []ReadyIssue
			}{{"unit", ready.Units}, {"topic", ready.Topics}} {
				for _, issue := range group.issues {
					parent := ""
					if issue.Parent != nil {
						parent = fmt.Sprintf(" (parent #%d: %s)", issue.Parent.Number, issue.Parent.Title)
					}
					if _, err := fmt.Fprintf(env.Stdout, "%s #%d %s %s%s\n", group.kind, issue.Number, issue.Title, issue.URL, parent); err != nil {
						return err
					}
				}
			}
			return nil
		}},
		{Area: "issues", Name: "show", Usage: "devkit issues show <number> [--json]", Run: func(args []string, env cli.Env) error {
			if len(args) < 1 || len(args) > 2 || len(args) == 2 && args[1] != "--json" {
				return cli.Usage("devkit issues show <number> [--json]")
			}
			number, err := strconv.Atoi(args[0])
			if err != nil || number <= 0 {
				return cli.Usage("devkit issues show <number> [--json]")
			}
			issue, err := (Client{env}).Read(number)
			if err != nil {
				return err
			}
			if len(args) == 2 {
				return env.JSON(issue)
			}
			if _, err := fmt.Fprintf(env.Stdout, "#%d %s [%s]\n%s\nLabels: %s\n", issue.Number, issue.Title, issue.State, issue.URL, strings.Join(issue.Labels, ", ")); err != nil {
				return err
			}
			for _, relation := range []struct {
				name   string
				issues []Reference
			}{{"Sub-issue", issue.SubIssues}, {"Blocked by", issue.BlockedBy}, {"Blocking", issue.Blocking}} {
				for _, item := range relation.issues {
					if _, err := fmt.Fprintf(env.Stdout, "%s: #%d %s [%s] %s\n", relation.name, item.Number, item.Title, item.State, item.URL); err != nil {
						return err
					}
				}
			}
			if issue.Parent != nil {
				if _, err := fmt.Fprintf(env.Stdout, "Parent: #%d %s %s\n", issue.Parent.Number, issue.Parent.Title, issue.Parent.URL); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(env.Stdout, "\n"+issue.Body)
			return err
		}},
	}
}
