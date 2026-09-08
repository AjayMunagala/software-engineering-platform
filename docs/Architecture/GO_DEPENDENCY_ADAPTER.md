# Phase 5.0.3 Go Dependency Adapter Design

## Status and review boundary

Design approved, implementation review candidate, 2026-09-08. Phase 5.0.2 is accepted at
`e8c78a2b762131bd679c007ea2d26b73c67bd475`; governance commit `5f32d905`
records that acceptance. Phase 5.0.3 implementation was explicitly authorized;
its evidence is in `docs/Validation/GO_DEPENDENCY_ADAPTER_VALIDATION_REPORT.md`.
Engineering acceptance remains pending. Phase 5.0.4 algorithms, integration,
and production release remain unauthorized.

Review this document with `GO_DEPENDENCY_ADAPTER_CANDIDATE_API.md`, ADR 0021,
and `GO_DEPENDENCY_ADAPTER_VALIDATION_PLAN.md`.

## Responsibility and ownership

The proposed `backend/die/golang` package translates existing Go facts into
`die.GraphInput`, then invokes `die.Core.Normalize` once per successful analysis.
It owns proof interpretation and Go-specific translation. The core retains
identity encoding, aggregation, canonical ordering, and immutable output.
The adapter imports the released RIE/Go LIE contracts; the neutral core does
not import the adapter. No released 1.0.0 artifact changes are proposed.

The result remains `DependencyInventory 0.1.0`. Its SCC and cycle collections
remain empty. This phase computes no SCC, cycle, impact, call graph, or type
resolution and adds no Repository Service, database, runtime, transport, or AI
integration. Production analysis performs no filesystem reads, source parsing,
manifest parsing, network requests, cloning, or tool execution.

## Input authority and limits of verification

Required concrete artifacts are `RepositorySnapshot`, `GoLanguageInventory`,
`GoPackageIdentityInventory`, and `GoSemanticInventory`, all version `1.0.0`.
Only public immutable accessors are used. Reject zero/uninitialized artifacts
using their stored metadata, not merely constant-returning name/version methods.

Before translation, validate:

1. stored metadata names, versions, and supported identity schemes;
2. required prerequisite name/version references and conflicting duplicates;
3. unique syntax file/package, semantic declaration/reference/import, module,
   context, and proof IDs; duplicate conflicting IDs fail integrity validation;
4. semantic file IDs and package IDs against syntax, syntax file membership
   against the supplied snapshot, and safe repository-relative paths;
5. syntax/semantic content digests where the released records supply them;
6. binding proof ID, importing package, import path, target identity, resolution
   context, and any candidate IDs against their referenced records.

Malformed joins and contradictions return an error with no inventory. A valid
released record explicitly marked stale/ambiguous/unresolved remains partial
evidence; it is not an integrity failure merely because it lacks a target.
Missing records explained by upstream omissions produce a fixed diagnostic and
no guessed local edge. Unexplained dangling references fail integrity checks.

Released prerequisite references do not contain a common scan digest. These
checks establish internal consistency, not cryptographic same-scan provenance
or current on-disk freshness. The caller must supply a coherent artifact set.
The adapter never invents that missing proof or rereads source to obtain it.
Output artifact references contain supplied names/versions; optional digest
fields stay empty because this phase does not own a frozen serialization codec.

## Deterministic translation

| Fact | Output | Proof requirement |
|---|---|---|
| Module record | `module` node | Unique module ID, valid path/root, declared evidence |
| Syntax package | `package` node | Unique syntax package ID and directory; identity is existence, not successful import resolution |
| Syntax file | `file` node | Unique file ID and snapshot membership; failed/skipped analysis is diagnosed |
| Package within module | `module_contains_package` | Unique deepest path-segment ancestor among declared module roots; equal-root conflicts fail |
| Syntax file package membership | `package_contains_file` | Exact syntax package ID; no directory-name guessing |
| Source import | Package `imports` edge | Exact syntax import occurrence matched to binding and identity proof, or explicit unresolved boundary |
| Cross-module local import | Module `imports` edge | Both package owners established and different; one contribution per source import |
| Semantic reference to declaration | File `references` edge | Resolved reference and declaration, verified file identities/digests, exact target file |

Local qualified names use released module path for modules and released syntax
IDs for packages/files, with repository path included in the frozen node ID.
This preserves independent locations and avoids conflating same-named packages.
SourceIdentity references the authoritative module/package/file record.

Nested modules use normalized segment-aware ancestry (`a/b` does not own `a/bc`).
Workspace membership and replace/vendor precedence are read from identity
proofs, never recomputed from manifest text. Packages outside a declared module
remain package nodes without invented module containment. Standard-library and
external package targets are package-boundary nodes, not invented external
modules. Module edges are limited to proven local module pairs in this phase;
other targets remain visible in package/file boundary edges and diagnostics.

Match imports by source file, location, import path, and alias form. Count an
import occurrence once; the syntax record and matching binding are not two
occurrences. Default, named, dot, and blank imports all retain structural import
dependencies. If a syntax import has no binding, emit an unresolved boundary
with a fixed missing-binding diagnostic rather than silently dropping it.

An exact local import also permits a file-to-package boundary edge in the file
graph; it never permits a guessed file-to-file import edge. File references
use `SemanticReference` only; adding TypeRelation or interface-derived edges
would require a separately reviewed rule. Same-file references and same-module
imports do not create inter-file/inter-module coupling. Genuine package
self-imports remain edges for later algorithm analysis.

## Resolution decision table

| Evidence | Edge state and destination |
|---|---|
| Resolved binding, matching local proof, existing target | `resolved_local`, exact package target |
| Explicit standard-library proof with consistent binding | `standard_library`, standard-library boundary |
| Explicit external proof/binding with no local target | `external`, external-package boundary |
| Explicit ambiguity | `ambiguous`, unresolved-kind boundary; no candidate selected |
| Explicit stale file/proof evidence | `stale`, unresolved-kind boundary; no local relationship |
| Missing/partial resolution without stale evidence | `unresolved`, unresolved-kind boundary |
| Contradictory resolved proof/binding or unknown enum | Integrity/invalid-input error; no inventory |

`ResolutionPartial` alone does not prove staleness. Standard-library status is
never inferred from an import string. Nonlocal boundary qualified names use a
versioned length-prefixed encoding of importing package ID, resolution context
ID (when available), and import path. The frozen node hash algorithm remains
unchanged; new boundary-name golden vectors must precede implementation output.
Boundary evidence retains the source proof/binding rather than a fabricated
target identity.

## Evidence, diagnostics, and bounds

Each source occurrence contributes fixed rule identifiers, source IDs, and
safe locations. Never copy source text, raw upstream messages, or arbitrary
proof Value strings into diagnostics. Output source references must cover every
emitted SourceIdentity. Translate upstream omissions into fixed diagnostics so
an empty graph cannot be mistaken for complete knowledge. Diagnostic messages
are constant per code; code/graph/location/stable ID determine deduplication.

Proposed raw-fact budgets apply before allocating adapter indexes: total
records, total evidence records, and normalized unique nodes. Exceeding a raw
budget fails `limit_exceeded`; it does not discard records by input order.
Defaults/ceilings are specified in the candidate API. Use bounded initial map
capacities and do not reserve capacity from an unvalidated raw length. Count
evidence before passing candidates to the core. Published caps and omission
accounting remain owned by the core.

`DIE-HARDEN-001` remains open: this adapter budget limits the volume it supplies
but does not fix all direct callers of the accepted core. `DIE-HARDEN-002`
also remains open for direct core callers. The adapter explicitly rejects nil
contexts at its own boundary. Neither requirement is silently marked resolved.

Check context before each accessor, every 1,024 translated records, before
normalization, and before returning. Released defensive-copy accessors and the
existing core sort may form longer uninterruptible units; measure and report
these separately. This design does not claim a new hard wall-clock bound for
those existing operations. No mutable state is reused across calls.

## Implementation and acceptance gates

After explicit design approval, implement the standard eight-file package
layout under `backend/die/golang`. Do not import the experiment module or copy
its proof shortcuts into production. Tests may construct fixtures using real
released engines in temporary directories; this is test setup only.

Conformance precedes adapter tests, followed by regression, cross-platform
determinism, race, fuzz, and memory evidence. Any required change to the core
contract must be reported as a candidate refinement with targeted evidence.
Completion of implementation requests Phase 5.0.3 acceptance only; it grants
no authority for Phase 5.0.4 or release.
