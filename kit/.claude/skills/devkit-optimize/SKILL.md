---
name: devkit-optimize
description: Fit an installed dev-kit to repository checks, conventions, layout and owner-requested workflow settings, and triage existing open Issues. Starts the reviewed optimize workflow after init or when the repository or the owner's workflow preferences change; relays questions and exact-version approval, then reports the PR.
---

# Fit dev-kit through optimize

Invoke `devkit-runs` for starting, monitoring and resuming runs and relaying results.
Take the owner's instructions verbatim, including an empty value when none were supplied.
Write them to a file outside the checkout and start `optimize` on an `optimize/<name>` branch
with the same file-input pattern used by devkit-runs:

```sh
archon workflow run optimize --branch optimize/<name> --detach   --input "instructions=$(cat <instructions file>)"
```

The writer drafts the repository fit and issue triage; machine checks and a fresh read-only judge
review up to three rounds. Relay recorded questions verbatim. For changes, pass the owner's answer
through an answers file. Only after the owner approves the exact reviewed version, restart optimize
on the same branch with `approve=<version>`. Never synthesize approval.
The machine publishes the branch as a PR and applies the approved Issue triage. Relay the PR URL
and issue_numbers. The owner reviews and merges the PR. If publication fails, use devkit-runs'
same-version retry procedure; do not apply or recreate the triage in the session.
