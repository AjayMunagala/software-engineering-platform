# Phase 4.0.7 Real-Repository Validation Harness

This directory contains validation-only operator wrappers for the accepted
Phase 4.0.7 design. It is not a product command, transport, source fetcher, or
alternate Repository Service implementation.

## Safety boundary

- Repositories must be fetched by the operator at the approved revision.
- The harness rejects dirty or revision-mismatched trees.
- It never runs repository-owned commands, package managers, builds, or tests.
- PostgreSQL is disposable and initialized only through accepted migrations.
- Results and tracked-file manifests must be outside the analyzed repository.
- Phase 4.0.8 remains unauthorized.

## One disposable case

Run from Ubuntu as the `postgres` operating-system user. Paths passed to the
script use WSL syntax even when the Go process runs on Windows.

```bash
bash validate_disposable.sh \
  --platform windows \
  --root /mnt/d/validation/repositories/fzf \
  --case go-cli \
  --commit 235a726fae89bec3ac6d3e7facd2716d78bb625d \
  --pass windows-worker8-a \
  --workers 8 \
  --output /mnt/d/validation/results/go-cli-windows-a.json
```

Use `--platform linux` with `AEGIS_GO_BINARY` pointing to checksum-verified Go
1.26.2. Add `--crash-recovery` to a representative successful case to perform
an immediate PostgreSQL stop, restart, and exact publication/export proof.

The script records an adjacent `.outcome.json` with exactly one classification:

- `success`;
- `memory_ceiling`;
- `timeout`;
- `correctness_failure`;
- `environment_failure`.

## Comparison

```powershell
.\compare_results.ps1 -Paths @(
  'D:\validation\results\go-cli-windows-a.json',
  'D:\validation\results\go-cli-linux-a.json'
)
```

The comparison uses `normalized_sha256`. Scan-bound public and physical
artifact IDs are validated in each run but are intentionally not compared
between distinct Scan IDs. Exact artifact bytes, digests, metadata,
dependencies, selected facts, and publication shape are compared.

`validate_matrix.sh` runs the required two clean Windows passes, Windows
one-worker pass, and Ubuntu eight-worker pass for one pinned fixture.
