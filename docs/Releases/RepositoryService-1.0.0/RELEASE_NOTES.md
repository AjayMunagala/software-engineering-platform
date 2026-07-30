# Repository Service 1.0.0 Proposed Release Notes

Repository Service `1.0.0` is the proposed first stable application-service
contract over the released repository intelligence, Go language intelligence,
persistence, PostgreSQL, and runtime foundations.

The service accepts an already-authorized opaque source handle, coordinates a
deterministic synchronous scan, publishes immutable exact-byte artifacts
atomically, and exposes scoped metadata and streamed artifact retrieval. It
does not fetch, execute, build, test, or mutate repositories.

Validation covers conformance, pinned real repositories, deterministic output,
cross-platform races, PostgreSQL recovery, coverage, fuzzing, performance,
scope/privacy boundaries, and release audits. See `KNOWN_LIMITATIONS.md` for
the open larger-host Kubernetes qualification.

These notes remain proposed until final engineering acceptance and version
promotion.
