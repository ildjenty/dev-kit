package status

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
)

func watch(args []string, env cli.Env) error {
	if len(args) == 0 {
		return cli.Usage("devkit watch <run-id> [<run-id> ...]")
	}
	previous := ""
	for {
		current := ""
		ended := 0
		for _, run := range args {
			cmd := exec.Command("archon", "workflow", "get", run)
			cmd.Dir, cmd.Env = env.Cwd, env.Environ
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("run %s: archon workflow get failed: %w\n%s", run, err, strings.TrimSpace(stderr.String()))
			}
			status := ""
			for _, line := range strings.Split(string(out), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[0] == "Status:" {
					status = fields[1]
					break
				}
			}
			if status == "" {
				return fmt.Errorf("run %s: archon workflow get returned no Status field\n%s", run, strings.TrimSpace(stderr.String()))
			}
			current += " " + run + "=" + status
			switch status {
			case "completed", "failed", "cancelled", "abandoned":
				ended++
			}
		}
		if current != previous {
			if _, err := fmt.Fprintln(env.Stdout, "status:"+current); err != nil {
				return err
			}
			previous = current
		}
		if ended == len(args) {
			return nil
		}
		time.Sleep(30 * time.Second)
	}
}
