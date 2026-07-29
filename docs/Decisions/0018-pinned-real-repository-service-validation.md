# ADR 0018 — Pinned Real-Repository Service Validation

## Status

Accepted

## Date

2026-07-29

## Context

Phase 4.0.6 connects the Repository Service candidate to released intelligence,
persistence, and runtime components. Unit, conformance, fuzz, race, benchmark,
and disposable PostgreSQL tests pass, but release stabilization requires proof
on representative real repositories through the complete service boundary.

Unpinned repositories, network-enabled analysis, mutable checkouts, ad hoc
normalization, or reused large-process heaps would make evidence difficult to
reproduce and could conceal privacy, determinism, or resource defects.

## Decision

1. Validate only exact, design-reviewed repository revisions and controlled
   fixture tree digests.
2. Reuse the accepted RIE and Go LIE corpus where possible so upstream input is
   stable and service integration is the variable under test.
3. Allow network access only for operator-owned preflight fetching. The
   Repository Service never clones, fetches, downloads dependencies, or runs
   repository commands.
4. Require a clean tree and canonical path-free source manifest before every
   measured run.
5. Use disposable migrated PostgreSQL databases and production composition;
   do not add a validation-only product execution path.
6. Compare exact artifact bytes, digests, dependencies, diagnostics,
   statistics, and omission counts. Validate scan-bound public/physical IDs
   independently and exclude them only between passes with distinct Scan IDs,
   along with explicitly listed timestamps and operational measurements.
7. Require repeated clean-process, one/eight-worker, Windows, and Ubuntu
   evidence when canonical source manifests match.
8. Run Kubernetes-scale passes in separate clean processes with bounded
   workers and resource monitoring.
9. Treat malformed source, stale source, cancellation, scope isolation,
   pre-publication failure, and response-lost publication as mandatory
   controlled evidence.
10. Keep Phase 4.0.7 validation-owned. It may add a harness and compatible
    defect fixes, but no new service, engine, persistence, runtime, transport,
    authentication, UI, or AI capability.
11. Require a committed validation report, machine-readable results, defect
    history, and explicit engineering acceptance before Phase 4.0.8.
12. Require an interrupted PostgreSQL crash-recovery proof after atomic
    publication, exact Git-version evidence during preflight, and explicit
    Kubernetes outcome classification.

## Rationale

Pinned inputs and exact-byte comparison make nondeterminism observable.
Production composition exercises the actual service boundary. Separating
operator fetch from analysis preserves the no-network/no-execution contract.
Clean processes prevent retained heap from distorting large-repository
comparisons on the reference workstation.

## Alternatives

### Validate repository default branches

Rejected. Moving inputs make failures and performance changes irreproducible.

### Let the service clone repositories

Rejected. Remote source acquisition is outside the accepted Repository Service
contract and would introduce credentials, network, and mutation policy.

### Compare only summaries

Rejected. Summary equality can miss byte, dependency, ordering, payload, and
publication defects.

### Run only Kubernetes

Rejected. Small and medium cases isolate failures faster and exercise both the
seven- and ten-artifact branches before the most resource-intensive case.

### Reuse one process for all large comparisons

Rejected. Go heap retention and operating-system cache effects would make peak
memory and repeated-run results misleading on the reference workstation.

## Consequences

- Repository revisions and controlled fixture digests become part of the
  Phase 4.0.7 evidence contract.
- Validation takes longer and requires disposable PostgreSQL plus both Windows
  and Ubuntu environments.
- Checkout/fetch time is excluded from service performance.
- Source equivalence is proven before cross-platform exact-byte comparison.
- Any contract change discovered during validation returns to a separate
  design gate instead of being hidden in the harness.

## Security consequences

- no production/shared credentials or databases are used;
- no source handle, local path, credential, SQL, or payload enters logs or
  metric labels;
- outbound network and command execution are audited during analysis;
- cross-scope operations are mandatory negative tests;
- validation repositories remain outside the project working tree.

## Acceptance gate

Engineering accepted this ADR with recommendations on 2026-07-29 after review
with the architecture, fixture manifest, and validation plan. Acceptance
authorizes Phase 4.0.7 validation execution only. Phase 4.0.8 remains
unauthorized.
