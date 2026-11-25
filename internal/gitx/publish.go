package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
)

type Publication struct {
	Branch, BaseBranch, Title, Body, State string
	Recorded                               map[string]bool
}

func PublicationCommand(cwd string, env cli.Env, body, name string, args ...string) (string, error) {
	executable := name
	if name == "gh" && env.Getenv("DEVKIT_GH") != "" {
		executable = env.Getenv("DEVKIT_GH")
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env, cmd.Stderr = cwd, env.Environ, env.Stderr
	if name == "gh" {
		cmd.Stdin = strings.NewReader(body)
	}
	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Command failed: %s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

func PushKind(remote, head string, isAncestor func(string, string) bool, recorded map[string]bool) string {
	if remote == "" {
		return "new"
	}
	if remote == head || isAncestor(remote, head) {
		return "forward"
	}
	if recorded[remote] {
		return "replace"
	}
	return ""
}

func (g *Git) Publish(env cli.Env, p Publication, beforePush func() (string, error)) (string, string, error) {
	head, err := g.Head()
	if err != nil {
		return "", "", err
	}
	run := func(name string, args ...string) (string, error) {
		return PublicationCommand(g.Cwd, env, p.Body, name, args...)
	}
	remoteOutput, err := run("git", "ls-remote", "--heads", "origin", "refs/heads/"+p.Branch)
	if err != nil {
		return "", "", err
	}
	remote := ""
	if fields := strings.Fields(remoteOutput); len(fields) > 0 {
		remote = fields[0]
	}
	kind := PushKind(remote, head, g.IsAncestor, p.Recorded)
	if kind == "" {
		return "", "", fmt.Errorf("origin/%s is at %s, which HEAD does not contain and this request did not record", p.Branch, remote)
	}
	if beforePush != nil {
		head, err = beforePush()
		if err != nil {
			return "", "", err
		}
	}
	push := []string{"push", "-u"}
	if kind == "replace" {
		push = append(push, "--force-with-lease=refs/heads/"+p.Branch+":"+remote)
	}
	if kind != "forward" || remote != head {
		if _, err = run("git", append(push, "origin", "HEAD:refs/heads/"+p.Branch)...); err != nil {
			return "", "", err
		}
	}
	address, err := run("gh", "pr", "list", "--head", p.Branch, "--state", p.State, "--json", "url", "--jq", ".[0].url // empty")
	if err != nil {
		return "", "", err
	}
	if address != "" {
		_, err = run("gh", "pr", "edit", address, "--title", p.Title, "--body-file", "-")
	} else {
		address, err = run("gh", "pr", "create", "--base", p.BaseBranch, "--head", p.Branch, "--title", p.Title, "--body-file", "-")
	}
	return address, head, err
}
