# Runtime Infrastructure Changelog

## 1.0.0 — 2026-07-27

### Released

- Promoted Runtime Configuration, PostgreSQL Runtime, Runtime Health,
  Application Runtime Lifecycle, and Runtime Observability to stable `1.0.0`.
- Froze the `1.x` compatibility policy after Phase 3.5.4 engineering
  acceptance.
- Added the concise operator release checklist and final namespaced tag plan.

### Compatibility

- Runtime Infrastructure `1.0.0` consumes Persistence Port `1.0.0`, PostgreSQL
  Adapter `1.0.0`, and persistence schema contract `1.0.0`.
- No persistence, schema, migration, or intelligence-engine contract changed.

## 1.0.0-rc.1 — 2026-07-26

### Added

- Strict immutable runtime configuration with profile-aware validation.
- External secret resolution and safe redacted configuration views.
- Runtime-owned PostgreSQL TLS and capability pool lifecycle.
- Migration-maintained schema compatibility and least-privilege proof.
- Application lifecycle, health, work admission, drain, cancellation, and
  idempotent shutdown.
- Transport-neutral structured logging and bounded runtime metrics.
- Windows, Ubuntu, race, disposable PostgreSQL, TLS, lifecycle-cycle, coverage,
  benchmark, and security validation harnesses.

### Candidate API changes

- Candidate contracts from Phases 3.5.1–3.5.3 are consolidated as
  `1.0.0-rc.1`.
- PostgreSQL runtime adds a detached operational-statistics view for the
  runtime observability source; pool and driver objects remain private.
- Application runtime adds an optional observability factory while preserving
  the original isolated `NewStarter` constructor.

### Compatibility

- No Persistence Port `1.0.0` API change.
- No PostgreSQL Adapter `1.0.0` API change.
- No persistence schema contract change.
- No migration history rewrite.
