package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

func fixtureGitHub(args []string, env cli.Env) error {
	if err := guard(env); err != nil {
		return err
	}
	if len(args) >= 2 && (args[0] == "pr" || args[0] == "issue" && args[1] == "create") {
		path := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "github.json")
		state, err := read(path)
		if os.IsNotExist(err) {
			state = doc{"issues": 0}
			err = nil
		} else if err != nil {
			return err
		}
		if args[0] == "pr" && args[1] == "list" {
			if address, ok := state["pr_url"].(string); ok {
				_, err = fmt.Fprint(env.Stdout, address)
			}
			return err
		}
		body, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		if args[0] == "pr" {
			head, err := gitx.Run(env.Cwd, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			branch, err := gitx.Run(env.Cwd, "symbolic-ref", "HEAD")
			if err != nil {
				return err
			}
			remote, err := gitx.Run(env.Cwd, "ls-remote", "origin", branch)
			if err != nil || !strings.HasPrefix(remote, head+"\t") {
				return fmt.Errorf("approval did not push before PR publication: %s", remote)
			}
			state["pr_url"], state["pr_body"] = "https://github.example/team/repo/pull/80", string(body)
			if err = save(path, state); err != nil {
				return err
			}
			_, err = fmt.Fprint(env.Stdout, state["pr_url"])
			return err
		}
		if state["pr_url"] == nil {
			return fmt.Errorf("approval created an Issue before publishing its PR")
		}
		count, _ := state["issues"].(float64)
		number := int(count) + 101
		state["issues"] = number - 100
		if err = save(filepath.Join(env.Getenv("ARTIFACTS_DIR"), fmt.Sprintf("issue-%d.json", number)), doc{"body": string(body)}); err != nil {
			return err
		}
		if err = save(path, state); err != nil {
			return err
		}
		_, err = fmt.Fprintf(env.Stdout, "https://github.example/team/repo/issues/%d", number)
		return err
	}
	if len(args) >= 2 && args[0] == "issue" && (args[1] == "close" || args[1] == "comment") {
		return nil
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
		if strings.Contains(strings.Join(args, " "), "--method") {
			return env.JSON(doc{})
		}
		if !strings.Contains(args[1], "?") {
			return env.JSON(doc{"id": 9001})
		}
		if strings.Contains(args[1], "/44/dependencies/blocked_by?") {
			return env.JSON([][]doc{{{"number": 42, "state": "open", "title": "Update fixture", "html_url": "https://github.example/team/repo/issues/42"}}})
		}
		return env.JSON([][]doc{{}})
	}
	return fmt.Errorf("unexpected fixture GitHub arguments: %v", args)
}
