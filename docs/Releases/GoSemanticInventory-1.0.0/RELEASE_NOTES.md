# GoSemanticInventory 1.0.0 Release Notes

## Status

Released on 2026-07-23 under the annotated tag
`go-semantic-inventory/v1.0.0`. Its stable Package Identity prerequisite was
released first under `go-package-identity-inventory/v1.0.0`.

`GoSemanticInventory` is the stable semantic foundation for Go source in the
Software Intelligence Platform. It synthesizes deterministic, bounded facts
from the frozen repository snapshot, syntax inventory, and package-identity
inventory without mutating those artifacts or persisting compiler state.

The release provides declarations and scopes, receiver/type binding,
proof-backed references and imports, conditional interface satisfaction,
explicit unknown states, deterministic diagnostics, and an immutable typed and
JSON contract.

Phase 2.2.8 validated the engine on fzf, PocketBase, lo, Kustomize,
OpenTelemetry Go, Kubernetes, malformed source, and stale source. Phase 2.2.9
reduced large-repository memory while preserving the accepted artifact hashes.

Public API: [GO_SEMANTIC_PUBLIC_API.md](../../API/GO_SEMANTIC_PUBLIC_API.md)

Validation: [LIE_PHASE_2_2_9_STABILIZATION.md](../../Validation/LIE_PHASE_2_2_9_STABILIZATION.md)
