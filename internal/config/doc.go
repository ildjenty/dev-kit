// Package config parses and validates supplied TOML, applies repository setting defaults,
// and converts settings and command registrations to internal types.
// It leaves source selection to adapters and excludes model names and paths for launching session hosts.
// It depends on no other modules; judge and verification commands handle their own setup and cleanup.
// Canonical design: docs/architecture/modules.md#config
package config
