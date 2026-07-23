# GoSemanticInventory Changelog

## 1.0.0 — 2026-07-23

- Added digest-verified source authorization and explicit file outcomes.
- Added declaration reconciliation, lexical/package scopes, stable semantic
  IDs, receiver binding, local type relations, references, and imports.
- Added bounded evidence-derived interface satisfaction.
- Added typed artifact-store integration and detached immutable JSON output.
- Validated six pinned real repositories plus malformed and stale fixtures.
- Fixed blank-identifier and alias reconciliation defects found in validation.
- Applied reference limits before full binding to prevent runaway allocation.
- Reduced Kubernetes peak heap from 5.08–5.25 GiB to 3.88–3.92 GiB without
  changing artifact hashes or omission counts.
- Hardened public API, enum, configuration, provenance, and compatibility
  documentation.

Released under `go-semantic-inventory/v1.0.0`.
