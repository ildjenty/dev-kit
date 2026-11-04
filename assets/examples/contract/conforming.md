## Request

Check the input contract for approval state. The requirement is that an approved state always has an approver. Only the save function accepts external JSON, which is stored as received.

## Target

# Approval input

- Input: status is draft or approved; approved_by is an optional string.
- Acceptance: Validate status and approved_by at the save function boundary. If status is approved and approved_by is missing or empty, reject as an input error without storing.
- Guarantee: This function is the only storage update path, so approved cannot be stored without an approver. Distinguish draft from approved with an approver internally.
- Failures: Return input errors separately from storage failures.
