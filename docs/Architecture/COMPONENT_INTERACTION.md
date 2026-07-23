# Component Interaction

## Released interaction contract

Released engines publish immutable typed artifacts through `ArtifactStore`.
Each engine consumes only the lowest-level prerequisite containing the facts it
needs. Go semantic resolution consumes the frozen snapshot, syntax inventory,
and package-identity inventory.

```text
Repository -> RIE artifacts 1.0.0
RIE artifacts -> GoLanguageInventory 1.0.0
Snapshot + GoLanguageInventory -> GoPackageIdentityInventory 1.0.0
Snapshot + GoLanguageInventory + PackageIdentity -> GoSemanticInventory 1.0.0
```

## Proposed persistence interaction

Application orchestration serializes a completed artifact through its public
contract and submits an envelope plus exact bytes to a storage-neutral
persistence port. The PostgreSQL adapter implements that port later.

Persistence never calls engines and engines never call persistence.

## Future interaction rules

- Repository intelligence owns repository facts.
- The reasoning engine consumes facts and records hypotheses separately.
- The validation engine owns command execution and results.
- The model adapter never becomes the system of record.
- Every conclusion carries its evidence references.
- Exact stored artifact payloads remain authoritative; query indexes are
  rebuildable projections.
