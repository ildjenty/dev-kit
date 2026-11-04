These Markdown templates use Go `text/template` syntax to format skills, aspects, PR bodies, and design proposals. The installed kit uses the `brief/` templates for work, judgment, and planning requests.

- `skill.md.tmpl`: Composes a skill from its purpose, inputs, steps, outputs, and references.
- `aspect.md.tmpl`: Composes an aspect file from checks, blocking conditions, items not to report, and the output format.
- `pr.md.tmpl`: Includes the change scope, plan summary, decisions, what was verified and how, unconfirmed items and risks, and references.
- `design.md.tmpl`: Includes the proposal, constraints, irreversible decisions, alternatives considered, and verification method.

If a repository has an existing PR template, preserve its headings. For the design proposal's "Irreversible decisions" section, supply a reference to the plan's list and any required updates.

Fixed JSON formats are defined in the installed kit copy's `schemas/` directory. Questions explain that people may answer in their own words, give answers outside the options or conditional answers, ask questions in return, and revise their answers. When requesting screen design approval, set the question's `screen_id` and a tree approval target that includes the displayed content.
