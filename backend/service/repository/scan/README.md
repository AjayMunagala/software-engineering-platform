# Scan Execution Core 0.1.0

This package implements only Phase 4.0.4 synchronous scan coordination.

It provides:

- an explicit running/succeeded/failed/canceled state machine;
- one admission lease per newly created execution flight;
- keyed in-process single-flight with independent waiter cancellation;
- all-waiter and admission-drain cancellation;
- store-owned atomic begin, finalization, cancellation, and publication;
- durable orphan detection and publication ambiguity reconciliation;
- deterministic artifact metadata over fake, already-prepared analysis inputs;
- scan/artifact get, list, cancel, and exact export orchestration;
- stable redacted errors and bounded detached cleanup.

The package contains no real RIE/LIE execution, artifact materializer,
Persistence Port adapter, PostgreSQL, SQL, pgx, runtime package, filesystem,
network, command, listener, authentication, UI, or AI implementation.

The contract remains `0.1.0` until later adapter, integration, validation, and
stabilization milestones are accepted.
