## Request

The change affects only local daily report storage; there are no UI or external communication changes. The specification requires storing the input count unchanged and retrieving that count. Compare against the independent observation record.

## Target

# Storage observation record

- Environment: Started the CLI with a temporary directory as the storage location.
- Operation: Saved a daily report count of 0, read the stored file, and retrieved it through the CLI.
- Results: The input count was 0, the stored file count was 1, and the retrieved count was 1.
- Console errors: None.
- Evidence: Recorded the input, stored file, and retrieved value above.
- Cleanup: Stopped the CLI and deleted the temporary directory.
