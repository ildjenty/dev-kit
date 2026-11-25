package main

import (
	"errors"
	"os/exec"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/cli"
)

func fixtureGit(args []string, env cli.Env) error {
	if (env.Getenv("INPUTS_APPROVE") != "" || env.Getenv("INPUTS_AI") == "none") && guard(env) == nil {
		for i, arg := range args {
			if arg == "origin" {
				args[i] = filepath.Join(env.Getenv("ARTIFACTS_DIR"), "origin.git")
			}
		}
	}
	cmd := exec.Command(env.Getenv("DEVKIT_FIXTURE_GIT"), args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env.Cwd, env.Environ, env.Stdin, env.Stdout, env.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &cli.ExitError{Code: exit.ExitCode()}
	}
	return err
}
