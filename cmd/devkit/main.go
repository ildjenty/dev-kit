package main

import (
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/doctor"
	"github.com/ildjenty/dev-kit/internal/fix"
	"github.com/ildjenty/dev-kit/internal/implement"
	"github.com/ildjenty/dev-kit/internal/initialize"
	"github.com/ildjenty/dev-kit/internal/judge"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/pr"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/refine"
	"github.com/ildjenty/dev-kit/internal/requirements"
	"github.com/ildjenty/dev-kit/internal/retro"
	"github.com/ildjenty/dev-kit/internal/sync"
)

var version = "dev"

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	registry := newRegistry()
	os.Exit(registry.Execute(os.Args[1:], cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Cwd: cwd, LookupEnv: os.LookupEnv, Environ: os.Environ()}))
}

func newRegistry() *cli.Registry {
	return cli.New(version, config.Commands(), record.Commands(), checks.Commands(), criteria.Commands(), decisions.Commands(), initialize.Commands(), doctor.Commands(), requirements.Commands(), plan.Commands(), implement.Commands(), fix.Commands(), judge.Commands(), pr.Commands(), sync.Commands(), refine.Commands(), concerns.Commands(), retro.Commands())
}
