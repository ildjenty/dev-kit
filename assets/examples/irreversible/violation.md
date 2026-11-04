## Request

Compare the design proposal for stored daily reports with the list of difficult-to-change decisions.

## Target

# Daily report retention

- Requirement: Delete submitted daily reports.
- Design: Permanently delete submitted daily reports from storage every night. No recovery copy exists.
- List of difficult-to-change decisions: None. Since retention can be changed later, publish deletion processing first.
