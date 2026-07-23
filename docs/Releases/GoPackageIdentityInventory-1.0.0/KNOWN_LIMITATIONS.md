# GoPackageIdentityInventory 1.0.0 Known Limitations

- Standard-library identity is not guessed from import-path shape; an approved
  exact-version standard-library index is deferred.
- External dependencies are not downloaded or loaded from the module cache.
- Full minimal-version selection is not reproduced.
- Ambient `GOWORK`, `GOFLAGS`, `GOROOT`, and toolchain state are not hidden
  inputs.
- Compiler build-context selection is outside the identity artifact.
- Execution is a full deterministic rebuild; incremental invalidation is
  deferred.

These limitations produce explicit external, unresolved, ambiguous, or stale
states rather than false local proofs.
