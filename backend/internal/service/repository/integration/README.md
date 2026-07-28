# Repository Service Persistence & Runtime Integration

This package is the sole Phase 4.0.6 application boundary between the neutral
Repository Service coordinators and the frozen Persistence/Runtime contracts.

It owns UUID and model translation, source-proof consistency, runtime work
admission, exact streaming payload staging, deterministic physical artifact
UUIDs, canonical manifest construction, atomic publication, exact reads, and
safe error translation.

It does not own database pools, TLS, credentials, migrations, runtime shutdown,
transports, engine behavior, repository cloning, projections, or background
work. The normative identity and manifest vectors are in
`docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`.

