# Repository Service 1.0.0 Benchmark Summary

## Environment

- Intel i5-12450H, 8 cores / 12 logical processors
- Windows 11 10.0.26200 and Ubuntu 24.04 under WSL2
- Go 1.26.2
- three runs per platform

## Results

| Benchmark | Windows | Ubuntu | Allocations |
| --- | ---: | ---: | ---: |
| Public artifact identity | 1.968-2.307 us | 1.036-1.141 us | 6 |
| Register validation | 482.5-556.6 ns | 203.9-256.5 ns | 0 |
| Independent scan execution | 50.36-57.38 us | 33.44-45.80 us | 75 |
| 10,000-entry materialization | 76.08-102.51 ms | 10.92-12.94 ms | ~20,092 / 20,051 |
| Physical artifact ID | 1.552-1.732 us | 890.7-994.0 ns | 11 |
| Ten-artifact manifest | 17.70-21.29 us | 8.811-11.15 us | 39 |
| Ten-artifact integration translation | 36.37-42.34 us | 18.10-20.69 us | 149 |

The integration stages remain far below their 5 ms manifest and 25 ms
translation gates. Materialization uses approximately 2.62 MiB on Windows and
1.56 MiB on Ubuntu and does not create an extra artifact-sized copy. Windows
filesystem synchronization and filter-driver behavior explain the documented
platform variance.

The accepted Phase 4.0.7 corpus measurements, including Kubernetes, remain the
normative end-to-end engine/service resource evidence.
