# Repository Lifecycle 1.0.0

This package implements only Phase 4.0.3 repository registration, lookup,
listing, and archival.

It coordinates:

- opaque source-handle resolution into path-free SHA-256 proof;
- bounded resolver cleanup before persistence;
- deterministic mutation fingerprints;
- store-owned atomic idempotency;
- already-authorized repository scope isolation;
- detached repository views and pagination;
- stable redacted dependency errors.

The `Store` interface owns atomic repository persistence behavior. No database,
SQL, pgx, Persistence Port adapter, pool, engine, runtime, filesystem, network,
command, transport, authentication, UI, scan execution, or AI implementation
appears here.

The contract is frozen at `1.0.0`. The `1.x` line accepts compatible bug fixes
only and does not add methods to frozen interfaces.
