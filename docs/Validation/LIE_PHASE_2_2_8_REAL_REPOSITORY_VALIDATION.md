# LIE Phase 2.2.8 Validation — Real Repositories

## Status

- Date: 2026-07-23
- Candidate implementation: `ef6d0f3`
- Validation fixes: `830eef7`, `b8bda32`
- Artifact: `GoSemanticInventory 0.1.0`
- Validation: complete
- Local exit evidence: collected
- Governance: accepted on 2026-07-23
- Phase 2.2.9: authorized

## Reference Environment

- OS: Windows 11 Home Single Language 10.0.26200, build 26200
- Architecture: amd64
- CPU: 12th Gen Intel Core i5-12450H, 12 logical processors
- Visible memory: 7.7 GiB
- Go: 1.26.2
- Semantic workers: 8 (except explicit one-worker determinism passes)
- Race toolchain: MSYS2 UCRT64 GCC 16.1.0 with `CGO_ENABLED=1`

Repositories were fetched at exact commits into
`D:\Project_Ai_validation_228`, outside the project working tree. The engine
performed no repository command execution, dependency download, module-cache
read, network access, or source write. Network access occurred only before
analysis, when the validation operator fetched the pinned repositories.

## Repository Matrix

| Category | Repository | Pinned commit |
|---|---|---|
| Small CLI | `junegunn/fzf` | `235a726fae89bec3ac6d3e7facd2716d78bb625d` |
| Medium service | `pocketbase/pocketbase` | `cc4e85709074c8a81284c3d9c5064d2adbf4c854` |
| Generics-heavy library | `samber/lo` | `bb6e45518419e49b59a20a55de900df7698ad644` |
| Kubernetes-style multi-module | `kubernetes-sigs/kustomize` | `f7d33c2610d868318bdd4442b32a8616e3b66eb8` |
| Large multi-module / unavailable dependencies | `open-telemetry/opentelemetry-go` | `d98b5f42f2550ba2b851d9d95f4ddb6c4f98c4b0` |
| Large Kubernetes repository | `kubernetes/kubernetes` | `1056cbb4511a20ae5e4d0173813cd3403c6d836a` |
| Malformed source | controlled local fixture | not applicable |
| Stale source | controlled local fixture | not applicable |

## File and Declaration Results

Every valid real-repository Go source was parsed. Semantic files remain
`partial` because external importing, compiler build selection, and later graph
layers are deliberately unavailable; `partial` is an honest capability state,
not a parse failure.

| Repository | Candidate | Partial | Failed | Stale | Skipped | Resolved declarations | Ambiguous declarations |
|---|---:|---:|---:|---:|---:|---:|---:|
| fzf | 81 | 81 | 0 | 0 | 0 | 8,134 | 72 |
| PocketBase | 453 | 453 | 0 | 0 | 0 | 26,167 | 6 |
| lo | 109 | 109 | 0 | 0 | 0 | 23,663 | 8 |
| Kustomize | 770 | 770 | 0 | 0 | 0 | 25,459 | 2 |
| OpenTelemetry Go | 1,216 | 1,216 | 0 | 0 | 0 | 179,365 | 16 |
| Kubernetes | 17,521 | 17,521 | 0 | 0 | 0 | 1,176,568 | 73,391 |
| broken-go | 2 | 1 | 1 | 0 | 0 | 2 | 0 |
| stale-go | 1 | 0 | 0 | 1 | 0 | 0 | 0 |

Ambiguous declarations in real repositories are dominated by mutually
exclusive GOOS, GOARCH, Go-version, and generated/build-tag alternatives. The
candidate intentionally does not select a compiler build context.

## Reference and Import Results

| Repository | References resolved | External | Unresolved | Ambiguous | Imports resolved | External | Unresolved |
|---|---:|---:|---:|---:|---:|---:|---:|
| fzf | 31,140 | 7,349 | 13,171 | 240 | 46 | 295 | 0 |
| PocketBase | 69,044 | 24,947 | 65,078 | 26 | 788 | 2,112 | 0 |
| lo | 52,548 | 21,329 | 35,693 | 31 | 51 | 278 | 0 |
| Kustomize | 67,451 | 18,492 | 53,429 | 48 | 1,650 | 1,910 | 26 |
| OpenTelemetry Go | 356,825 | 89,878 | 227,945 | 8,095 | 1,881 | 3,194 | 455 |
| Kubernetes emitted prefix | 28,325 | 9,586 | 27,881 | 232 | 35,392 | 37,229 | 34,558 |

Kubernetes reached the configured one-million total relationship ceiling.
Imports, receivers, and type relations retain priority, leaving 66,024 emitted
references; 7,061,102 unique relationships were deterministically omitted and
reported. Identical clean-process runs produced the same omission count and
artifact hash.

## Package Identity Evidence

| Repository | Resolved proofs | External | Unresolved | Proof conflicts | Notable proof kinds |
|---|---:|---:|---:|---:|---|
| fzf | 7 | 98 | 0 | 0 | same-module |
| PocketBase | 226 | 772 | 0 | 0 | same-module |
| lo | 14 | 67 | 0 | 0 | same-module, local-replace |
| Kustomize | 1,864 | 2,330 | 0 | 0 | same-module, local-replace, workspace-module |
| OpenTelemetry Go | 1,123 | 1,904 | 421 | 0 | same-module, local-replace |
| Kubernetes | 110,905 | 97,488 | 0 | 0 | same-module, local-replace, workspace-module, vendor |

No proof was marked stale in a real checkout. OpenTelemetry's unavailable
external dependencies remained external/unresolved without importer, tool, or
network fallback. Kubernetes produced 16 bounded identity diagnostics and no
proof conflict.

## Relationship and Diagnostic Results

| Repository | Receivers | Type relations | Interface checks | Relationships omitted | Diagnostics published | Diagnostics omitted |
|---|---:|---:|---:|---:|---:|---:|
| fzf | 503 | 4,240 | 4 | 0 | 41 | 0 |
| PocketBase | 1,326 | 15,301 | 111 | 0 | 3 | 0 |
| lo | 43 | 18,414 | 1 | 0 | 4 | 0 |
| Kustomize | 1,465 | 12,885 | 43 | 0 | 1 | 0 |
| OpenTelemetry Go | 26,001 | 134,981 | 236 | 0 | 6 | 0 |
| Kubernetes | 88,706 | 738,091 | 0 | 7,061,102 | 1,000 | 8,513 |

All emitted interface results in this matrix are `unknown` because their
candidate packages contain unavailable imports or incomplete build-context
evidence. The engine did not convert missing evidence into a false proof. On
Kubernetes, foundational relationships consume the configured limit before
interface checks, so zero interface checks are emitted and the omission is
explicit.

Kubernetes diagnostic output deterministically reserves the relationship-limit
and diagnostic-limit aggregates. Its ordinary diagnostics are primarily
build-context package conflicts and receiver ambiguity derived from those
conflicts.

## Determinism

- fzf, PocketBase, lo, Kustomize, and OpenTelemetry produced byte-equivalent
  JSON/artifact hashes across repeated full rebuilds and one-vs-eight workers.
- Kubernetes was run twice in separate clean Go processes because retaining
  two large runtime heaps in one 7.7-GiB workstation process causes severe
  paging. Both runs produced SHA-256
  `0ee39ff75da62a68e0e674e8cd758b8d26b59f69fa690cd65af4f6cba50f6ce0`,
  identical statistics, 7,061,102 omitted relationships, and 1,000 published
  diagnostics.
- Broken and stale fixtures were deterministic across repeated and
  one-vs-eight-worker execution.

No nondeterminism was observed.

## Timing and Memory

The `pipeline` column includes RIE, Phase 2.1 parsing, and package identity.
The semantic columns measure full semantic candidate rebuilds. Filesystem cache
eviction was not controlled on Windows; therefore pipeline values are recorded
as filesystem-inclusive observations, not falsely labeled as reproducible cold
cache results.

| Repository | Pipeline | Semantic first | Semantic repeat | Peak heap | Allocated traffic | Allocations |
|---|---:|---:|---:|---:|---:|---:|
| fzf | 3.01 s | 254.87 ms | 194.77 ms | 85.8 MiB | 225.0 MiB | 1,348,550 |
| PocketBase | 11.98 s | 623.00 ms | 497.21 ms | 268.7 MiB | 722.6 MiB | 4,320,451 |
| lo | 4.54 s | 593.11 ms | 380.30 ms | 160.6 MiB | 558.0 MiB | 3,493,051 |
| Kustomize | 15.11 s | 880.47 ms | 764.87 ms | 249.3 MiB | 710.3 MiB | 4,139,249 |
| OpenTelemetry Go | 14.05 s | 4.33 s | 8.84 s | 1,384.6 MiB | 4,118.4 MiB | 24,135,403 |
| Kubernetes pass B | 10.88 s | 121.40 s | separate process | 5,250.7 MiB | 29,092.8 MiB | 172,229,769 |
| Kubernetes pass C | cached | 129.49 s | separate process | 5,080.3 MiB | 29,089.4 MiB | 172,235,198 |

The first Kubernetes attempts exposed late reference limiting: the process
committed more than 12 GiB and was stopped before the operating system became
unstable. After deterministic batched candidate limiting, a complete rebuild
peaks at approximately 5.1–5.3 GiB heap. This is a major improvement and allows
the reference workstation to complete, but Kubernetes-scale repeated-run heap
reuse and profiling remain explicit Phase 2.2.9 stabilization work.

## Cancellation

- OpenTelemetry cancellation requested 10 ms after start was observed in
  270.97 ms on the final candidate.
- Kustomize previously observed the same real-run cancellation scenario in
  25.13 ms.
- Unit/spike gates continue to verify checks before/after file work and package
  type checking, every 1,024 traversal nodes, and every 256 relationships.
- `go/parser` and `go/types` synchronous calls remain the documented upper bound
  on cooperative latency. Phase 2.2.9 must approve the final wall-clock target.

## Controlled Failure Evidence

- `broken-go`: one malformed file becomes `failed` with
  `semantic_prerequisite_file_failed`; one valid neighboring file remains
  partial and contributes two resolved declarations.
- `stale-go`: the harness changes one file after Phase 2.1 publication. The
  semantic result contains exactly one stale file and
  `semantic_digest_mismatch`, with no stale declarations or relationships.
- No repository escape, source mutation by the engine, crash, panic, or data
  race occurred.

## Defects Found and Corrected

### 1. Package blank identifiers were not reconciled

- Found in: PocketBase compile-time interface assertions.
- Symptom: 138 false `semantic_syntax_symbol_unmatched` diagnostics for `_`.
- Root cause: semantic collection skipped blank identifiers even though frozen
  Phase 2.1 intentionally records those top-level declaration sites.
- Fix: reconcile `_` as evidence without inserting it into lexical or package
  scopes.
- Commit: `830eef7`.
- Result: zero unmatched blank-identifier diagnostics after revalidation.

### 2. Relationship limits were applied after full reference allocation

- Found in: Kubernetes.
- Severity: high scalability defect.
- Symptom: more than 12 GiB process commitment and severe paging before final
  truncation.
- Root cause: every resolved reference object was retained before applying
  `MaxRelationships`.
- Fix: deterministically sort, compact, and bound unique reference candidates
  in path-ordered batches; bind only the publishable prefix while preserving
  exact omission counts and priority.
- Commit: `b8bda32`.
- Result: complete Kubernetes runs at approximately 5.1–5.3 GiB peak heap with
  stable hashes and exact omission reporting.

### 3. Struct/interface aliases did not reconcile Phase 2.1 symbols

- Found in: Kubernetes `type RunData = interface{}` and three similar aliases.
- Symptom: four false unmatched syntax-symbol diagnostics.
- Root cause: Phase 2.1 classifies the underlying struct/interface syntax while
  semantic collection correctly models the declaration as an alias but did not
  connect the two artifacts.
- Fix: retain `DeclarationTypeAlias` while reconciling the frozen underlying
  struct/interface syntax symbol.
- Commit: `b8bda32`.
- Result: zero unmatched alias diagnostics after revalidation.

Published candidate history was not rewritten.

## Manual Verification Samples

- Inspected PocketBase package-level interface assertions and confirmed blank
  identifiers are artifact evidence but never scope bindings.
- Inspected Kubernetes `RunData = interface{}` and confirmed alias kind plus the
  frozen interface syntax reference.
- Inspected fzf and Kubernetes GOOS/GOARCH alternatives and confirmed ambiguity
  is explicit instead of selecting a build context.
- Verified Kustomize workspace/local-replace proof counts and zero proof
  conflicts.
- Verified malformed and stale fixtures against their exact source files and
  diagnostics.

## Local Quality Gates

| Check | Result |
|---|---:|
| Semantic package tests | PASS |
| Ten shuffled semantic runs | PASS |
| Full backend regression | PASS |
| `go vet ./...` | PASS |
| Semantic statement coverage | 85.8% |
| Targeted race test | PASS |
| Full backend race test | PASS |
| Data races | 0 |

## Known Limitations for Stabilization Review

- No compiler build-context selection; mutually exclusive files remain
  explicitly ambiguous.
- No external importer or standard-library index; missing evidence remains
  external/unresolved/unknown.
- Kubernetes reaches the default one-million relationship limit, so later
  relationship categories may be entirely omitted.
- Kubernetes-scale peak heap is approximately 5.1–5.3 GiB on this candidate.
- The Go runtime retains substantial address space after a very large rebuild;
  repeated same-process enterprise scans require Phase 2.2.9 profiling.
- Windows filesystem cache could not be evicted reproducibly; a controlled
  cold-cache CI measurement remains unavailable.
- A final cancellation wall-clock target remains a Phase 2.2.9 decision.

## Exit-Gate Assessment

Phase 2.2.8 has completed its authorized validation work. The matrix contains
all required repository categories, every discovered correctness defect has a
small traceable fix and successful revalidation, deterministic output is
demonstrated, explicit failure states behave correctly, and local quality gates
pass.

Engineering accepted the evidence on 2026-07-23 and authorized Phase 2.2.9.
Stabilization must explicitly review Kubernetes memory, the performance gate,
cold-cache methodology, cancellation target, and the impact of
relationship-priority exhaustion before freezing `1.0.0`.
