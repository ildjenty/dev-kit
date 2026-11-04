`rules.md` supplies default rules and shared difficult-to-change decisions. `devkit init` copies it to `.devkit/rules.md`, which the repository maintains with each decision's enforcement mechanism and grounds. Keep task-specific irreversible decisions in the plan.

The rules are Markdown. Apply them to target code and documents; "Enforcement: Not implemented" identifies rules without an enforcing implementation. Principle and kit references resolve within the installed `.devkit/kit/<kit_version>/` copy. Host settings fragments are in that copy's `hosts/` directory.
