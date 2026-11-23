package main

import (
	"fmt"
	"github.com/ildjenty/dev-kit/internal/cli"
	"os"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(cli.New("dev", Commands()).Execute(append([]string{"fixture"}, os.Args[1:]...), cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Cwd: cwd, LookupEnv: os.LookupEnv, Environ: os.Environ()}))
}
