# Repository Service Real-Repository Validation Architecture

## Status

- Phase: 4.0.7 design
- Status: Approved with recommendations on 2026-07-29
- Implementation/validation execution: authorized
- Date: 2026-07-29

## One-sentence responsibility

Phase 4.0.7 proves that the accepted Repository Service pipeline remains
correct, deterministic, private, bounded, and operational on pinned real-world
repositories without adding product capabilities or changing frozen contracts.

## In scope

- execute the complete released Repository Service path from registration
  through exact artifact export;
- validate the seven-artifact non-Go and ten-artifact Go profile branches;
- exercise small, medium, multi-module, unavailable-dependency, and large
  repositories at exact commits;
- validate malformed and digest-stale controlled fixtures;
- compare repeated, worker-count, process, and operating-system results;
- measure discovery, analysis, materialization, staging, publication, export,
  memory, and service overhead separately;
- verify cancellation, failure isolation, scope isolation, source privacy,
  exact bytes, dependency order, and atomic visibility;
- record defects and compatible fixes discovered by validation.

## Out of scope

- new RIE, Go LIE, persistence, runtime, or Repository Service behavior;
- REST, gRPC, HTTP health endpoints, UI, authentication, or authorization
  policy;
- remote repository cloning or fetching by the service;
- repository build, test, package-manager, generator, or arbitrary command
  execution;
- dependency download, network fallback, or external Go importing;
- distributed scheduling, queues, leases, or background workers;
- performance tuning without a reproduced, classified defect;
- Phase 4.0.8 API freeze or release tagging.

## Frozen dependencies

Validation consumes these unchanged contracts:

- RIE `1.0.0`;
- Go Language, Package Identity, and Semantic inventories `1.0.0`;
- Persistence Port and PostgreSQL Adapter `1.0.0`;
- Runtime Infrastructure `1.0.0`;
- Repository Service candidate contract `0.1.0`;
- `repository-go/v1` profile and digest;
- `canonical-json/1.0.0` codecs;
- `repository-service-artifact-id/v1`;
- `repository-service-storage-artifact-id/v1`;
- `repository-service-manifest/v1`;
- accepted PostgreSQL migrations and four-MiB payload contract.

Any required contract change stops validation and returns the project to a
separate design decision. Phase 4.0.7 may produce compatible defect fixes, but
it may not silently broaden a frozen contract.

## Validation topology

```text
Pinned clean repository tree
        |
        v
Deployment-owned source resolver
        |
        v
Repository Service 0.1.0 candidate
        |
        +--> RIE 1.0.0
        +--> Go LIE 1.0.0 when Go is present
        |
        v
Sealed exact-byte materialization
        |
        v
Runtime admission + PostgreSQL Adapter 1.0.0
        |
        v
Atomic scan publication
        |
        v
List / get / exact streaming export / independent digest proof
```

Each case uses a disposable database created from accepted migrations. The
service never runs migrations. The validation host owns checkout preparation,
credentials, PostgreSQL lifecycle, source handles, and cleanup.

## Repository corpus

The normative corpus and exact revisions are defined in
`docs/Validation/REPOSITORY_SERVICE_REAL_REPOSITORY_FIXTURE_MANIFEST.md`.
It covers:

- small non-Go repository;
- framework-rich non-Go repository;
- small Go CLI;
- medium Go service;
- generics-heavy Go library;
- Go workspace/multi-module repository;
- large repository with unavailable external dependencies;
- Kubernetes-scale repository;
- malformed and stale controlled fixtures.

The corpus reuses revisions already exercised by released RIE and Go LIE
validation so Phase 4.0.7 measures service integration risk rather than moving
upstream inputs.

## Checkout and network boundary

The operator may fetch a pinned revision before the measured run. Every fetch
occurs outside the Repository Service, outside the timed region, and outside
the project working tree. Before analysis, the harness proves:

- exact commit and Git tree identity;
- clean tracked state;
- no untracked files inside the authorized source root;
- no initialized dependency or generated-output mutation;
- a canonical path-free file manifest for cross-platform equivalence.

After preflight, service execution runs with outbound network denied or
audited as zero. No repository-owned executable or script is invoked.

## Isolation model

Every test case receives explicit canonical UUID values for scope, repository,
registration request, scan request, and scan. Each negative test uses a second
scope and repository. The harness must prove that the second scope cannot
observe existence through get, list, cancel, export, or mutation behavior.

Repository roots, source handles, authenticated URLs, credentials, SQL,
payload contents, and raw dependency errors must not occur in durable models,
returned errors, structured events, metric labels, or the published report.
The redaction audit searches native, slash-normalized, and JSON-escaped forms
of every secret or path canary.

## Execution passes

For every primary real repository:

1. prepare and verify the pinned clean tree;
2. create the disposable migrated database and compatible runtime;
3. register the repository and verify idempotent registration;
4. run scan A with the approved worker setting;
5. list and get the published scan and every artifact;
6. stream-export every artifact and independently verify bytes and SHA-256;
7. repeat as scan B in a clean service process;
8. compare normalized durable outcomes;
9. run the selected worker-count and cross-platform passes;
10. drain, stop, verify cleanup, and remove disposable state.

Kubernetes-scale passes run in separate clean processes. The harness does not
retain two Kubernetes semantic artifacts in one process.

## Determinism contract

The following must match for equivalent source content and profile:

- ordered artifact names, versions, producers, codecs, and stable public IDs;
- exact artifact byte sizes and SHA-256 digests;
- ordered artifact dependency graph;
- functional diagnostics, warnings, statistics, omission counts, and metadata;
- exported bytes;
- terminal success state and publication contents.

The comparison intentionally excludes scan UUIDs, request UUIDs, persistence-
owned timestamps, elapsed durations, throughput, pool observations, process
IDs, and machine-specific resource measurements. Exclusion rules are fixed in
the validation harness and listed in the report; they are never applied ad hoc.

One-worker versus eight-worker output must match. Windows versus Ubuntu output
must match when the canonical source manifest matches. A source-tree mismatch
is a fixture failure, not an allowed artifact mismatch.

## Failure and cancellation evidence

Controlled tests must prove:

- malformed Go files produce the documented partial/failed facts without
  panic or partial publication;
- source mutation after prerequisite publication produces explicit stale
  evidence and never publishes facts derived from stale bytes;
- cancellation before analysis, during analysis, during staging, and before
  publication reaches the documented terminal behavior;
- injected failure after publication commit but before response is reconciled
  to the complete durable publication;
- after a successful publication, an operator-forced PostgreSQL immediate stop
  and restart completes crash recovery, after which the scan, envelopes,
  dependency graph, exact exports, and SHA-256 proofs remain intact;
- injected pre-publication failures expose no artifact envelopes;
- staged unreferenced payloads remain eligible for the accepted retention
  policy;
- runtime drain refuses new work and bounds admitted-work shutdown.

## Measurement model

Each result records:

- tracked, discovered, ignored, analyzed, parsed, failed, stale, and skipped
  file counts;
- language, framework, build, module, package, declaration, relationship,
  diagnostic, warning, and omission counts available from released artifacts;
- per-engine and total analysis duration;
- materialization, stage, publication, query, and export duration;
- artifact count, bytes, chunks, SHA-256, and dependency count;
- service overhead separately from engine, codec, and database time;
- process peak working set and Go heap peak;
- PostgreSQL database growth, payload bytes, WAL growth, and connection use;
- cancellation latency and cleanup duration;
- errors, retries, reconciliations, crashes, panics, and data races.

Cold filesystem observations are labeled as observations. Warm operating-
system cache measurements are the reproducible comparison baseline. Network
fetch time is never included.

## Resource safety

The reference workstation has limited memory. Large cases therefore use clean
processes, bounded workers, streaming export, and continuous resource sampling.
The harness must stop safely before host instability if the approved memory
ceiling is crossed and preserve logs without claiming a pass. Every
Kubernetes-scale outcome is classified exactly as `success`, `memory_ceiling`,
`timeout`, `correctness_failure`, or `environment_failure`; a generic resource
failure label is forbidden. It must not disable integrity, relationship, or
diagnostic limits to obtain a successful run.

## Defect policy

Every finding is classified as:

- correctness;
- determinism;
- privacy/security;
- integrity/publication;
- performance/memory;
- fixture/environment;
- documented limitation.

Critical correctness, privacy, integrity, crash, panic, scope, or
nondeterminism defects block acceptance. Fixes use focused follow-up commits;
accepted history is not rewritten. Every fix receives a reproducer, regression
test, and re-run of affected cases plus the full required quality gates.

## Deliverables

- approved design and fixture manifest;
- isolated validation harness and operator instructions;
- machine-readable per-run results;
- `REPOSITORY_SERVICE_REAL_REPOSITORY_VALIDATION_REPORT.md`;
- defect register with fix commits and revalidation evidence;
- updated roadmap and project metrics.

## Governance

Engineering approved this design with non-blocking recommendations on
2026-07-29 and authorized Phase 4.0.7 validation execution. The three
recommendations are incorporated as mandatory evidence in the validation
package. Phase 4.0.8 remains separately gated.
