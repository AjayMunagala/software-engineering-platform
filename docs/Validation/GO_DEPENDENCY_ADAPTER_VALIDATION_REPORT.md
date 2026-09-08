# Phase 5.0.3 — Go Dependency Adapter Validation

Date: 2026-09-08. Candidate version: **0.1.0**.
Status: **Engineering accepted** for implementation commit
[`a0a1a779734b23a0f6c2a8f711130063822a9761`](https://github.com/AjayMunagala/software-engineering-platform/commit/a0a1a779734b23a0f6c2a8f711130063822a9761).
ADR 0021 is **Accepted**. Candidate 0.1.0 is not a production release.
Phase 5.0.4, algorithms, integration, and release remain unauthorized.
The measurements below are the original submitted evidence, not new test runs.

## Traceability and scope

Design approval applies to commit
`4b77de2034af7619704d8df5f5f65ef31b5dd552`.
The independent boundary vectors were frozen and pushed in
[`7eb2a98271499f381dd60ca773ab1e6f02a11b01`](https://github.com/AjayMunagala/software-engineering-platform/commit/7eb2a98271499f381dd60ca773ab1e6f02a11b01)
before production encoding. Expected hashes were computed using a standalone
PowerShell/.NET byte encoder, not the Go implementation under test. The GitHub
implementation review unit is `a0a1a779734b23a0f6c2a8f711130063822a9761`;
the subsequent governance commit records acceptance without changing that evidence.

The new eight-file `backend/die/golang` package consumes released immutable
artifacts, validates joins, translates Go facts, and invokes `die.Core.Normalize`.
No released artifact or accepted core file was changed. No SCC, cycle, impact,
platform integration, parser, source reader, tool execution, network, database,
runtime, or service behavior was introduced.

## Environment

| Item | Windows | Ubuntu |
|---|---|---|
| OS | Windows 11 Home Single Language, 10.0.26200 | Ubuntu 24.04 under WSL2 |
| Kernel | Windows | 6.18.33.2-microsoft-standard-WSL2 |
| Go | 1.26.2 windows/amd64 | 1.26.2 linux/amd64, existing `/opt/aegis-go-1.26.2` |
| Git | 2.54.0.windows.1 | 2.43.0 |
| CPU | Intel Core i5-12450H, 8 cores / 12 logical processors | Same physical host |
| Memory | 8,090,104 KiB OS-visible; 1,499,132 KiB free at audit sample | 3,770 MiB guest RAM; 1,024 MiB swap |
| Race compiler | Existing MSYS2 UCRT64, CGO enabled | Existing Linux C compiler, CGO enabled |

These are workstation measurements, not isolated CI results. Ubuntu's default
system Go is 1.22.2; validation explicitly selected the existing 1.26.2 toolchain
with `GOTOOLCHAIN=local`. No toolchain, database, or credentials were installed.
One initial WSL invocation failed because a Windows PATH containing spaces was
expanded by the shell; the corrected invocation used a fixed Linux PATH.

## Quality evidence

Commands are run from `backend` unless otherwise noted.

| Gate | Command / evidence | Result |
|---|---|---|
| Conformance first | `go test ./die/conformance ./die/golang -count=1 -v`; reference core suite ran first | PASS after correcting an unsupported fixture expectation |
| Adapter unit tests | `go test ./die/golang -count=1` | PASS |
| Statement coverage | `go test ./die/golang -coverprofile=adapter-coverage.out` | 95.2%; exceeds 85% gate |
| Windows regression | `go test ./...` | PASS |
| Ubuntu regression | `go test ./...` | PASS |
| Vet | `go vet ./...`, both platforms | PASS |
| Shuffled execution | Windows full backend `-shuffle=on -count=3`; Ubuntu adapter same flags | PASS |
| Full race | `CGO_ENABLED=1 go test -race ./...`, both platforms | PASS, no races detected |
| Final adapter race | `go test -race ./die/golang -count=1`, both platforms | PASS |
| Identity/path fuzz | `-fuzz '^FuzzBoundaryAndPaths$' -fuzztime=20s -parallel=2` | PASS; 74,328 executions |
| Proof/status/budget fuzz | `-fuzz '^FuzzProofJoins$' -fuzztime=20s -parallel=2` | PASS; 101,289 executions |
| Total fuzz executions | Two bounded Windows campaigns | 175,617 |
| Direct-import audit | `go list -f '{{join .Imports "\n"}}' ./die/golang` | Only standard utility packages and released RIE/LIE/core imports |

Full backend commands exercise the default test suite. They do not claim a new
live PostgreSQL integration campaign; opt-in infrastructure tests remain opt-in
and persistence/runtime integration is excluded from this milestone. Some
unaffected packages reused valid Go test cache entries on final reruns.

## Correctness and determinism

Tests cover zero inputs/configuration, stored metadata checks, duplicate IDs,
package membership, unsafe paths, contradictory digests, proof importer/context/
target/candidate joins, unknown resolution states, exact alias/location joins,
missing bindings, explicit omissions, and stable redacted errors.

Released-engine fixtures cover empty notes repositories, unmanaged packages,
workspace/local replacement, nested modules, segment-prefix separation,
Unicode paths, vendor evidence, malformed source, external imports, all four
alias forms, and cross-file references. Controlled detached facts separately
exercise standard-library proof, ambiguous/stale/partial states, and explicit
vendor-context selection. They do not claim the released engine selected a
context that it actually left unresolved.

The small workspace fixture contains three Go files, two modules, two imports,
and a Unicode filename. Its output has **8 nodes and 7 edges**. Complete JSON
bytes, not just graph counts, match across Windows/Ubuntu, one/eight upstream
workers, three independent test processes on each host, repeated calls, and
shuffled detached inputs. Its SHA-256 is:

```text
6e6886d02d64ae4e3a98e467a21d2f444485c3ea171681f2b8ac259ab6eb7d3e
```

The production translator is serial. Eight concurrent calls on the same engine
also return identical bytes. Prerequisite bytes are unchanged; mutating output
accessor copies cannot change the inventory. SCC and cycle collections are empty.
The alias fixture retains five syntax import occurrences in the package graph;
binding/proof evidence does not multiply occurrence counts.

## Limits and cancellation

Raw input, nested identifier, generated record/evidence, and unique-node limits
fail with no artifact. Evidence expansion is checked before constructing the
expanded graph evidence slice. Generated diagnostics also obey the raw budget.
Published caps and exact available omission accounting remain core-owned.

Nil, canceled, and expired contexts return safe errors. A deterministic context
cancels at successive checks through extraction, indexing, translation, proof
handling, normalization, and return; canceled calls publish no artifact. This
is checkpoint evidence, not a hard wall-clock cancellation guarantee. Released
accessor clones and the accepted core sort remain uninterruptible units.

## Performance characterization

`BenchmarkAdapter` runs fixture engines and filesystem setup **outside** the
timed region. Each operation includes accessor copies, validation/indexing,
translation, and core normalization. Counts below are source files/imports;
each file declares one function that imports/calls `fmt`. This deliberately
aggregates many package-edge evidence contributions onto a duplicate-heavy edge.
Each range represents three `-benchtime=1x -count=3 -benchmem` samples.

| Files/imports | Windows time | Ubuntu time | Windows allocated bytes/op |
|---|---|---|---|
| 100 | 5.06–5.65 ms | 4.87–7.68 ms | 3,663,688–3,719,848 |
| 1,000 | 96.01–98.55 ms | 62.53–67.25 ms | 44,111,432–44,366,568 |
| 10,000 | 1.031–1.161 s | 0.653–0.727 s | 497,991,136–501,804,616 |

At 10,000 imports Windows allocated approximately 8.39–8.48 million objects;
Ubuntu allocated 499,045,264–503,563,664 bytes and 8.42–8.51 million objects.
These totals are not peak live heap. An earlier Windows run overlapped other
quality jobs and measured 1.148–1.718 s at 10,000; the table uses the later run
without the full regression/race jobs competing for resources.

Initial stage measurements at 1,000 imports on Windows:

| Timed stage | Time | Allocation bytes/op |
|---|---|---|
| Released accessor extraction | 0.454–0.470 ms | 1,145,872 |
| Validation/indexing/translation | 12.91–13.50 ms | ~9,733,224 |
| Accepted core normalization | 80.06–95.26 ms | ~32,930,000 |

These stage measurements preceded the final additional proof-loop cancellation
checks; they identify the cost distribution, not a separately frozen API gate.
No adapter-specific performance acceptance threshold has been invented. The
unchanged neutral-core 100k-node/1m-edge two-GiB gate was not rerun because no core
behavior changed.

### Sampled memory — opt-in 10,000-file run

`DIE_ADAPTER_MEMORY_TEST=1 go test ./die/golang -run TestScaleMemory -count=1 -v`
samples Go heap every millisecond. The baseline includes live prerequisite
artifacts; fixture engine execution precedes the measured interval.

| Metric | Windows | Ubuntu |
|---|---|---|
| Baseline live heap, bytes | 21,633,680 | 20,809,608 |
| Sampled peak live heap, bytes | 139,540,736 | 125,169,032 |
| Allocated bytes during analysis | 497,618,776 | 499,926,152 |
| Allocations during analysis | 8,380,455 | 8,438,908 |
| Instrumented elapsed | 3.586 s | 0.750 s |

Both outputs contain 10,003 nodes, 10,001 edges, 1,000 published diagnostics,
and 29,992 omitted evidence records. Heap sampling adds overhead and can miss
short-lived peaks; these figures are not RSS, a hard memory ceiling, or a
Kubernetes qualification. Windows sampling overlapped other validation work.

## Findings and refinements

1. A test initially assumed `fmt` was automatically standard-library-proven.
   Released default proofs classify it as external without explicit stdlib
   authority. The test was corrected; production never infers this from strings.
2. The vendor fixture supplies a resolved vendor proof plus an external
   single-module proof, while its semantic binding remains unresolved. The
   adapter preserves that uncertainty instead of selecting the vendor candidate.
3. Omission-explained missing proofs now produce unresolved boundaries rather
   than contradictory local claims. Unexplained dangling joins still fail.
4. Alias names and full source ranges are checked, not just import path and kind.
5. Nested ID counts, generated diagnostics, pre-expansion evidence bounds, and
   proof-loop cancellation checks were hardened before review submission.
6. Module lookup walks declared ancestor roots instead of comparing every
   package against every module; the unused earlier ancestry helper was removed.

## Accepted limitations and governance decision

- No cryptographic same-scan proof or present filesystem freshness is available.
- Byte memory is not capped by count budgets; released clones precede size checks.
- Core normalization remains the dominant measured allocation cost.
- Direct-core DIE-HARDEN-001 and DIE-HARDEN-002 stay open. Adapter guards do not
  close those accepted release obligations.
- No production release, SCC/cycle/impact, language expansion, or platform
  integration was implemented or authorized.

Engineering explicitly accepted **Phase 5.0.3 only** after reviewing the GitHub
implementation commit and this evidence. ADR 0021 is promoted to Accepted.
The limitations above are accepted, not blockers for this milestone; the two
direct-core hardening obligations remain open. No new performance gate or
release approval is implied. Phase 5.0.4 design is a separate future milestone;
this governance update does not begin it.
