# Security principles

## 1. Validate input at the trust boundary and keep data out of code

- **Applicability**: When handling input that crosses a trust boundary (requests, files, environment, messages, or responses from external services), or building queries, commands, paths, markup, or templates from it.
- **Exemptions**: None.
- **Principle**: Decide where each input crosses from an untrusted to a trusted context and validate it there against an allowlist of type, length, range, format, and encoding before use. Reject input that fails validation instead of repairing it silently. Pass untrusted values to interpreters only through mechanisms that keep data separate from code: parameterized queries, argument arrays instead of shell strings, path resolution confined to an allowed root, and context-aware output encoding. Treat data that was stored after validation as untrusted again when the context that interprets it differs. Do not rely on client-side checks for guarantees the server must enforce.
- **Violations and signs**:
  - Concatenating input into SQL, shell commands, LDAP or XPath queries, regular expressions, or templates.
  - Joining user-supplied names into file paths without confining the result to an allowed directory.
  - Rendering stored or external text into HTML without encoding for the output context.
  - Validating with a denylist of known bad characters.
  - Deserializing untrusted data into types that can execute code or allocate without bound.
  - Accepting input only because the client already validated it.
- **Before and after**: `exec("convert " + name + " out.png")` → `exec(["convert", "--", resolveUnder(uploadDir, name), "out.png"])`, rejecting names that resolve outside `uploadDir`.
- **blocking**: Untrusted input reaches an interpreter, file path, or deserializer without validation and separation from code.
- **Evidence**: For each input reaching an interpreter, path, or deserializer, the trust boundary where it is validated, the validation rule, the separation mechanism used, and tests with malicious inputs.

## 2. Keep secrets out of code, history, and output

- **Applicability**: When handling credentials, tokens, keys, session identifiers, or personal or confidential data.
- **Exemptions**: None.
- **Principle**: Load secrets at runtime from the environment or a secret store; do not write them in code, configuration committed to the repository, test fixtures, or command lines visible to other processes. Do not write secrets or sensitive personal data to logs, error messages, metrics, URLs, or responses; record identifiers or redacted forms needed for investigation instead. Store passwords only as hashes from a slow, salted algorithm intended for passwords. Encrypt sensitive data in transit and, where the contract requires, at rest, using maintained libraries rather than custom cryptography. When a secret has been exposed, treat removal from the working tree as insufficient and report that rotation is needed.
- **Violations and signs**:
  - API keys, passwords, or private keys in source files, committed `.env` files, or test data.
  - Logging full request headers, bodies, or tokens.
  - Putting tokens in query strings.
  - Hashing passwords with a fast general-purpose hash or without a salt.
  - Implementing encryption, token signing, or random token generation without a maintained library and a cryptographically secure source.
- **Before and after**: `log.info("login", { user, password })` → `log.info("login", { userId: user.id })`.
- **blocking**: A secret is committed, logged, or returned; or passwords are stored reversibly or with a fast hash.
- **Evidence**: Where each secret is loaded from, the fields redacted from logs and errors, and the library and parameters used for hashing and encryption.

## 3. Authenticate and authorize every operation on the server side

- **Applicability**: When adding or changing operations that read or change data, or that act on behalf of a user or service.
- **Exemptions**: None.
- **Principle**: Establish who is calling before any operation that requires identity, and check on the server for every request that the caller may perform this operation on this specific resource. Derive the resource owner from trusted records, not from identifiers supplied in the request. Deny when the decision cannot be made (fail-closed, principle 1 of `principles/engineering.md`). Put the check where every path to the operation passes through it (principle 4 of `principles/structure.md`). Expire and invalidate sessions and tokens on logout, password change, and privilege change.
- **Violations and signs**:
  - Fetching a record by an ID from the request without checking that the caller owns or may access it.
  - Checking permissions only in the UI or only on some of the routes to the same operation.
  - Trusting a role or user ID sent by the client.
  - Allowing access when the permission service fails or returns an unknown result.
  - Sessions that remain valid after logout or password change.
- **Before and after**: `return db.orders.get(req.params.id)` → `const order = db.orders.get(req.params.id); if (order?.ownerId !== session.userId) throw new Forbidden(); return order`.
- **blocking**: An operation can be performed on a resource without a server-side check that the authenticated caller may do so.
- **Evidence**: For each changed operation, where identity is established, where authorization is checked against which resource, the behavior when the decision fails, and tests for access by another user.

## 4. Grant the least privilege and choose safe defaults

- **Applicability**: When configuring permissions, credentials, network exposure, file modes, feature flags, or default settings.
- **Exemptions**: None.
- **Principle**: Give each process, service account, token, and user only the permissions its function requires, with the narrowest scope and shortest lifetime that work. Make the default configuration the safe one: features that widen exposure are off until enabled, listeners bind to the narrowest interface needed, debug endpoints and verbose errors are disabled outside development, and TLS verification stays on. When a safe default must be weakened, require an explicit setting and state its effect.
- **Violations and signs**:
  - Using an administrator or owner credential for a process that only reads.
  - Wildcard permissions, CORS origins, or file modes such as `0777`.
  - Disabling TLS certificate verification to make a connection work.
  - Debug mode, stack traces, or admin endpoints enabled by default.
  - Binding a local-only service to all interfaces.
- **Before and after**: A deploy token with organization-wide write access → a token limited to the one repository and the push permission, expiring after the job.
- **blocking**: A change grants permissions or exposure beyond what its function requires, or turns a safe default unsafe without an explicit setting.
- **Evidence**: The permissions and exposure each component needs and why, and the defaults with what enabling each wider option changes.

## 5. Add dependencies deliberately and pin what is executed

- **Applicability**: When adding, updating, or loading dependencies, build plugins, actions, container images, or scripts from outside the repository.
- **Exemptions**: None.
- **Principle**: Before adding a dependency, check that it is maintained, widely used or reviewed, and necessary compared with the code it replaces. Pin versions through a lock file or digest so that builds execute what was reviewed, and update them through reviewed changes. Do not execute scripts fetched at build or run time without a pinned version and integrity check. Check known vulnerabilities for added and updated versions with the repository's scanner when one exists. Verify package names against the intended source to avoid typosquatting and dependency confusion.
- **Violations and signs**:
  - Unpinned version ranges or `latest` tags for executed code without a lock file.
  - `curl ... | sh` in builds or setup.
  - CI actions or images referenced by mutable tags.
  - Adding a package for a few lines of code or without checking its maintainer and source.
  - Lock file changes unrelated to the stated dependency change.
- **Before and after**: `uses: some/action@main` → `uses: some/action@<commit SHA>  # v2.3.1`.
- **blocking**: Executed external code is unpinned or fetched without integrity verification, or an added dependency has a known exploitable vulnerability in the used functionality.
- **Evidence**: For each added or updated dependency, its purpose, source, pinned version, and vulnerability check result.

## 6. Record security-relevant events without recording sensitive data

- **Applicability**: When adding or changing authentication, authorization, administrative operations, or changes to permissions and secrets.
- **Exemptions**: None.
- **Principle**: Record who did what to which resource, when, and with what result for authentication attempts, authorization denials, administrative actions, and permission or secret changes, so that misuse can be detected and investigated. Keep these records free of secrets and sensitive data (principle 2). Protect them from modification by the actors they record. Neutralize control characters in logged input so that entries cannot be forged.
- **Violations and signs**:
  - Failed logins or authorization denials leave no record.
  - Administrative changes without the acting identity.
  - Logging raw user input that can insert newlines or fake entries.
- **Before and after**: Permission change with no record → a structured entry with actor ID, target ID, old and new role, time, and result.
- **blocking**: None (record as reference information).
- **Evidence**: The events recorded for the changed operations and the fields included and excluded.
