# GoSemanticInventory 1.0.0 Benchmark Summary

## Reference Environment

- Windows 11 Home Single Language, amd64
- 12th Gen Intel Core i5-12450H, 12 logical processors
- 7.7 GiB visible memory
- Go 1.26.2
- Maximum eight semantic workers

## Proposed Release Gates

| Gate | Target | Observed | Result |
|---|---:|---:|---:|
| Package identity, 10,000 proofs | < 1 s | 55.0–58.3 ms | PASS |
| Full semantic candidate, 1,000 files | < 1 s | 51–126 ms | PASS |
| OpenTelemetry, 1,216 files, semantic | < 30 s | 14.14 s stabilized | PASS |
| OpenTelemetry peak heap | < 1.5 GiB | 1,059.1 MiB | PASS |
| Kubernetes, 17,521 files, semantic | < 300 s | 116.9 s and 283.0 s | PASS |
| Kubernetes peak heap | < 4.5 GiB | 3.88 GiB and 3.92 GiB | PASS |
| Cancellation on pinned OpenTelemetry | < 1 s | 471.28 ms | PASS |

Kubernetes uses the documented default one-million relationship ceiling. Both
stabilization runs reproduced SHA-256
`0ee39ff75da62a68e0e674e8cd758b8d26b59f69fa690cd65af4f6cba50f6ce0`
and exactly 7,061,102 omitted relationships.

## Allocation Comparison

| Repository | Accepted peak | Stabilized peak | Change |
|---|---:|---:|---:|
| OpenTelemetry Go | 1,384.6 MiB | 1,059.1 MiB | -23.5% |
| Kubernetes pass B | 5,250.7 MiB | 3,884.9 MiB | -26.0% |
| Kubernetes pass C | 5,080.3 MiB | 3,918.0 MiB | -22.9% |

Kubernetes allocation traffic fell from approximately 29,090 MiB to 28,466
MiB. The optimization removes repeated prerequisite clones, releases
consolidated candidate buffers earlier, discards verified source bytes after
bounded type checking, and transfers private result ownership into the
immutable artifact without an extra construction copy.

## Methodology Limitation

Windows cache/filter-driver behavior varied substantially. One filesystem-cold
stabilization pass spent 319 seconds in the prerequisite pipeline while a
warmer pass spent 44 seconds; semantic time and peak heap are reported
separately. Cold-cache filesystem time is an observation, not a release gate.

A full Kubernetes allocation profile was attempted but profiling overhead
raised process commitment above 11 GiB and caused paging. The isolated test was
stopped safely and produced no partial evidence file. Targeted 1,000-file CPU
and allocation profiles plus unprofiled real-repository peak sampling were used
instead.
