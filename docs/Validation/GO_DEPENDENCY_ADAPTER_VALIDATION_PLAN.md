# Phase 5.0.3 Go Dependency Adapter Validation Plan

Approved design plan, 2026-09-08. Phase 5.0.3 implementation and validation
were explicitly authorized. Execution evidence is recorded separately in
`GO_DEPENDENCY_ADAPTER_VALIDATION_REPORT.md`; implementation acceptance is pending.

## Order and controlled fixtures

First run accepted `die/conformance`, then adapter contract tests, then fixtures
constructed by the released engines. Fixtures cover empty input, isolated
packages, multiple modules, nested roots, workspace/replace/vendor proofs,
stdlib/external imports, all alias forms, missing bindings, ambiguous proofs,
partial/stale records, exact cross-file references, and Unicode paths.

| Gate | Required assertion |
|---|---|
| Input metadata | Reject zero artifacts even when name/version methods return constants |
| Provenance joins | Reject conflicting IDs, mismatched importing package/path/target, unexplained dangling joins, and contradictory digests |
| Evidence limitations | Do not claim same-scan or filesystem freshness from name/version references |
| Ownership | Deepest unique segment ancestor; nested module boundaries; equal-root conflicts; packages with no owner |
| Import counting | One occurrence per syntax import, even when binding/proof evidence also exists; all alias forms retained |
| Local resolution | Exact matching proof and target required; unresolved candidates never become local edges |
| Resolution mapping | Partial without stale evidence stays unresolved; explicit stale stays stale; stdlib requires proof |
| File graph | Exact resolved reference/declaration target; no guessed import target file; same-file references excluded |
| Missing upstream facts | Fixed omission/missing-binding diagnostics, no invented facts or raw upstream messages |
| Source references | Every emitted evidence identity refers to one supplied artifact |
| Identity | Existing frozen vectors plus proposed boundary-name vectors; ambiguous boundary contexts stay separate |
| Immutability | Input artifact bytes/accessor values unchanged; output accessor mutation cannot affect inventory |
| Hard limits | Raw records/evidence and unique nodes fail with no artifact; no input-order truncation |
| Cancellation | Nil, pre-canceled, deadline, translation checkpoints, and cancellation before return |
| Scope | Components/cycles remain empty; no algorithm, integration, source-read, or execution capability |

## Determinism and quality

Compare complete output bytes and IDs across shuffled fixture facts, three
repeated processes, Windows and Ubuntu. Translator is serial; upstream fixture
generation at one/eight workers must still yield equal outputs where the
released upstream contract promises equality. Preserve occurrence, diagnostic,
and omission equality. Conformance, unit, fuzz, vet, targeted/full backend race,
and regression must pass; candidate package coverage target is at least 85%.

Fuzz proof joins, safe paths, boundary identity segments, input budgets, and
status combinations. Test errors for accidental source-path/message leakage.
Production dependency audit rejects filesystem traversal, process execution,
network, database, transport, runtime, and Repository Service imports. Released
engine execution and temporary source creation belong only to fixture setup.

## Measurements and hardening

Record commit, Go/Git versions, OS, CPU, available RAM, fixture counts/digests,
elapsed time, allocations, sampled peak heap, diagnostics, and omissions.
Exclude fixture engine execution from adapter timing. Measure adapter input
copies/indexes/translation and core normalization separately where possible.
Start with 100, 1,000, and 10,000 import/reference fixtures plus a duplicate-heavy
evidence case; freeze any adapter-specific performance gate from evidence and
engineering review rather than claiming the neutral synthetic gate covers it.

Rerun the existing 100k-node/1m-edge core gate if core behavior changes; its
30-second/two-GiB criterion is unchanged. Document clone/sort cancellation units
that cannot be interrupted through released accessors. A failing resource gate
must identify the limit, not masquerade as a correctness pass.

`DIE-HARDEN-001` and `DIE-HARDEN-002` remain required release items. Show the
adapter budget and nil-context guard work, but do not mark direct-core issues
closed without their separate tests and accepted implementation changes.

## Deliverable and exit

After authorized implementation, publish a validation report with commands,
results, defects, limitations, and commit URL. Request Phase 5.0.3 acceptance.
No version promotion, release tag, or Phase 5.0.4 implementation follows
automatically. This plan is not itself an engineering acceptance decision.
