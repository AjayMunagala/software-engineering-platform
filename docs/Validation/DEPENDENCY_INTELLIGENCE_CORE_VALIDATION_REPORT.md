# Phase 5.0.2 Neutral Graph Artifact and Core Validation Report

## Status

- Date: 2026-09-08
- Implementation: complete review candidate
- Engineering acceptance: pending
- ADR 0020: Accepted
- Phase 5.0.3: not authorized
- Production release: not authorized

## Scope

The production `backend/die` package implements only the language-neutral
Phase 5.0.2 contract:

- immutable `DependencyInventory` candidate `0.1.0`;
- node, containment, dependency, evidence, diagnostic, and statistics models;
- deterministic normalization and duplicate aggregation;
- frozen node, edge, containment, SCC, and cycle ID contracts;
- canonical ordering and detached accessors/views;
- hard `MaxNodes` enforcement;
- bounded edges, containment, evidence, and diagnostics with exact omissions;
- stable redacted errors and context cancellation semantics;
- reusable adapter-independent conformance harness.

The strong-component and cycle collections are modeled but remain empty until
the separately gated Phase 5.0.4 algorithms. No Go adapter, RIE/LIE import,
persistence, runtime, Repository Service, transport, filesystem, network,
process execution, AI, or repository mutation was introduced.

## Required governance refinements

| Requirement from Phase 5.0.1 acceptance | Evidence | Result |
|---|---|---|
| Production `MaxNodes` bound | Unique normalized nodes are counted before publication; overflow returns `limit_exceeded` and no inventory | Pass |
| Containment identity before publication | Field order is kind, parent node ID, child node ID; exact vector is committed and tested | Pass |
| Two-GiB synthetic peak-live-heap gate | Explicit 100k-node/1m-edge sampled scale test on Windows and Ubuntu | Pass |
| Reduce spike allocations | Total allocation reduced from about 2.77 GB to about 1.45 GB for the exact benchmark shape | Pass |

## Frozen containment vector

Input:

```text
kind=module_contains_package
parent_id=dependency-node-id/v1:sha256:parent
child_id=dependency-node-id/v1:sha256:child
```

Output:

```text
dependency-containment-id/v1:sha256:3b47d635724cdb8ab06ff0319c2ded1e67d1289fc6a92df92a02a0c29f0c4334
```

The production test also reasserts the frozen Phase 5.0.1 node and edge
vectors. Windows and Ubuntu produce the same values.

## Functional evidence

Tests cover:

- empty repositories and non-null empty JSON arrays;
- canonical source, node, containment, edge, evidence, and diagnostic order;
- duplicate node/evidence/edge aggregation;
- exact occurrence and omission counts;
- hard node failure and deterministic collection caps;
- missing endpoint integrity failures;
- invalid classifications, sources, digests, evidence, and diagnostics;
- repository-relative slash normalization and path-escape rejection;
- accessor/view defensive copies;
- byte-identical repeated and shuffled-input output;
- stable error kinds/codes and cancellation unwrapping;
- reusable reference-core conformance.

## Quality results

| Gate | Windows | Ubuntu 24.04 WSL |
|---|---|---|
| Package tests | Pass | Pass |
| Shuffled package tests | Pass, 10 repeats | Pass |
| `go vet` | Pass | Pass |
| Targeted race | Pass, zero races | Pass, zero races |
| Full backend regression | Pass | Pass |
| Full backend race | Pass, zero races | Pass, zero races |
| Core statement coverage | 88.3% | 88.3% |
| Conformance statement coverage | 86.2% | 86.2% |
| Bounded fuzzing | 919,477 executions, no panic | Not required |

## Performance and memory

Reference host: 12th Gen Intel Core i5-12450H, amd64, Go 1.26.2. The exact
fixture contains 100,000 unique package nodes and 1,000,000 unique dependency
edges. Input construction is outside the timed benchmark region.

| Measurement | Windows | Ubuntu 24.04 WSL |
|---|---:|---:|
| Normalize benchmark | 3.027 s | 3.681 s |
| Total allocated bytes/op | 1,451,668,928 | 1,451,702,280 |
| Allocations/op | 14,708,786 | 14,708,758 |
| Sampled scale-gate elapsed | 4.191 s | 4.573 s |
| Peak live Go heap | 1,026,210,664 bytes | 1,029,284,928 bytes |

Both scale runs pass the accepted 30-second and two-GiB candidate gates. The
sampled scale duration includes frequent `runtime.ReadMemStats` sampling and
is intentionally kept separate from the normal benchmark.

## Security and dependency audit

The production packages import no RIE, Go LIE, persistence, PostgreSQL,
runtime, Repository Service, SQL, HTTP, filesystem traversal, network,
execution, or model/LLM packages. The artifact rejects absolute/escaping
repository paths and line breaks/NULs in safe text. Errors expose stable kinds
and redacted messages rather than implementation failures or input contents.

## Exit-gate assessment

Phase 5.0.2 implementation satisfies its local functional, immutability,
determinism, safety, cross-platform, race, coverage, fuzz, and performance
objectives. This report requests engineering acceptance only for Phase 5.0.2.
Phase 5.0.3 and every later milestone remain unauthorized.
