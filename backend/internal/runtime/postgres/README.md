# PostgreSQL Runtime

## Status

- Phase: 3.5.2
- Contract: candidate `0.1.0`
- Local implementation gate: complete
- Engineering acceptance: accepted on 2026-07-26
- Phase 3.5.3 integration: accepted on 2026-07-26

## Purpose

This package turns an accepted, secret-free runtime configuration into a
proved set of PostgreSQL resources. It owns TLS material loading,
capability-specific `pgxpool` instances, startup connectivity and compatibility
proofs, PostgreSQL Adapter construction, and reverse-order resource cleanup.

It does not own migrations, runtime configuration loading, health,
observability, application startup orchestration, APIs, UI, or business logic.

## Resource Model

Local and CI profiles use one combined pool. Staging and production use three
independent pools:

```text
Runtime
├── ingest pool    -> platform_ingestor
├── read pool      -> platform_artifact_reader
└── retention pool -> platform_retention_worker
```

The runtime passes each pool to the frozen PostgreSQL Adapter `1.0.0` through
its existing minimal database capability. Callers receive only the narrow
neutral persistence interfaces; they never receive a pool, SQL interface,
credential, or PostgreSQL driver type.

## Open Contract

`NewFactory().Open(ctx, loadedConfiguration)` performs these bounded steps:

1. load and cryptographically validate bounded TLS material;
2. resolve only the required password reference;
3. construct the profile-approved pool set with `MinConns = 0`;
4. enforce the TLS transport policy immediately after negotiation;
5. acquire and ping each pool within its configured timeout;
6. verify PostgreSQL 18 and deterministic session settings;
7. read the migration-maintained compatibility proof;
8. verify the capability's positive and negative privileges;
9. construct the frozen PostgreSQL Adapter;
10. return a runtime only after every required capability succeeds.

Any failure closes already-created pools in reverse construction order. Close
is idempotent. The runtime never runs or repairs migrations.

## Integrity and Security

- Production uses verify-full TLS with TLS 1.2 or newer.
- Custom CA and client-certificate material is limited to 1 MiB per file.
- Expired CA and client certificates are rejected.
- Encrypted client keys are not silently accepted without an approved
  passphrase design.
- Driver errors, addresses, identities, SQL, certificate paths, and secrets do
  not appear in public errors.
- The compatibility record is read-only to runtime roles.
- Runtime roles cannot access Atlas migration history or create schema objects.
- A TLS connection is proved at the client transport boundary; it does not
  depend on statistics-view privileges after `SET ROLE`.

## Compatibility Proof

Migration `202607260008_create_runtime_compatibility.sql` publishes exactly one
row with:

- contract key `aegis-postgresql-persistence`;
- schema contract `1.0.0`;
- supported adapter major range containing `1`;
- migration revision `202607260008`;
- a non-future publication timestamp.

All capability pools must independently read the same proof.

## Validation

Unit tests cover combined and separate pools, immutable routing, TLS material,
session/schema/privilege rejection, cancellation, redacted errors, idempotent
close, 1,000 open/close cycles, and the required partial-startup failure:

```text
ingest succeeds -> read succeeds -> retention fails
                                      ↓
                            close read -> close ingest
```

The disposable integration harness creates PostgreSQL 18 under `/tmp`, applies
all checksum-verified migrations, generates one-use TLS certificates, and
validates both combined disabled-TLS and separate verify-full-TLS runtimes.

Run package tests:

```powershell
cd D:\Project_Ai\backend
go test ./internal/runtime/postgres
go test -race ./internal/runtime/postgres
```

Run disposable PostgreSQL integration:

```powershell
wsl.exe -d Ubuntu-24.04 -u postgres -- bash -lc `
  'cd /mnt/d/Project_Ai/backend/internal/runtime/postgres/tests && bash validate.sh'
```

## Future Work

Phase 3.5.3 consumes this package only through its public capabilities to
implement lifecycle coordination, liveness, readiness, admission, drain, and
graceful shutdown. Pool internals remain private.
