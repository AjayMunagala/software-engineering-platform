# Repository Service Real-Repository Validation Plan

## Status

- Phase: 4.0.7 design
- Status: Approved with recommendations on 2026-07-29
- Execution: complete on available host
- Acceptance: accepted with one open release qualification on 2026-07-30
- Phase 4.0.8 design: authorized
- Phase 4.0.8 implementation: unauthorized pending design review
- Date: 2026-07-29

## Purpose

Define the reproducible evidence required to accept the complete Repository
Service candidate on pinned real-world repositories before stabilization and a
future `1.0.0` freeze.

## Review package

Engineering reviews these documents together:

- `docs/Architecture/REPOSITORY_SERVICE_REAL_REPOSITORY_VALIDATION.md`;
- `docs/Validation/REPOSITORY_SERVICE_REAL_REPOSITORY_FIXTURE_MANIFEST.md`;
- this validation plan;
- `docs/Decisions/0018-pinned-real-repository-service-validation.md`.

Engineering approved this package on 2026-07-29. Validation-owned harness work,
pinned preflight fetches, execution, and compatible defect fixes are authorized.
Engineering accepted the resulting evidence on 2026-07-30. Phase 4.0.8 design
is authorized; stabilization implementation remains unauthorized pending its
design review.

## Required environments

Evidence is collected separately for:

- Windows 11 with the pinned Go and MSYS2 race toolchain;
- Ubuntu 24.04 under WSL with the checksum-verified pinned Go toolchain;
- disposable PostgreSQL 18.4 initialized only through accepted migrations.

The final report records OS/build, kernel, CPU, logical cores, visible RAM,
storage/filesystem, Go, compiler, PostgreSQL, Atlas, repository-service commit,
migration checksum, pool configuration, worker limits, operational limits, and
cache state. Repository preflight additionally records the exact Git version.
Credentials are disposable and never recorded.

## Harness deliverables

After design acceptance, implementation may add only validation-owned code:

- a fixture preflight and canonical-manifest tool;
- a service runner using production composition and no alternate product path;
- structured result capture;
- independent artifact export/digest verification;
- Windows and Ubuntu operator wrappers;
- cleanup and leak assertions.

The harness must not introduce a new public API, engine behavior, persistence
capability, runtime contract, transport, or remote-source feature.

## Mandatory ordering

For every code revision tested:

1. neutral Repository Service conformance;
2. PostgreSQL adapter conformance;
3. targeted integration regression;
4. fixture preflight;
5. small non-Go and small Go cases;
6. medium, generics, workspace, and unavailable-dependency cases;
7. controlled malformed, stale, cancellation, and failure-injection cases;
8. Kubernetes-scale case in a clean process;
9. cross-process, worker-count, and cross-platform comparisons;
10. full regression, vet, shuffle, race, coverage, audits, and benchmarks.

A failure in an earlier gate stops dependent later gates. Kubernetes is not run
to conceal failures in smaller, easier-to-diagnose cases.

## Functional gates

For each successful scan:

- repository registration and retry are idempotent;
- the scan transitions through allowed states to exactly one success;
- exactly seven or ten profile artifacts are atomically visible;
- ExecuteScan and persisted ListArtifacts results match the released
  deterministic artifact-name order; dependency ordinals retain their frozen
  per-consumer manifest meaning;
- every public artifact ID maps to the expected physical UUID;
- every dependency edge is complete, same-scan, ordered, and acyclic;
- every export byte count and SHA-256 matches the envelope and payload;
- no artifact is visible before publication;
- no extra staged payload becomes a visible artifact;
- detached returned views cannot mutate stored state;
- runtime admission, readiness, drain, and shutdown behave as released.

## Determinism gates

For every mandatory real repository:

- two clean-process scans with the same worker count have identical normalized
  durable outcomes;
- one-worker and eight-worker outcomes are identical;
- Windows and Ubuntu outcomes are identical when source manifests match;
- discovery order perturbation does not change artifact order or bytes;
- exact bytes, digests, dependencies, diagnostics, statistics, and omission
  counts match. Scan-bound public IDs and physical UUIDs are independently
  recomputed for each scan; they match directly only when isolated passes use
  the same fixed Scan ID.

The report publishes the normalization schema and both pre-normalization and
normalized result digests. Only IDs/timestamps and operational measurements
listed in the architecture document may be excluded.

## Failure, cancellation, and reconciliation gates

- cancel before analysis: no publication and documented canceled state;
- cancel during analysis: bounded observation and cleanup;
- cancel during materialization/staging: no partial publication and no leaked
  readers or spools;
- cancel after durable publication: published success remains authoritative;
- inject pre-commit failure: zero visible artifact envelopes;
- inject commit-success/response-loss: complete-state reconciliation returns
  success;
- publish successfully, force PostgreSQL to stop immediately, restart it, then
  verify the scan, envelopes, dependencies, exact exports, and SHA-256 proofs;
- malformed source: explicit artifact diagnostics, no panic;
- stale source: digest mismatch is explicit and stale facts are not published;
- all-interest cancellation and independent waiter cancellation retain the
  accepted single-flight semantics;
- runtime drain rejects new work and bounds admitted work.

Cancellation latency is recorded per stage. A synchronous standard-library
parser/type-check call remains the documented cooperative upper bound; any
material regression is reviewed rather than hidden by a larger timeout.

## Scope and privacy gates

- a second scope cannot infer repository, scan, artifact, or payload existence;
- wrong-scope get/list/export/cancel/archive behavior matches not-found policy;
- source handle and repository root canaries are absent in durable rows,
  exported artifacts, results, errors, logs, metrics, and reports;
- physical artifact UUIDs remain internal;
- database URLs, passwords, TLS keys, SQL, driver errors, and payload content
  are absent from logs and metrics;
- outbound network and command execution during analysis are zero;
- repository source bytes and metadata are unchanged after every case.

## Performance and memory evidence

Measurements are evidence, not permission to weaken correctness limits.

For each repository record p50/p95/max where repeated samples are practical:

- total scan wall time;
- RIE, Go syntax, package identity, semantic, codec/materialization, staging,
  publication, query, and export time;
- service-only overhead;
- files per second and payload stage/export throughput;
- peak Go heap, total allocation traffic, allocations, and process working set;
- PostgreSQL database/WAL growth, chunk count, and pool usage;
- cleanup and shutdown time.

Release-candidate comparisons use a warm operating-system cache. First-pass
filesystem-inclusive values are recorded separately. Kubernetes uses separate
clean processes and at most eight workers. The test is stopped safely and
reported failed if host memory pressure threatens system stability.

Candidate acceptance targets:

- no regression beyond 20% against the accepted component baselines without
  a measured explanation and engineering approval;
- service orchestration overhead remains below the accepted 25 ms p95 target
  for the synthetic 20-artifact benchmark;
- materialization and export remain streaming with zero extra complete payload
  copies;
- PostgreSQL publication remains within the accepted persistence target;
- no unbounded growth across 25 repeated medium-repository lifecycle cycles.

Every Kubernetes-scale run records exactly one terminal classification:
`success`, `memory_ceiling`, `timeout`, `correctness_failure`, or
`environment_failure`. A generic resource-failure category is not accepted.

Real-repository end-to-end duration has no invented universal pass number in
this phase; it is compared with upstream engine baselines and classified using
measured stage attribution.

## Quality gates

From `backend/`:

```text
go test ./...
go test -shuffle=on ./...
go vet ./...
go test -race ./...
```

Additionally require:

- targeted Windows and Ubuntu race runs;
- changed-package statement coverage at or above 85%;
- repeatable benchmarks with environment details;
- fuzzing of fixture manifests, result normalization, identifier translation,
  redaction, and failure translation;
- dependency, secret, listener, network, command, path, and mutation audits;
- zero data races, crashes, panics, scope escapes, or unexplained leaks.

## Manual verification

For every repository, independently inspect at least:

- repository identity and profile branch;
- file/language/framework/build counts against released artifacts;
- one artifact envelope, dependency chain, staged payload, and exact export;
- representative diagnostic/partial/omission facts;
- database publication visibility before and after commit;
- absence of source-path and secret canaries.

Manual samples and the evidence used to verify them are recorded in the final
report. Manual judgment does not replace automated exact-byte gates.

## Defect handling

Each defect records repository, revision, environment, severity, category,
reproducer, root cause, fix commit, targeted regression, affected-case rerun,
and final status. Critical defects block acceptance. Documented limitations
must be evidence-backed and must not be relabeled defects merely to pass.

## Exit gate

Phase 4.0.7 may be accepted only when:

- every mandatory corpus case completes or has an approved, evidence-backed
  limitation that does not violate the candidate contract;
- all functional, determinism, integrity, privacy, scope, cancellation,
  failure, race, and audit gates pass;
- all critical/high defects are fixed and revalidated;
- performance and memory findings are stage-attributed and reviewed;
- the validation report and machine-readable results are committed and pushed;
- engineering explicitly accepts the evidence.

Engineering granted that acceptance on 2026-07-30 while retaining the
larger-host Kubernetes matrix as an open release qualification. Phase 4.0.8
design is authorized. Stabilization implementation, API freeze, version
promotion, and release tagging remain unauthorized pending design review.
