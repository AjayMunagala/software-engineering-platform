# RIE v0.7 — Repository Intelligence Summary

The summary is the immutable entry point for future intelligence consumers. It composes `RepositoryMetadata 1.0.0` and publishes known-section and unavailable-capability states without rescanning or copying underlying inventory data.

## Honesty policy

Controllers, services, tests, coverage, and complete diagnostic counts are explicitly unavailable in v0.7. They are never reported as zero and never inferred from folder names.

## JSON behavior

The report's `summary` section is a compact index of artifact references and availability. Repository facts remain in the existing `metadata` section and are not serialized twice.

## Package standard

- `interface.go` — engine and artifact retrieval contracts
- `implementation.go` — artifact composition and availability projection
- `config.go` — unavailable capability definitions
- `model.go` — immutable summary artifact
- `errors.go` — stable prerequisite and configuration errors
- `implementation_test.go` — composition, honesty, and immutability tests
- `implementation_benchmark_test.go` — constant-work benchmark
