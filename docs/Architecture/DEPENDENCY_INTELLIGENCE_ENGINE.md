# Dependency Intelligence Engine Architecture

## Status

- Phase: 5.0 design
- Design status: review candidate
- Production implementation: not authorized
- Candidate contract: `0.1.0`

## Responsibility

The Dependency Intelligence Engine (DIE) deterministically derives structural
module, package, and file dependency graphs from released immutable artifacts
without rereading source code, executing repository tools, using the network,
or guessing unresolved relationships.

## Questions answered

- Which modules, packages, and files depend on which other nodes?
- Which dependencies point into the repository, standard library, or an
  unresolved/external boundary?
- Which nodes directly or transitively depend on a selected node?
- Which strongly connected components and structural cycles exist?
- What bounded impact boundary follows from changing a node?

The engine reports evidence. It does not decide whether a dependency is
architecturally desirable.

## Inputs

The first adapter is Go-backed because Go is the only released semantic
language contract. The core artifact is language-neutral.

Required frozen inputs:

| Artifact | Required facts |
|---|---|
| `RepositorySnapshot 1.0.0` | repository-relative file identity and repository boundary |
| `GoLanguageInventory 1.0.0` | packages, files, and declared imports |
| `GoPackageIdentityInventory 1.0.0` | modules and proof-backed package identities |
| `GoSemanticInventory 1.0.0` | resolved import bindings and exact file/package identities |

`BuildInventory` is not a mandatory input for the Go adapter because the
released package-identity artifact already owns authoritative Go module and
workspace proof. Future adapters may consume the lowest released artifact
that contains their ecosystem facts.

The engine rejects missing or incompatible artifact versions. It never falls
back to mutable `RunContext` data, presentation summaries, filesystem scans,
manifest parsing, or source parsing.

## Output

The engine publishes one immutable, versioned `DependencyInventory` candidate.
Its contract is specified in `DEPENDENCY_INTELLIGENCE_ARTIFACTS.md`.

The inventory contains:

- typed nodes;
- typed directed edges;
- explicit containment relationships;
- strongly connected components;
- cycle findings where cycles are meaningful for that graph;
- source artifact references and evidence;
- deterministic diagnostics and statistics.

It does not serialize an all-pairs transitive closure. Direct adjacency is the
source of truth; bounded reachability and impact are deterministic queries over
that graph.

## Graph semantics

### Module graph

A module node represents a proof-backed module identity. An edge `A -> B`
means code in module A contains at least one proof-backed import of a package
owned by module B. Standard-library and unresolved/external targets are kept as
explicit boundary nodes when applicable.

### Package import graph

A package node represents a proof-backed package identity. An edge `A -> B`
means a source file in package A imports package B. Multiple import statements
are aggregated into one edge with sorted evidence and an occurrence count.

### File dependency graph

A file node represents one released semantic file identity. A directed edge
`A -> B` is emitted only when released semantic evidence proves that a
reference originating in A targets a declaration owned by B, or when an import
binding provides an exact package-boundary dependency represented by a boundary
node. Unresolved and ambiguous references are never converted into guessed
file targets.

File cycles inside one Go package are normal structural facts. They must not be
reported as architectural defects. Cycle findings therefore include graph kind
and policy classification rather than a generic warning.

### Containment

Module-to-package and package-to-file relationships are containment, not
dependency edges. They are represented separately so consumers cannot confuse
ownership with coupling.

## Resolution states

Every edge is one of:

- `resolved_local`;
- `standard_library`;
- `external`;
- `unresolved`;
- `ambiguous`;
- `stale`.

Unknown is preferable to guessed. An unresolved, ambiguous, or stale source
fact is preserved as a diagnostic/statistic or boundary edge only when its
identity is safe and deterministic. It never becomes a resolved local edge.

## Algorithms and complexity

- graph construction: `O(V + E)` time and memory;
- canonical sorting: `O(V log V + E log E)`;
- strongly connected components: deterministic Tarjan or Kosaraju,
  `O(V + E)`;
- direct dependency/dependent lookup: adjacency-index based;
- bounded reachability and impact: `O(V_reached + E_traversed)`;
- no unrestricted all-pairs reachability;
- no `all interfaces x all types` or other Cartesian analysis.

Serialized slices are canonical. Maps may be used only as private build/query
indexes and are never relied on for output order.

## Pipeline

```text
Released immutable artifacts
        |
        v
Language-specific dependency adapter (Go first)
        |
        v
Normalized structural graph
        |
        +--> deterministic SCC/cycle analysis
        +--> bounded impact query support
        |
        v
DependencyInventory 0.1.0 candidate
```

Future language adapters publish into the same technology-neutral artifact
model. They do not change the core graph engine.

## Configuration

Configuration is immutable after construction and uses documented zero-value
defaults. The candidate includes:

- maximum nodes;
- maximum edges;
- maximum evidence items per edge;
- maximum diagnostics;
- maximum traversal depth;
- maximum traversal results;
- maximum workers, capped at eight.

Limits fail explicitly or publish deterministic omission counts according to
the candidate API. They never silently truncate in iteration order.

## Cancellation

The engine checks cancellation:

- before reading each prerequisite artifact;
- at bounded node/edge construction batches;
- between graph normalization and SCC analysis;
- at each SCC root/bounded batch;
- during each bounded traversal batch;
- before artifact publication.

No single cancellation unit may exceed one file, one package, or 1,024 graph
items, whichever is smaller for the operation.

## Error and diagnostic policy

Errors stop artifact publication for invalid input, incompatible versions,
corrupt invariants, cancellation, or exceeded hard safety limits.

Diagnostics describe safe partial knowledge such as unresolved imports, stale
proofs, ambiguous identities, and deterministic omissions. Ordering is by code,
graph kind, source identity, location, then stable ID. Exact duplicates are
suppressed before the configured limit is applied.

## Package structure after implementation authorization

```text
backend/die/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    *_test.go
    *_benchmark_test.go

backend/die/golang/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    *_test.go
    *_benchmark_test.go
```

This is a proposed implementation layout, not authorization to create the
packages during Phase 5.0.

## In scope for the first release

- Go module graph;
- Go package import graph;
- proof-backed file dependency graph;
- local, standard-library, external, unresolved, ambiguous, and stale states;
- deterministic SCC and cycle detection;
- direct dependencies and reverse dependents;
- bounded reachability and change-impact boundaries;
- immutable canonical artifact and stable provenance.

## Out of scope

- call graphs;
- control flow and data flow;
- runtime/dynamic dependencies;
- build execution or package-manager execution;
- network/module downloads;
- vulnerability lookup;
- architecture-layer inference or policy enforcement;
- coupling-quality scores without an approved definition;
- AI/LLM reasoning;
- refactoring, code generation, patching, or repository mutation;
- transport APIs, UI, authentication, or authorization;
- changes to released RIE, Go LIE, Persistence, Runtime, or Repository Service
  contracts.

## Release principles

The first stable release must be correct, fast, testable, reusable,
deterministic, immutable, and evidence-backed. The candidate stays at `0.1.0`
until real-repository validation and stabilization are accepted.
