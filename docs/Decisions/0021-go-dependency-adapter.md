# ADR 0021 - Go Dependency Translation from Released Artifacts

## Status

Accepted. Engineering accepted Phase 5.0.3 implementation commit
`a0a1a779734b23a0f6c2a8f711130063822a9761` and its validation evidence,
explicitly authorizing promotion of this ADR from Proposed to Accepted.
The candidate remains 0.1.0, not a production release. Phase 5.0.4, SCC/cycle/
impact algorithms, integration, and release remain unauthorized.

Evidence: `docs/Validation/GO_DEPENDENCY_ADAPTER_VALIDATION_REPORT.md`.
DIE-HARDEN-001 and DIE-HARDEN-002 remain open; the documented memory,
cancellation, provenance, and performance limitations remain accepted limitations.

## Context

The accepted neutral core normalizes generic graph candidates. Go import and
reference interpretation belongs in a separate adapter. The isolated spike
proved feasibility but did not define production proof validation, limits, or
complete handling of partial upstream facts.

## Decision

1. Introduce `backend/die/golang`. Consume four released 1.0.0
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

Satisfied by explicit engineering review of the committed implementation and
validation evidence at `a0a1a779734b23a0f6c2a8f711130063822a9761`.
Boundary vectors were independently frozen first in `7eb2a98`.
This acceptance closes Phase 5.0.3 only. A separate Phase 5.0.4 design milestone
and explicit authorization are required before any later work begins.
