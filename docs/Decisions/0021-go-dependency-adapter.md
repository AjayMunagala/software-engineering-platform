# ADR 0021 - Go Dependency Translation from Released Artifacts

## Status

Proposed, 2026-09-08. Engineering approved the design and authorized Phase
5.0.3 implementation. This ADR remains Proposed until implementation evidence
is accepted. Every later milestone remains unauthorized. ADR 0020 is Accepted.

Implementation evidence is now submitted in
`docs/Validation/GO_DEPENDENCY_ADAPTER_VALIDATION_REPORT.md`. This submission
does not promote this ADR or authorize Phase 5.0.4.

## Context

The accepted neutral core normalizes generic graph candidates. Go import and
reference interpretation belongs in a separate adapter. The isolated spike
proved feasibility but did not define production proof validation, limits, or
complete handling of partial upstream facts.

## Proposed decision

1. Introduce `backend/die/golang` after approval. Consume four released 1.0.0
   artifacts through public immutable accessors and call the neutral core once.
2. Validate stored metadata, identity joins, snapshot membership, and available
   file digests. Document that name/version references cannot prove same-scan
   provenance or current filesystem freshness.
3. Use exact package-identity proof and semantic binding agreement for local
   import edges. Preserve missing, ambiguous, external, and stale knowledge.
   Partial resolution is not automatically stale.
4. Derive module ownership from unique declared roots and exact segment-aware
   ancestry. Do not redo workspace, replacement, vendor, or stdlib resolution.
5. Emit reference-based file edges only for exact verified declaration targets.
   Import-to-package boundaries never imply an arbitrary target file.
6. Enforce raw record/evidence budgets and nil-context checks at the adapter
   boundary. Keep both recorded direct-core hardening requirements open.
7. Leave SCC, cycle, impact, platform integration, and release gated.

## Alternatives

Adding Go rules to `die` would reverse the dependency direction. Re-parsing
source or manifests would duplicate released ownership. Copying the experiment
unchanged would inherit incomplete proof validation and the incorrect shortcut
that every partial semantic result implies staleness. All are rejected.

## Consequences

The adapter provides structural dependency facts limited by upstream evidence.
Unavailable external-module identity remains a package boundary, and a coherent
input set remains the caller's responsibility. The design adds no released
artifact fields, storage contract, runtime dependency, or algorithm milestone.

## Acceptance gate

Review the architecture, candidate API, and validation plan together. An
explicit engineering decision must authorize implementation. Updating this
document alone does not grant that authority. Implementation evidence must
then pass its own review before Phase 5.0.3 is accepted.
