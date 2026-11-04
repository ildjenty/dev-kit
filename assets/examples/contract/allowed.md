## Request

Check the input contract for approval state. The requirement is that an approved state always has an approver. Only the save function accepts external JSON, which is stored as received.

## Target

# Approval input

- Input: Validate the external JSON structure at the boundary, checking that status is draft or approved and approved_by is an optional string, then store it.
- Guarantee: A DB CHECK constraint rejects approved rows whose approved_by is empty or absent. The same constraint applies to all INSERT and UPDATE operations, with no bypassing storage path.
- Exemption: Under principle 3 of principles/data.md, waive the requirement to add the same rule to internal types solely for this property. The DB constraint guarantees it on every acceptance and storage path; outputs are built only from rows that passed the constraint.
- Failures: Return input errors separately from storage failures. Do not treat constraint violations as success.
