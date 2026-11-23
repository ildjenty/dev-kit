# Executed workflow fixtures

`fixtures/<workflow>/` holds source-only Archon fixtures that execute the Go commands and stub the AI nodes. They are neither embedded in `devkit` nor installed by `devkit init`.

`scripts/test-kit.sh` first validates a fresh installation and runs its shipped stub fixtures with the normal binary. It then copies these fixtures into the temporary repository, builds `cmd/devkit-fixture` and uses its before/after helpers around the normal commands to exercise each workflow's main path.
