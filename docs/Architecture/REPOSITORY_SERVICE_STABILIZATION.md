# Repository Service Stabilization Architecture

## Status

- Phase: 4.0.8 design
- Status: Proposed for engineering review
- Production stabilization: unauthorized pending design acceptance
- Version promotion and release tags: unauthorized
- Date: 2026-07-30

## Responsibility

Phase 4.0.8 stabilizes, audits, validates, and packages the existing Repository
Service candidate for a possible `1.0.0` release without adding service
behavior.

## Inputs

The phase accepts only the implementation and evidence already produced by
Phases 4.0.0 through 4.0.7. Its principal inputs are:

- the neutral Repository Service `0.1.0` candidate;
- repository lifecycle and synchronous scan orchestration;
- released RIE and Go LIE adapters and canonical codecs;
- accepted Persistence Port, PostgreSQL Adapter, and Runtime Infrastructure;
- the accepted Phase 4.0.7 real-repository evidence;
- the open larger-host Kubernetes release qualification.

## In scope

1. Review every exported Repository Service type, constant, constructor,
   method, validation rule, error kind, and observable ordering rule.
2. Propose the exact public contract to freeze as `1.0.0`.
3. Freeze golden-vector contracts for public artifact identity, physical
   artifact identity, manifest identity, profile identity, and canonical
   codecs.
4. Audit immutability, scope isolation, idempotency, cancellation,
   single-flight, atomic publication, reconciliation, and exact-byte export.
5. Profile CPU and memory without changing results or adding capabilities.
6. Run conformance, regression, race, fuzz, integration, determinism,
   security, dependency, and documentation gates.
7. Prepare release notes, changelog, compatibility policy, known limitations,
   qualification register, and operator checklist.
8. Propose annotated namespaced tags only after final engineering acceptance.

## Out of scope

- new Repository Service operations or behaviors;
- asynchronous scans, queues, schedulers, workers, or leases;
- REST, gRPC, GraphQL, HTTP health endpoints, or other transports;
- authentication, authorization, UI, IDE integration, or AI orchestration;
- changes to RIE, Go LIE, Persistence Port, PostgreSQL Adapter, or Runtime
  Infrastructure contracts;
- repository cloning, fetching, command execution, builds, tests, or mutation;
- redesign solely to fit the complete Kubernetes matrix into the reference
  8-GB-class validation host.

## Proposed freeze set

The following existing contracts are candidates for the `1.0.0` freeze:

- `RepositoryLifecycleService`, `ScanExecutionService`,
  `ArtifactQueryService`, and the convenience `Service` composition;
- immutable request, response, page, result, profile, repository, scan, and
  artifact values;
- the existing stable Repository Service error kinds and retry semantics;
- lowercase canonical UUID requirements for scope, repository, and scan IDs;
- deterministic cursor, pagination, artifact, and dependency ordering;
- synchronous execution, keyed single-flight, waiter cancellation, terminal
  scan states, idempotency, and publication reconciliation semantics;
- `repository-service-artifact-id/v1`;
- `repository-service-storage-artifact-id/v1`;
- `repository-service-manifest/v1`;
- `repository-service-profile/v1`;
- `repository-go/v1` and its exact profile digest;
- `canonical-json/1.0.0` and the released artifact codec mappings.

The implementation remains `0.1.0` until engineering accepts this design and
later accepts the stabilization evidence. This document does not promote any
version.

## Compatibility policy

For a future `1.x` line:

- existing interface methods, signatures, exported constants, validation
  meanings, and error-kind meanings do not change incompatibly;
- new optional capabilities require new narrow interfaces rather than methods
  added to frozen interfaces;
- new analysis behavior requires a new profile version and digest;
- canonical preimage or ID changes require a new scheme version;
- codec changes require a new codec version;
- service, artifact, codec, stable-ID, persistence schema, adapter, and runtime
  versions evolve independently;
- bug fixes may tighten conformance only when they restore already documented
  behavior and do not reinterpret successful historical results.

The neutral Go models do not establish an HTTP or JSON wire API. Future
transports translate the frozen service semantics through separately versioned
transport contracts.

## Stabilization workflow

```text
Accepted Phase 4.0.7 evidence
        |
        v
Public API and invariant audit
        |
        v
Compatible defect fixes only
        |
        v
Conformance and integration gates
        |
        v
Cross-platform race and deterministic validation
        |
        v
Release package and qualification review
        |
        v
Explicit engineering acceptance
        |
        v
Version promotion, commit, and namespaced tags
```

No tag or version promotion occurs before the explicit acceptance step.

## Release qualification policy

Phase 4.0.7 was accepted with one open qualification: the Kubernetes
one-worker Windows and Ubuntu matrices could not complete within the safe
memory ceilings of the available host. Completed Kubernetes Windows
eight-worker executions were deterministic and no correctness defect was
observed.

Phase 4.0.8 must:

1. preserve that finding without reclassifying it as a pass;
2. include it in the qualification register and release notes;
3. permit append-only larger-host evidence later;
4. require engineering to decide explicitly whether the qualification is
   acceptable for `1.0.0` at the final release gate.

The phase must not weaken memory safety limits or silently omit the matrix.

## Allowed implementation changes

After design acceptance, only these changes are permitted:

- compatible defect fixes supported by a failing regression;
- internal performance or memory improvements that preserve exact outputs;
- tests, benchmarks, audit tooling, documentation, and release metadata;
- version promotion after the final release decision.

Every observable-output-preserving optimization must prove identical
authoritative artifact bytes, digests, dependency graphs, diagnostics,
statistics, omission counts, and error semantics for affected fixtures.

## Exit gate

Phase 4.0.8 is complete only when:

- the public API review has no unresolved breaking ambiguity;
- all frozen algorithms have committed golden vectors;
- conformance and required quality gates pass;
- resource and performance results are reviewed;
- source privacy, scope isolation, secret, dependency, listener, execution,
  and mutation audits pass;
- release documentation and the qualification register are complete;
- engineering explicitly accepts the release evidence and qualification;
- approved version changes are committed and pushed before tags are created;
- annotated namespaced tags point to the reviewed commit.

## Still gated

Phase 4.1 transport design and every later implementation phase remain
unauthorized until Repository Service `1.0.0` is explicitly accepted and
released.
