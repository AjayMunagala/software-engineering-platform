# RIE 1.0.0 Real-World Validation Report

**Validation date:** 2026-07-20
**RIE release:** `v1.0.0` (`91d68db`)
**Environment:** Windows/amd64, Go 1.26.2, Intel Core i5-12450H

## Decision

RIE 1.0.0 passed real-world validation. No correctness or stability defect requiring a patch release was confirmed, so `v1.0.1` is not created.

## Method

- Cloned 15 public repositories outside the project workspace with `git clone --depth 1`.
- Recorded the exact commit tested for every repository.
- Compared RIE file totals with `git ls-files` on each clean checkout.
- Independently counted every extension supported by Language Engine and compared the counts with `LanguageInventory`.
- Verified every framework and build/package detection had non-empty rule, value, and location-aware evidence pointing to a real file.
- Compared reported Git branches with Git directly.
- Ran every repository twice and compared all functional report sections, excluding scan IDs, timestamps, duration, and throughput.
- Ran five performance samples per repository and recorded the median pipeline duration.

## Repository results

| Repository | Commit | Files | Languages reported | Frameworks | Build/package facts | Median | Result |
|---|---:|---:|---|---|---|---:|---|
| [spf13/cobra](https://github.com/spf13/cobra) | `adbc8813901b` | 66 | Go | None | Go Modules, Go Toolchain | 9.878 ms | Pass |
| [gin-gonic/gin](https://github.com/gin-gonic/gin) | `34dac209ffb6` | 130 | Go | Gin | Go Modules, Go Toolchain | 7.969 ms | Pass |
| [go-chi/chi](https://github.com/go-chi/chi) | `8b258c7bb28f` | 99 | Go | Chi | Three location-aware Go modules | 7.575 ms | Pass |
| [sqlc-dev/sqlc](https://github.com/sqlc-dev/sqlc) | `e209d865845b` | 6,761 | Go, SQL, Python | None | Go Modules, pip, Go Toolchain | 1,815.209 ms | Pass |
| [amacneil/dbmate](https://github.com/amacneil/dbmate) | `2183dd8c631a` | 88 | Go, TypeScript, SQL | None | Go Modules, npm, Go Toolchain | 22.855 ms | Pass |
| [sindresorhus/p-limit](https://github.com/sindresorhus/p-limit) | `42599ebbbb12` | 16 | JavaScript, TypeScript | None | None declared with sufficient evidence | 4.690 ms | Pass |
| [reduxjs/redux](https://github.com/reduxjs/redux) | `5d65348e2663` | 476 | JavaScript, TypeScript | Express, React, Redux | npm and Yarn at preserved locations | 109.265 ms | Pass |
| [changesets/changesets](https://github.com/changesets/changesets) | `934950382e96` | 376 | TypeScript, JavaScript | None | pnpm, two workspaces | 47.226 ms | Pass |
| [expressjs/express](https://github.com/expressjs/express) | `ae6dd37680e3` | 213 | JavaScript | None | None declared with sufficient evidence | 26.951 ms | Pass |
| [pallets/click](https://github.com/pallets/click) | `333c28d79cd9` | 164 | Python | None | uv and flit build backends | 16.544 ms | Pass |
| [google/gson](https://github.com/google/gson) | `c9f3fd55854a` | 311 | Java | None | Maven multi-module workspace | 149.936 ms | Pass |
| [sharkdp/fd](https://github.com/sharkdp/fd) | `1bfeea237a48` | 59 | None; Rust is outside the frozen language set | None | Cargo | 4.530 ms | Pass with scope note |
| [slimphp/Slim](https://github.com/slimphp/Slim) | `80900fb39caf` | 145 | None; PHP is outside the frozen language set | None | Composer | 10.902 ms | Pass with scope note |
| [ardalis/SmartEnum](https://github.com/ardalis/SmartEnum) | `9bc3f7a43055` | 220 | C# | None | .NET project detection is not implemented | 281.567 ms | Pass with scope note |
| [gothinkster/golang-gin-realworld-example-app](https://github.com/gothinkster/golang-gin-realworld-example-app) | `626c372d2594` | 39 | Go | Gin | Go Modules, Go Toolchain | 5.339 ms | Pass |

## Gate results

| Gate | Result |
|---|---|
| All scans completed | Pass: 15/15 |
| File totals | Pass: exact match with tracked files in 15/15 clean checkouts |
| Supported-language counts | Pass: exact per-language match in 15/15 |
| Git presence and current branch | Pass: 15/15 |
| Evidence completeness and paths | Pass: every emitted framework/build item |
| Functional determinism | Pass: identical functional output on repeated scans, 15/15 |
| Errors | Pass: zero across all repositories |
| Performance | Pass: largest checkout contained 6,761 files and completed in a 1.815-second median |

## Warnings reviewed

- Cobra produced four warnings for Git ignore character classes.
- SmartEnum produced sixteen warnings for Git ignore character classes.
- These warnings are intentional: RIE 1.0 documents character classes as unsupported and skips them visibly instead of guessing.
- Every other tested repository completed with zero warnings.

## Scope findings

The following are real coverage gaps, but they do not violate the frozen RIE 1.0 contract:

1. Rust `.rs` and PHP `.php` files remain unknown even though Cargo and Composer are detected.
2. Git ignore character classes such as `[Dd]ebug` are warned about and skipped.
3. `.csproj`, `.sln`, and NuGet build/package facts are not detected.
4. Example and fixture projects with independent manifests contribute to deterministic monorepo classification; RIE does not yet classify project roles.

These findings are recorded in the technical debt register. They are candidates for additive RIE 1.x improvements only when separately approved.

## Environment note

An attempted Spectre.Console checkout was excluded because Windows rejected generated paths longer than the local checkout limit. It was replaced by SmartEnum. This occurred before RIE scanned the repository and is not an RIE failure.

## Final outcome

RIE remains frozen at `v1.0.0`. Phase 1.1 validation and hardening is complete. No LIE code or design was added during this work.
