package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ildjenty/dev-kit/internal/kitinfo"
)

type Env struct {
	Registry  *Registry
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Cwd       string
	LookupEnv func(string) (string, bool)
	Environ   []string
}

func (e Env) Getenv(key string) string {
	if e.LookupEnv != nil {
		value, ok := e.LookupEnv(key)
		if !ok {
			return ""
		}
		return value
	}
	return os.Getenv(key)
}

func (e Env) Path(path string) string {
	if path == "" {
		return e.Cwd
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return e.Cwd + "/" + path
}

func (e Env) Require(keys ...string) error {
	for _, key := range keys {
		if e.Getenv(key) == "" {
			return fmt.Errorf("%s must be set", key)
		}
	}
	return nil
}

func (e Env) JSON(value any) error { return json.NewEncoder(e.Stdout).Encode(value) }

type Command struct {
	Area  string
	Name  string
	Usage string
	Run   func(args []string, env Env) error
}

type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }
func Usage(usage string) error     { return &ExitError{2, "Usage: " + usage} }

type Registry struct {
	commands []Command
	version  string
}

func New(version string, groups ...[]Command) *Registry {
	r := &Registry{version: version}
	for _, commands := range groups {
		r.commands = append(r.commands, commands...)
	}
	return r
}

func (r *Registry) Commands() []Command { return append([]Command(nil), r.commands...) }

func (r *Registry) Version() string {
	if r == nil {
		return "dev"
	}
	return r.version
}

func (r *Registry) MissingCommands(calls []string) []string {
	commands, areas := map[string]bool{"help": true, "version": true}, map[string]bool{}
	if r != nil {
		for _, command := range r.commands {
			if command.Name == "" {
				areas[command.Area] = true
			} else {
				commands[command.Area+" "+command.Name] = true
			}
		}
		for _, command := range r.commands {
			if command.Name != "" {
				delete(areas, command.Area)
			}
		}
	}
	missing := []string{}
	for _, call := range calls {
		area, _, _ := strings.Cut(call, " ")
		if !areas[area] && !commands[call] {
			missing = append(missing, "devkit "+call)
		}
	}
	sort.Strings(missing)
	return missing
}

func (r *Registry) Run(args []string, env Env) error {
	env.Registry = r
	if len(args) == 0 || args[0] == "help" {
		lines := []string{"devkit help", "devkit version"}
		for _, command := range r.commands {
			lines = append(lines, command.Usage)
		}
		sort.Strings(lines)
		_, err := fmt.Fprintln(env.Stdout, strings.Join(lines, "\n"))
		return err
	}
	if args[0] == "version" {
		if len(args) != 1 {
			return Usage("devkit version")
		}
		_, err := fmt.Fprintln(env.Stdout, r.version, kitinfo.EmbeddedDigest())
		return err
	}
	for _, command := range r.commands {
		if command.Area != args[0] {
			continue
		}
		if command.Name == "" {
			return command.Run(args[1:], env)
		}
		if len(args) > 1 && command.Name == args[1] {
			return command.Run(args[2:], env)
		}
	}
	return Usage("devkit <area> <subcommand> [args]; devkit help lists commands")
}

func (r *Registry) Execute(args []string, env Env) int {
	err := r.Run(args, env)
	if err == nil {
		return 0
	}
	if err.Error() != "" {
		fmt.Fprintln(env.Stderr, err)
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	return 1
}
