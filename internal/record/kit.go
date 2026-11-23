package record

import (
	"fmt"
	"os"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
)

func KitStartReason(env cli.Env, workflow string) (string, error) {
	folder := env.Getenv("PACK_DIR")
	if folder == "" {
		folder = env.Path(kitinfo.Workflows)
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			return "", nil
		}
	}
	calls, err := kitinfo.Invocations(os.DirFS(env.Path(folder)), workflow, false)
	if err != nil {
		return "", err
	}
	missing := env.Registry.MissingCommands(calls)
	if len(missing) == 0 {
		return "", nil
	}
	installed := "the devkit that installed these workflows"
	stamp, err := kitinfo.ReadStamp(env.Path(folder))
	if err == nil {
		installed = fmt.Sprintf("devkit %s, which installed these workflows", stamp.DevkitVersion)
	}
	return fmt.Sprintf("missing command: %s; the installed workflows and this devkit binary come from different versions; run `devkit init` in the repository with the matching devkit, or install %s", strings.Join(missing, ", "), installed), nil
}
