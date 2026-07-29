# Repository Service Real-Repository Validation Report

## Decision state

- Phase: 4.0.7
- Evidence date: 2026-07-29
- Implementation: complete
- Validation status: engineering review required
- Phase 4.0.7 acceptance: **not yet granted**
- Phase 4.0.8: **unauthorized**

The production Repository Service path is correct and deterministic for every
completed case. One release gate remains open: Kubernetes completes twice with
eight workers on Windows, but one-worker Windows and eight-worker Ubuntu runs
exceed this host's safe memory ceilings. The harness cancels those runs instead
of threatening system stability. No correctness failure, panic, data race,
partial publication, or payload-integrity failure was observed.

## Tested revision and environments

The candidate was tested from `main` at design-acceptance revision `79c5125`
plus the validation-only changes in this review commit.

| Property | Windows | Ubuntu |
|---|---|---|
| OS | Windows 11 10.0.26200 | Ubuntu 24.04 on WSL2, kernel 6.18.33.2 |
| CPU | Intel Core i5-12450H, 12 logical CPUs | same host, 12 visible CPUs |
| Visible RAM | 7.72 GiB | 3.7 GiB plus 1 GiB swap |
| Filesystem | NTFS | same exact NTFS bytes through `/mnt/d` |
| Go | 1.26.2 windows/amd64 | checksum-verified 1.26.2 linux/amd64 |
| Git preflight | 2.54.0.windows.1 | same native Git over shared checkout |
| PostgreSQL | 18.4 disposable clusters | 18.4 disposable clusters |
| Atlas | 1.2.3 | 1.2.3 |

The accepted migration checksum was
`2EEFA1D68D1C2C9BF499733B26EB0E6204C4EACD7FCC27F52D1365DB50616327`.
Every measured analysis ran with `GOPROXY=off` and `GOSUMDB=off`; repository
commands, builds, tests, package managers, generators, hooks, and network fetch
were not executed.

## Corpus result

The corpus contains 35,725 unique tracked entries. Successful scans processed
81,170 tracked-file visits across clean processes. Each ordinary case ran as
two Windows eight-worker processes, one Windows one-worker process, and one
Ubuntu eight-worker process.

| Case | Files | Artifacts | Representative scan | Peak heap | Determinism |
|---|---:|---:|---:|---:|---|
| p-limit | 16 | 7 | 0.253 s | 8.8 MiB | repeat, 1/8 worker, OS pass |
| fzf | 153 | 10 | 2.817 s | 75.9 MiB | repeat, 1/8 worker, OS pass |
| PocketBase | 920 | 10 | 4.189 s | 241.8 MiB | repeat, 1/8 worker, OS pass |
| lo | 675 | 10 | 3.562 s | 195.5 MiB | repeat, 1/8 worker, OS pass |
| Kustomize | 1,538 | 10 | 4.153 s | 190.8 MiB | repeat, 1/8 worker, OS pass |
| OpenTelemetry Go | 1,555 | 10 | 19.631 s | 1,132.4 MiB | repeat, 1/8 worker, OS pass |
| malformed Go | 3 | 10 | 0.145 s | 8.9 MiB | repeat, 1/8 worker, OS pass |
| Kubernetes | 30,865 | 10 | 200.095 s | 4,312.3 MiB | repeat pass; resource gates open |

The machine-readable result summary is
`docs/Validation/REPOSITORY_SERVICE_REAL_REPOSITORY_RESULTS.json`. It excludes
source roots, source handles, credentials, runtime URLs, scan-bound IDs, and
payload content.

## Exact determinism

For every fully completed matrix, normalized outcomes were byte-equivalent.
Normalization excludes only scan-bound public/physical IDs, timestamps, and
operational measurements. Artifact bytes, SHA-256 values, versions, codecs,
producer metadata, dependencies, selected statistics, and publication shape
remain included.

| Case | Normalized SHA-256 |
|---|---|
| p-limit | `ab6097257f194bb62c15455b3eeb31544aebd924753463e93ec10a92dbeb4c59` |
| fzf | `7228a8d7038650052a9729dcce9371777fb8582a3a1aab3f06d6db283c76e3dc` |
| PocketBase | `33bdf4d3d0eca19ca70b60825be3ab4dfa1297614705406d44aff841257a0afd` |
| lo | `6475638f15bdf5b35ae37880adb67eb5fa73f6193258fa65b178670739fcc144` |
| Kustomize | `249ca881e7104183cbecc80c777e7c9aa46d4e8fc29a523abf50427244810e74` |
| OpenTelemetry Go | `15f7911a48d88049fc07e7d623a128f8a5adedcf08935ad72d05d0405629780c` |
| malformed Go | `ce1e949472f3c5cdf33be4b823bdf8dda4d2abfb318124383184242231fd475a` |
| Kubernetes, Windows repeat | `a4721ee2a395c3ecff977c871a1e99f498325b8fa336b3ae08f7e531bc194e26` |

## Persistence, runtime, scope, and privacy

Every completed service scan passed:

- idempotent repository registration;
- exactly one atomic scan publication;
- seven or ten released artifacts in deterministic name order;
- public-to-physical artifact ID recomputation;
- same-scan ordered dependency verification;
- streamed exact export size and SHA-256 verification;
- cross-scope not-found isolation;
- runtime ready/admission/drain/shutdown behavior;
- absence of source-root and source-handle canaries in durable/query/export
  views;
- database, WAL, chunk, and connection measurements.

The representative non-Go scan was published, PostgreSQL was stopped with
`pg_ctl -m immediate`, restarted, and re-opened through the released runtime.
The scan, seven envelopes, dependencies, publication digest, and all exact
exports remained valid.

## Controlled malformed, stale, cancellation, and failure evidence

- Malformed source produced deterministic partial/failed facts without panic
  through the complete Repository Service and PostgreSQL path.
- The controlled semantic boundary changed `main.go` after syntax publication.
  Windows and Ubuntu both produced exactly one stale file and one
  `semantic_digest_mismatch`; no stale declaration or relationship was
  published. The semantic artifact SHA-256 matched across operating systems.
- Pre-analysis, waiter, all-interest, external cancellation, failure injection,
  orphan handling, and response-loss publication reconciliation tests passed
  three repeated targeted runs in addition to the full suite.

The stale mutation is intentionally proven at the released semantic boundary.
No validation hook or alternate behavior was added to production Repository
Service code.

## Kubernetes resource finding

| Run | Result | Evidence |
|---|---|---|
| Windows, 8 workers, process A | success | 3m18.386s; 4,325.8 MiB peak heap |
| Windows, 8 workers, process B | success | 3m20.095s; 4,312.3 MiB peak heap; identical digest |
| Windows, 1 worker | `memory_ceiling` | canceled at the 6 GiB Go-system safety ceiling |
| Ubuntu, 8 workers | `memory_ceiling` | canceled at the 3 GiB Go-system safety ceiling |

A validation-only `GOMEMLIMIT=4GiB` retry did not keep the one-worker process
below the same 6 GiB safety ceiling. Raising the ceiling on an 8 GiB host would
risk system stability and was rejected. This is classified as a release-gate
resource limitation, not a correctness defect.

Engineering must either approve evidence from a larger race-capable host or
authorize a bounded memory-stabilization investigation. Until then, worker and
cross-platform Kubernetes determinism remain unproven and Phase 4.0.7 cannot be
marked accepted.

## Quality gates

| Gate | Result |
|---|---|
| `go test ./...` | pass |
| `go test -shuffle=on ./...` | pass |
| `go vet ./...` | pass |
| Windows `go test -race ./...` | pass, zero races |
| Ubuntu `go test -race ./...` | pass, zero races |
| Integration statement coverage | 85.2% |
| New manifest/normalization fuzz executions | 25,308, no panic |
| Targeted cancellation/failure repetitions | pass, count 3 |
| Secret/dependency/listener/network/command/path audits | pass |

Repeatable Windows integration benchmarks:

- physical artifact ID: 524.0-615.9 ns/op;
- canonical ten-artifact manifest: 7.297-8.178 us/op;
- ten-artifact integration translation: 15.329-17.319 us/op.

## Harness defects corrected

No production defect was found. Validation-only defects found and corrected:

1. WSL interpreted NTFS mode and line-ending normalization as source changes;
   native Git now performs exact shared-checkout preflight.
2. Gitlink entries appeared as directories; canonical manifests now consume
   index mode/object proof for submodules and symlinks.
3. Serial NTFS manifest hashing was too slow at Kubernetes scale; bounded I/O
   concurrency preserves ordered manifest construction.
4. The Go test process retained its default ten-minute timeout; the approved
   case timeout is now passed explicitly.
5. Outcome cleanup initially preferred an environment label over an observed
   memory ceiling; terminal classification now gives measured memory/timeout
   evidence priority.

## Engineering decision requested

The validation harness, completed evidence, and report are ready for review.
The requested decision is one of:

1. authorize a larger-host Kubernetes completion run;
2. authorize a bounded Kubernetes memory-stabilization investigation; or
3. explicitly accept the resource limitation as non-blocking.

This report does **not** accept Phase 4.0.7, freeze Repository Service 1.0.0,
or authorize Phase 4.0.8.
