# Benchmark Summary

## Approved release gate

> Analyze 10,000 Go files in under 30 seconds with a warm operating-system file
> cache on the reference CI runner, using at most eight workers; record cold-cache
> filesystem time separately.

## Release measurement

| Workload | Timed result | Allocated bytes | Result |
|---|---:|---:|---|
| 10 deterministic Go files | 4.326 ms | 217,760 B | Pass |
| 1,000 deterministic Go files | 48.037 ms | 7,508,488 B | Pass |
| 10,000 deterministic Go files | 0.479 s | 75,465,128 B | Pass |
| Empty LIE runner | 800 ns | 0 | Pass |

The Go benchmark warms only the operating-system file cache before timing. File
reads, SHA-256 hashing, parsing, fact collection, sorting, defensive copying, and
artifact construction remain inside the timed operation.

## Real-repository context

Kubernetes contained 17,521 Go files. Warm repeat passes ranged from 6.36 to
50.42 seconds on the Windows validation machine, while newly-created checkout
passes were slower and highly variable due to filesystem/filter-driver activity.
Those cold-cache measurements remain recorded operational evidence, not the
reproducible engine release gate.
