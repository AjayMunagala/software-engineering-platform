# Dependency Intelligence Engine Validation Plan

## Status

- Phase: 5.0 design
- Applies to candidate: `0.1.0`
- Execution: not authorized until the relevant milestone is approved

## Purpose

Prove that Dependency Intelligence produces correct, immutable, bounded,
deterministic, evidence-backed structural graphs without source rereads,
repository execution, network access, or changes to released platform
contracts.

## Validation order

1. golden-vector and model tests;
2. core graph conformance tests;
3. Go adapter tests;
4. SCC, cycle, and impact tests;
5. full backend regression and dependency audits;
6. race, fuzz, and benchmark gates;
7. pinned real-repository validation;
8. stabilization and release review.

Later steps do not replace earlier failures.

## Correctness fixtures

Minimum controlled fixtures:

- empty repository;
- one Go module and one package;
- multiple packages with a directed acyclic import graph;
- nested modules;
- `go.work` workspace;
- standard-library imports;
- external resolved package identities;
- unresolved import;
- ambiguous proof;
- stale semantic/package proof;
- duplicate import evidence;
- cross-file local references;
- isolated files/packages/modules;
- legal file-level cycle inside one Go package;
- malformed package import cycle;
- module-level cycle;
- self-loop;
- Unicode module/package/file identities;
- repository-relative paths with spaces;
- maximum evidence/edge/diagnostic limits;
- cancellation at every documented checkpoint.

## Required assertions

### Graph truth

- every local node maps to exactly one released source identity;
- every resolved edge has valid source and target nodes;
- containment and dependency are never conflated;
- aggregated occurrence counts equal source facts;
- omitted evidence retains exact occurrence and omission counts;
- no unresolved/ambiguous/stale fact becomes a resolved local edge;
- standard library and external boundaries are explicit;
- no absolute path or source text appears in the artifact.

### SCC and cycles

- SCC partition contains every graph node exactly once where applicable;
- deterministic SCC results match a simple reference implementation;
- self-loops are distinguished from multi-node cycles;
- legal file-level structural cycles are not labeled language-invalid;
- no exponential enumeration of simple cycles occurs.

### Impact queries

- direct dependencies and reverse dependents are exact;
- breadth-first traversal order is canonical;
- depth and node limits are enforced deterministically;
- truncation and unresolved-boundary states are explicit;
- repeated/paginated queries neither skip nor duplicate results;
- cancellation returns no misleading complete result.

## Determinism matrix

For identical released input artifact bytes and configuration, compare exact
artifact bytes and query results across:

- three clean-process repeats;
- one worker and eight workers;
- Windows and Ubuntu;
- randomized insertion/map order;
- shuffled test order;
- repeated serialization.

Compare stable IDs, nodes, containment, edges, SCCs, cycles, diagnostics,
statistics, omissions, and canonical JSON SHA-256.

## Immutability and API

- mutate every constructor input after construction;
- mutate every accessor result;
- mutate detached presentation views;
- run concurrent reads under the race detector;
- prove the original artifact bytes remain unchanged;
- reject incorrect names/versions and inconsistent prerequisite graphs;
- freeze API and ID golden vectors before production implementation.

## Performance targets proposed for review

Reference CI runner, warm operating-system cache, at most eight workers:

- 100,000 nodes and 1,000,000 direct edges: build plus SCC analysis under
  30 seconds;
- peak live Go heap no greater than 1.5 times normalized input-plus-output
  bytes for the synthetic graph gate, excluding the already materialized
  prerequisite artifacts;
- bounded impact of 100,000 reached nodes and 1,000,000 traversed edges under
  5 seconds;
- cancellation observed within one bounded unit and normally within 250 ms on
  the reference runner;
- no superlinear graph work other than canonical sorting.

The Phase 5.0.1 spike must validate or revise these targets before they become
release gates. Cold filesystem time is irrelevant because DIE performs no
filesystem I/O.

## Benchmarks

Repeatable synthetic benchmarks:

- sparse chain;
- wide fan-out;
- wide fan-in;
- layered DAG;
- many small SCCs;
- one large SCC;
- disconnected graph;
- evidence-heavy duplicate aggregation;
- bounded reverse-impact traversal.

Record Go version, commit, OS, architecture, CPU, RAM, worker count, fixture
seed/digest, nodes, edges, SCCs, elapsed time, allocations, bytes allocated,
peak live heap, diagnostics, and omissions.

## Fuzz and property testing

- ID canonical encoding never aliases distinct normalized identities;
- edge aggregation is commutative with respect to input order;
- SCC partition is stable and complete;
- serialization round trips preserve exact logical content;
- path normalization never emits an absolute/escaping path;
- arbitrary invalid inputs fail safely without panic;
- traversal limits always terminate.

## Security and dependency audits

Production DIE packages must not import or invoke:

- filesystem traversal or source readers;
- `os/exec` or shell execution;
- network clients;
- database/SQL/pgx packages;
- HTTP/gRPC transports;
- LLM/model SDKs;
- Repository Service internals;
- mutable RIE run context.

Scan committed outputs for credentials, absolute local paths, source handles,
raw source, and host-specific values.

## Real-repository corpus

Pin exact commits for:

- small Go CLI;
- medium Go service;
- generics-heavy library;
- multi-module/workspace repository;
- large Kubernetes-style repository;
- controlled unresolved, ambiguous, stale, and cyclic fixtures.

Record prerequisite artifact digests, nodes, edges by graph/state, SCCs, cycles,
diagnostics, omissions, timings, memory, workers, normalized output digest, Git
version, and exact repository revision.

## Quality gates

- unit/integration statement coverage at least 85 percent for production
  packages;
- full backend regression passes;
- `go vet ./...` passes;
- Windows and Ubuntu race suites pass with zero data races;
- shuffled tests pass repeatedly;
- fuzz/property targets meet the approved execution budget;
- benchmarks are repeatable and within accepted gates;
- no panic, repository mutation, network, or external execution;
- all known limitations are documented.

## Phase acceptance

No milestone self-authorizes the next. Each validation report must be committed
and reviewed. `DependencyInventory 1.0.0` promotion requires explicit final
engineering acceptance and annotated release tags.
