# Dependency Intelligence Validation Architecture

## Status

- Phase: 5.0 design
- Applies to: `DependencyInventory 0.1.0` candidate
- Validation execution: milestone-gated

## Purpose

Define how Dependency Intelligence evidence is collected without weakening the
production boundary or confusing fixture construction with engine behavior.

## Validation layers

```text
Golden vectors and immutable model tests
        |
        v
Neutral graph conformance and property tests
        |
        v
Go artifact-adapter fixtures
        |
        v
SCC, cycle, and bounded-impact validation
        |
        v
Released-artifact integration
        |
        v
Pinned real-repository validation
        |
        v
Cross-platform stabilization evidence
```

Each layer must pass before evidence from the next layer is used for milestone
acceptance.

## Fixture boundary

Controlled fixtures may construct released prerequisite artifacts through
their public constructors or run their released engines in a separate fixture
preparation step. The timed/validated DIE operation receives immutable
artifacts only. Tests must not give production DIE code a repository path,
source reader, AST, database, network client, or command executor.

Real-repository fixtures are pinned by repository URL and exact commit, but
checkout/preflight belongs to the validation harness outside DIE. The harness
records Git version, commit, tree digest, prerequisite artifact digests, OS,
Go version, CPU, RAM, worker count, and safety limits.

## Reference oracles

- hand-authored expected graphs for small fixtures;
- a simple single-threaded adjacency builder for aggregation comparison;
- an independently implemented SCC oracle for generated graphs;
- bounded breadth-first reference traversal for impact queries;
- committed canonical-byte and stable-ID golden vectors.

An implementation must not validate itself solely by comparing two executions
of the same algorithm.

## Evidence isolation

Authoritative comparisons exclude elapsed time, process IDs, timestamps,
absolute paths, memory addresses, hostnames, and database identities. Those are
recorded only as non-authoritative benchmark environment metadata.

Every result bundle contains:

- input artifact names, versions, and SHA-256 digests;
- engine and ID-scheme versions;
- configuration and worker count;
- canonical output SHA-256;
- nodes, edges, SCCs, cycles, diagnostics, and omission statistics;
- pass/fail/qualified status for each gate;
- defect classification and traceable fix commit when applicable.

## Failure classification

Failures are classified as:

- correctness;
- determinism;
- integrity/provenance;
- performance;
- memory ceiling;
- cancellation;
- race/concurrency;
- security/boundary;
- environment/tooling;
- known limitation.

An environment limitation is never reported as a pass. It remains a visible
qualification until append-only evidence closes it.

## Platform matrix

Mandatory release evidence includes Windows and Ubuntu, one and eight workers,
repeated clean processes, randomized input insertion order, race-capable
execution, and the approved pinned repository corpus. Exact canonical output
must match where inputs and configuration match.

## Resource protection

The harness applies declared time and memory ceilings and terminates safely
before destabilizing the host. It records whether termination was caused by a
memory ceiling, timeout, cancellation, correctness failure, or environment
failure. It never collapses these into a generic resource error.

## Governance

Spike evidence may change candidate algorithms or targets before production
implementation. Production validation may produce compatible defect fixes but
cannot add capability. Every report is committed, pushed, and reviewed before
the next milestone is authorized.
