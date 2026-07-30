# Repository Service Stabilization Validation Report

## Decision state

- Phase: 4.0.8 implementation and validation
- Candidate revision tested: `2f31c2273f36e81d18e17e7280d9b019c3c27d6c`
- Status: evidence complete; awaiting final engineering acceptance
- Repository Service version: `0.1.0`
- Proposed promotion: `1.0.0`
- Release tags: not created
- Date: 2026-07-30

## Outcome

All mandatory locally executable stabilization gates pass. No production defect
was found and no production behavior changed. The Phase 4.0.7 larger-host
Kubernetes qualification remains open and is presented for explicit release
disposition; it is not reported as a pass.

## Review recommendations resolved

1. Promotion criteria are separated into mandatory release gates and optional
   improvements in the accepted stabilization architecture and plan.
2. Production changes are limited to release-blocking compatible defect fixes.
   No such change was required.
3. Measurable exit criteria cover API, golden vectors, quality, determinism,
   integration, resources, security, documentation, qualifications, versioned
   revalidation, and tag verification.

## Environment

| Property | Windows | Ubuntu |
| --- | --- | --- |
| OS | Windows 11 10.0.26200 | Ubuntu 24.04 on WSL2, kernel 6.18.33.2 |
| CPU | Intel i5-12450H, 8 cores / 12 logical | same host |
| Visible RAM | 8,090,104 KiB | bounded WSL guest |
| Go | 1.26.2 windows/amd64 | 1.26.2 linux/amd64 |
| C compiler | MSYS2 UCRT64 GCC 16.1.0 | GCC 13.3.0 |
| Git | 2.54.0.windows.1 | 2.43.0 |
| PostgreSQL | 18.4 disposable Ubuntu cluster | 18.4 |

The disposable integration cluster used accepted migrations, loopback-only
trust for the temporary cluster, disposable capability identities, and no
committed credentials.

## Mandatory quality gates

| Gate | Result |
| --- | --- |
| Full backend regression | PASS on Windows and Ubuntu |
| Three shuffled full-backend runs | PASS |
| Linux five-run shuffled contract suite | PASS |
| `go vet ./...` | PASS on Windows and Ubuntu |
| Windows full race | PASS, zero races |
| Ubuntu targeted and full race | PASS, zero races |
| Golden vectors | PASS on Windows and Ubuntu |
| Production composition | PASS against disposable PostgreSQL 18.4 |
| Immediate-stop/restart recovery | PASS, seven exact artifacts |
| Listener/command audit | PASS |
| Neutral dependency audit | PASS |
| Secret audit | PASS |
| Production tree unchanged | PASS |

Coverage:

| Package | Statements |
| --- | ---: |
| Neutral Repository Service | 94.8% |
| Conformance | 85.0% |
| Repository lifecycle | 85.4% |
| Scan execution | 88.6% |
| Intelligence/materialization adapters | 86.4% |
| Persistence/runtime integration | 85.2% |

All changed production-package thresholds remain at or above 85%.

## Fuzz evidence

Ten deterministic fuzz targets covering artifact identity, source handles,
source proof, artifact candidates, forbidden source variants, physical IDs,
manifests, error translation, real source manifests, and result normalization
passed. Six campaigns reported at least 573,627 executions; four additional
bounded campaigns completed corpus gathering without a panic or contract
failure. Previously accepted Phase 4.0.6 and 4.0.7 fuzz evidence remains part
of the release history.

## Fresh production-composition proof

The pinned `sindresorhus/p-limit` revision
`42599ebbbb1228a5bdab381fcf8f4ac20eb8d551` was rerun through the production
composition with a fresh disposable migrated PostgreSQL cluster:

- seven artifacts published and exported exactly;
- normalized SHA-256:
  `ab6097257f194bb62c15455b3eeb31544aebd924753463e93ec10a92dbeb4c59`;
- scan duration: 471.644 ms;
- peak Go heap: 9,345,352 bytes;
- immediate PostgreSQL stop/restart recovery: PASS;
- recovery publication SHA-256:
  `7adc0a61a9ea9732fa02fa8fa43244d6cf1a12399589c8ce8486c0e077df46ba`.

The normalized digest is identical to the accepted Phase 4.0.7 evidence.

## Determinism and API review

- Public, physical, profile, and manifest golden tests pass on both operating
  systems.
- `repository-go/v1` still resolves to the frozen ordered artifact graph.
- The public contract remains transport-, persistence-, runtime-, and
  engine-neutral.
- The three capability interfaces and composed convenience interface are
  unchanged.
- All service/lifecycle/scan `ContractVersion` constants remain `0.1.0` until
  final acceptance.
- Exact candidate source-file hashes are recorded in the release API snapshot.

## Performance summary

No accepted absolute target regressed. Windows filesystem-backed
materialization remains within its previously documented filter-driver range;
Linux remains substantially faster. Representative fresh ranges:

| Benchmark | Windows | Ubuntu |
| --- | ---: | ---: |
| Public artifact identity | 1.968-2.307 us | 1.036-1.141 us |
| Register validation | 482.5-556.6 ns | 203.9-256.5 ns |
| Independent scan execution | 50.36-57.38 us | 33.44-45.80 us |
| 10,000-entry materialization | 76.08-102.51 ms | 10.92-12.94 ms |
| Physical artifact ID | 1.552-1.732 us | 890.7-994.0 ns |
| Ten-artifact manifest | 17.70-21.29 us | 8.811-11.15 us |
| Ten-artifact integration translation | 36.37-42.34 us | 18.10-20.69 us |

The manifest and integration values remain far below the accepted 5 ms and
25 ms gates. Materialization allocates approximately 2.62 MiB on Windows and
1.56 MiB on Ubuntu, below the 64 MiB working-memory gate and without an extra
artifact-sized copy.

## Security and boundary review

The neutral service dependency graph contains no pgx, `database/sql`, or
`net/http`. Production service/integration code contains no listener, command
execution, clone, or process-launch path. Secret scans find no committed
private key or authenticated PostgreSQL URL. Existing scope, source-handle,
absolute-path, repository-mutation, and raw-error tests pass.

## Defects

No production or validation defect was found during Phase 4.0.8. Therefore no
code change, API change, migration, engine change, or performance workaround
was introduced.

## Open release qualification

The following Phase 4.0.7 evidence remains pending a larger race-capable host:

- Kubernetes Windows one-worker completion;
- Kubernetes Ubuntu eight-worker completion.

The available host stopped these runs at safe memory ceilings. Two Windows
eight-worker clean processes completed with identical normalized output. No
correctness, corruption, race, publication, or determinism defect was observed
in completed runs.

Final engineering review must explicitly accept this qualification for
`1.0.0`, close it with append-only larger-host evidence, or block release.

## Final local decision

Phase 4.0.8 implementation and validation are locally complete. The candidate
is ready for final engineering review. Version promotion, the final versioned
regression/race rerun, and annotated namespaced tags remain unauthorized until
that review accepts the evidence and qualification.
