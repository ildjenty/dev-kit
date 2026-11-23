# Purpose of structure

Choose locations for new functions, types, files, folders, packages, and services by the purposes below. Use `structure/smells.md` for observations of failing purposes, `structure/deciding.md` for placement and dependency decisions and when to fix defects, and `structure/principles.md` for principles in specific situations. Treat existing structure, conventions, and documentation as subjects to verify.

## What structure preserves

As features grow and requirements change, preserve these five purposes. Judge structural quality only by whether they hold.

- **Contain failures and authority**: Failures, authority, and attacks do not spread beyond their boundaries.
- **Allow replacement and deletion**: A module of any size can be replaced as a whole while preserving its contract, or removed as a whole when no longer needed.
- **Keep changes in one place**: One change ends within one module. The number of locations touched does not grow disproportionately to the change.
- **Determine one placement**: The nature of a new item determines one location and way to write it. Fewer valid choices reduce variation in agent output.
- **Make purpose visible**: The structure reveals what each module is for without explanatory comments.

When the purposes conflict, prioritize those hardest to recover later: containing failures and authority, because leaked or destroyed information cannot be recovered; replacement and deletion; changes in one place; one placement; visible purpose.
