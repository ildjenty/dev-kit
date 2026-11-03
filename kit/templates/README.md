This directory contains templates for the fixed formats handled by form.
Template application and Markdown rendering belong to internal/form; HTML generation belongs to internal/view.
For placement, see `docs/architecture/modules.md#kit` from the repository root. For formats, see `docs/architecture/modules.md#form`.

The templates use `text/template` syntax. Pass the loaded text and the following input to `form.ApplyTemplate`.

- `skill.md.tmpl`: `form.SkillTemplate`. Composes a skill from its purpose, inputs, steps, outputs, and references.
- `aspect.md.tmpl`: `form.AspectTemplate`. Composes an aspect file from checks, blocking conditions, items not to report, and the output format.
- `pr.md.tmpl`: `form.PRTemplate`. Includes the change scope, plan summary, decisions, what was verified and how, unconfirmed items and risks, and references.
- `design.md.tmpl`: `form.DesignTemplate`. Includes the proposal, constraints, irreversible decisions, alternatives considered, and verification method.

If a repository has an existing PR template, pass its text to preserve its headings. For the design proposal's "Irreversible decisions" section, supply a reference to the plan's list and any required updates.

Render fixed-format values with `form.RenderMarkdown`. Questions explain that people may answer in their own words, give answers outside the options or conditional answers, ask questions in return, and revise their answers. When requesting screen design approval, the caller sets the question's `screen_id` and a tree approval target that includes the displayed content.
