# Executed workflow fixtures

`fixtures/<workflow>/` holds source-only Archon fixtures that execute the Go commands and stub the AI nodes. They are neither embedded in `devkit` nor installed by `devkit init`.

`scripts/test-kit.sh` first validates a fresh installation and runs its shipped stub fixtures with the normal binary. It then copies these fixtures into the temporary repository, builds `cmd/devkit-fixture` and uses its before/after helpers around the normal commands to exercise each workflow's main path.

Refine's main path includes a verbatim owner decision committed as agreed before the writer, with only AI additions in the judge's review targets. Its no-AI fixture uses `owner_decisions` and explicit `ai=none`, publishes a decision PR and asserts that writer/judge/approval artifacts are absent. It supplies no AI stubs, so reaching a writer or judge fails the fixture. Go tests cover exact text preservation, supersession, approval and publication retries without duplicate owner records, and strict input rejection.
