# Scan Execution Core 1.0.0

This package implements the synchronous scan coordinator introduced in Phase
4.0.4. Production intelligence and materialization behavior may satisfy its
narrow adapter interfaces without coupling the coordinator to any engine.

It provides:

- an explicit running/succeeded/failed/canceled state machine;
- one admission lease per newly created execution flight;
- keyed in-process single-flight with independent waiter cancellation;
- all-waiter and admission-drain cancellation;
- store-owned atomic begin, finalization, cancellation, and publication;
- durable orphan detection and publication ambiguity reconciliation;
- deterministic artifact metadata over adapter-prepared analysis inputs;
- scan/artifact get, list, cancel, and exact export orchestration;
- stable redacted errors and bounded detached cleanup.

The package contains no RIE/LIE imports, artifact codec or materializer,
Persistence Port adapter, PostgreSQL, SQL, pgx, runtime package, filesystem,
network, command, listener, authentication, UI, or AI implementation. Those
concerns remain behind its narrow interfaces.

The contract is frozen at `1.0.0`. The `1.x` line accepts compatible bug fixes
only and does not add methods to frozen interfaces.
