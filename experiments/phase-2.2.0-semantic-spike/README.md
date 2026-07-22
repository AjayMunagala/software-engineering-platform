# Phase 2.2.0 Semantic Design Spike

This isolated Go module validates assumptions required by ADR 0008 before any
production semantic package is authorized.

It uses only the Go standard library and does not import production packages.
The spike tests:

- partial `go/types` information with a rejecting importer;
- generics, embedded interfaces, and pointer/value method sets;
- deterministic full rebuilds and `go-semantic-id/v1` golden vectors;
- explicit package-identity precedence and ambiguity;
- bounded interface candidates, diagnostics, and cancellation checkpoints;
- absence of command, network, and `go/packages` imports;
- repeatable in-memory performance and allocation baselines.

Run from this directory:

```powershell
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go test -run '^$' -bench . -benchtime=3x -benchmem ./...
```

This is experimental evidence, not a public API or production implementation.

The captured results and governance recommendation are in
[`docs/Validation/LIE_PHASE_2_2_0_DESIGN_SPIKE.md`](../../docs/Validation/LIE_PHASE_2_2_0_DESIGN_SPIKE.md).
