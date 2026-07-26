# Persistence Subsystem Changelog

## 1.0.0 — 2026-07-26

First stable release of the storage-neutral Persistence Port, reusable
conformance harness, and PostgreSQL reference adapter.

### Architecture

- Established seven narrow capability interfaces plus the composed `Port`.
- Kept transactions inside lifecycle operations; no public transaction API.
- Kept engines, immutable artifact types, codecs, SQL, drivers, credentials,
  connection pools, and migrations outside the neutral contract.
- Made exact serialized artifact bytes authoritative and projections
  rebuildable.
- Required repository scope isolation on all 18 public operations.
- Adopted pgx v5 behind the internal PostgreSQL adapter boundary.

### Stable Guarantees

- Streaming payload staging and export.
- Exact size and SHA-256 verification.
- Fixed ordered 4 MiB PostgreSQL chunks.
- 4 GiB operational payload maximum and independent 8 GiB schema ceiling.
- Atomic complete-scan publication.
- Immutable request, record, page, and receipt values.
- Stable storage-neutral error categories without SQL leakage.
- Idempotent retry behavior and deterministic publication manifests.
- Retention marking, bounded purge, and delayed garbage collection.

### Breaking Changes from 0.1.0 Candidates

These changes occurred before the stable freeze and require candidate adapter
implementers to update:

- Replaced repository canonical text with proof-bearing `SourceIdentity`.
- Replaced scan producer metadata with `AnalysisProfileDigest` and optional
  source revision.
- Removed `PublicationID`; publication identity is scan-keyed and includes a
  manifest scheme.
- Changed artifact stable-ID evidence to one optional scheme string matching
  the physical envelope.
- Removed arbitrary artifact attributes; bounded projections are the supported
  query extension point.
- Capped configurable runtime payload size at 4 GiB rather than allowing the
  8 GiB physical ceiling.
- Removed the unnecessary `EqualJSON` helper and unused
  `ErrContextRequired` sentinel.

### Migration Notes for Adapter Implementers

1. Implement each capability interface or the composed `persistence.Port`.
2. Run `persistence/conformance` before adapter-specific integration tests.
3. Enforce scope on writes, target reads, lists, verification, publication,
   and retention. Cross-scope target reads return `not_found`.
4. Consume the complete stage stream even for idempotent retries and verify
   declared size and digest.
5. Publish a complete scan atomically; staged payloads remain invisible.
6. Map backend errors to neutral `ErrorKind` values without exposing driver
   messages, constraint names, or connection details.
7. Do not add methods to existing v1 interfaces. Introduce optional future
   capabilities as new standalone interfaces.
8. Treat neutral Go values as an API contract, not as a JSON wire format.

No database migration is required from an earlier production persistence
release because `1.0.0` is the first stable release.
