## Request

Compare the design proposal for stored daily reports with the list of difficult-to-change decisions.

## Target

# Daily report retention

- Requirement: Delete submitted daily reports.
- Difficult-to-change decision: Permanently deleting stored daily reports. Extending retention cannot recover already deleted reports.
- Decision material: Investigated reading the storage location and recovery methods, confirming that no copy exists and reports cannot be regenerated.
- Decision: The requester finalized retention at 30 days. Implement and publish deletion processing after finalization.
