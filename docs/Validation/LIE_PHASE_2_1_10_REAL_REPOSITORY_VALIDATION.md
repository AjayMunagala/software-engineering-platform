# LIE Phase 2.1.10 — Real Repository Validation

## Status

**Approved. `GoLanguageInventory 1.0.0` release gates passed.**

Correctness, containment, deterministic-output, diagnostic, and memory gates pass.
One real parser defect was found, fixed in a separate follow-up commit, and
revalidated. The project owner approved the reference warm-cache performance
gate on 2026-07-20; cold-cache filesystem time remains a separately recorded
operational measurement.

## Release Candidate

- Initial Phase 2.1 RC: `e506d8e` (`Implement Go language intelligence engine`)
- Validation defect fix: `f598c51` (`Fix parenthesized Go method receivers`)
- Artifact validated: `go-language-inventory 1.0.0`
- Platform: Windows/amd64
- CPU: 12th Gen Intel Core i5-12450H
- Go: 1.26.2
- Default workers: 8
- Validation date: 2026-07-20

The RC history was not rewritten. The defect correction is independently
traceable as requested by the release process.

## Repository Matrix

| Category | Repository | Pinned commit | Notes |
|---|---|---|---|
| Compact CLI | [junegunn/fzf](https://github.com/junegunn/fzf) | `235a726fae89bec3ac6d3e7facd2716d78bb625d` | Command-line application |
| Medium web application/service | [pocketbase/pocketbase](https://github.com/pocketbase/pocketbase) | `cc4e85709074c8a81284c3d9c5064d2adbf4c854` | HTTP application and service packages |
| Generics-heavy library | [samber/lo](https://github.com/samber/lo) | `bb6e45518419e49b59a20a55de900df7698ad644` | Generic collection and utility functions |
| Multi-module repository | [open-telemetry/opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) | `d98b5f42f2550ba2b851d9d95f4ddb6c4f98c4b0` | 28 `go.mod` files |
| Large Kubernetes repository | [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes) | `1056cbb4511a20ae5e4d0173813cd3403c6d836a` | 30,865 retained files and 17,521 Go files |
| Controlled malformed repository | Local fixture | Not applicable | Two valid Go files and one intentionally malformed file |

All open-source repositories were shallow-cloned into an isolated temporary
directory. The engine did not execute repository tools, resolve dependencies,
or access the network during analysis.

## Functional Results

| Repository | Files discovered | Go candidates | Parsed | Failed | Skipped | Packages | Imports | Diagnostics | Deterministic |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| fzf | 153 | 81 | 81 | 0 | 0 | 6 | 341 | 0 | Yes |
| pocketbase | 920 | 453 | 453 | 0 | 0 | 71 | 2,900 | 0 | Yes |
| lo | 675 | 109 | 109 | 0 | 0 | 9 | 329 | 0 | Yes |
| opentelemetry-go | 1,555 | 1,216 | 1,216 | 0 | 0 | 434 | 5,530 | 0 | Yes |
| kubernetes | 30,865 | 17,521 | 17,521 | 0 | 0 | 4,176 | 107,179 | 0 after fix | Yes |
| broken-go | 4 | 3 | 2 | 1 | 0 | 1 | 0 | 1 expected parse error | Yes |

Across the five real repositories, all **19,380** Go candidates parsed. There
were no crashes, panics, skipped files, or unexpected parse failures. The
controlled malformed file produced one `go_parse_error`; valid neighboring
files remained available in the artifact.

## Symbol Results

| Repository | Structs | Interfaces | Functions | Methods | Constants | Variables |
|---|---:|---:|---:|---:|---:|---:|
| fzf | 81 | 2 | 415 | 503 | 595 | 106 |
| pocketbase | 315 | 41 | 1,595 | 1,326 | 272 | 257 |
| lo | 30 | 9 | 2,606 | 43 | 19 | 21 |
| opentelemetry-go | 4,126 | 180 | 17,467 | 26,001 | 18,335 | 20,915 |
| kubernetes | 21,909 | 4,210 | 77,188 | 88,706 | 112,300 | 21,971 |
| broken-go | 1 | 0 | 1 | 0 | 0 | 0 |

These are syntax facts. Generated files are intentionally included, and the
engine does not infer semantic validity, type identity, or build selection.

## Timing and Memory

| Repository | First analysis | RIE scan | Retained heap after analysis | Cumulative allocated bytes |
|---|---:|---:|---:|---:|
| fzf | 1.121 s | 0.076 s | 1.83 MiB | 13.18 MiB |
| pocketbase | 6.699 s | 0.274 s | 2.54 MiB | 44.59 MiB |
| lo | 1.749 s | 0.164 s | 2.25 MiB | 28.80 MiB |
| opentelemetry-go | 15.562 s | 0.259 s | 41.22 MiB | 374.86 MiB |
| kubernetes | 103.348 s | 11.932 s | 365.90 MiB | 2,431.80 MiB |
| broken-go | 0.054 s | <0.001 s | 0.09 MiB | 0.09 MiB |

`Cumulative allocated bytes` is allocation traffic, not simultaneous memory
usage. The conservative retained-heap observation for Kubernetes remains below
the 1 GiB target. No out-of-memory event occurred.

### Performance interpretation requiring approval

Windows results for the newly-created Kubernetes checkout were highly variable
because file-open time was dominated by filesystem/filter-driver activity:

- Post-fix first passes observed: 103.35–134.38 seconds for 17,521 Go files.
- Immediate repeat passes observed: 6.36–50.42 seconds for the same content.
- The repeatable, explicitly warm-cache 10,000-file benchmark completed its
  timed engine operation in 0.280 seconds.
- The final 1.0.0 release rerun completed the same gate in 0.479 seconds.
- OpenTelemetry completed 1,216 files in 15.56 seconds on its initial pass.

The parsing algorithm remains O(n), uses at most eight workers, and stays within
the memory target. However, a statement such as “10,000 files under 30 seconds”
must specify cache state and environment to be a reproducible release gate.

Recommended release wording:

> Analyze 10,000 Go files in under 30 seconds with a warm operating-system file
> cache on the reference CI runner, using at most eight workers; record cold-cache
> filesystem time separately.

Until that wording is approved—or the original cold-cache target passes on an
approved CI runner—the performance exit criterion remains **conditional**.

## Issues

### Issue 1 — Parenthesized method receiver reported as unsupported

- Repository: Kubernetes
- Severity: Medium
- Category: Parser / receiver extraction
- Evidence: `pkg/volume/util/hostutil/hostutil_windows.go`, method `GetFileType`
- Syntax: `func (hu *(HostUtil)) GetFileType(...)`
- Original behavior: method retained, but receiver base was empty and
  `go_receiver_unsupported` was emitted.
- Root cause: receiver extraction unwrapped pointer and generic nodes but not
  `ast.ParenExpr` nodes.
- Resolution: recursively unwrap parenthesized, pointer, and generic receiver
  expressions without rendering or guessing source text.
- Regression test: added parenthesized generic pointer receiver coverage.
- Fix commit: `f598c51`
- Status: **Fixed and revalidated.** Kubernetes now produces zero diagnostics.

### Issue 2 — Windows cold-cache performance is not a stable release metric

- Repository: Kubernetes
- Severity: Release-gate decision
- Category: Performance methodology
- Description: newly-created repository files caused large and inconsistent
  file-open costs outside parser CPU work. Immediate repeat results varied
  substantially on the same commit and machine.
- Engine correctness impact: None.
- Status: **Approved on 2026-07-20.** Warm-cache engine performance is the
  reproducible release gate; cold-cache filesystem time is recorded separately.

## Exit Criteria

| Criterion | Result |
|---|---|
| No crashes or panics | Pass |
| No repository boundary regressions | Pass in the security regression suite; no real-repository violations |
| Deterministic output | Pass on all six repositories |
| Critical defects resolved | Pass; one medium defect found and fixed |
| Only documented limitations remain | Pass |
| Memory target | Pass based on retained heap |
| Performance target | Pass under the approved warm-cache/reference-CI definition |

## Release Decision

**Approved for `GoLanguageInventory 1.0.0`.**

The public artifact contract may be frozen and tagged. Phase 2.2 may begin only
after the release commit and namespaced Git tag are published.
