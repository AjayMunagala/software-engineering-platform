# Phase 5.0.1 Dependency Intelligence Design Spike

This isolated Go module validates the risky assumptions in ADR 0020. It is not
production Dependency Intelligence code and is not imported by `backend/`.

The spike validates canonical stable IDs, deterministic graph normalization,
duplicate aggregation, SCC/cycle analysis, bounded forward/reverse impact,
omission ordering, cancellation, one/eight-worker equality, and synthetic
performance/memory behavior.

The graph core uses only the Go standard library. A spike-only adapter consumes
the released backend artifact types through a local module replacement so the
dependency direction is compile-time validated. Test setup may produce those
artifacts from a disposable repository; the graph runner itself does not read
repositories, parse source, execute commands, use the network, access a
database, or mutate released artifacts.

Run:

```powershell
cd experiments/phase-5.0.1-dependency-spike
go test -count=1 ./...
go test -shuffle=on -count=10 ./...
go vet ./...
go test -run "^$" -bench . -benchtime=1x -benchmem ./...
```
