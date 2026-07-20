# Changelog

## GoLanguageInventory 1.0.0 — 2026-07-20

### Added

- Artifact-driven LIE runner with deterministic engine registration.
- Go package and file inventory.
- Import, struct, interface, function, method, constant, and variable discovery.
- Exact byte-offset, line, and byte-column evidence.
- SHA-256 digests over parsed bytes.
- Bounded diagnostics, cancellation, and eight-worker concurrency.
- Deeply immutable artifact collections and stable deterministic IDs.
- Repeatable 10-, 1,000-, and 10,000-file benchmarks.

### Fixed during validation

- Parenthesized method receiver extraction, including `*(T)` and generic forms.

### Security

- Repository-root containment through rooted filesystem access.
- Parent and file symlink escape detection.
- Bounded complete reads with no repository execution or network access.

### Compatibility

This release freezes the artifact contract at `1.0.0`. The `1.0.x` line is
restricted to compatible defect fixes.
