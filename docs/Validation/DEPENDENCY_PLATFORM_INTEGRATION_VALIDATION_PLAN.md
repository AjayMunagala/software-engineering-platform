# Phase 5.0.5 — Validation Architecture and Plan

2026-09-11. **Design only; execution not authorized.** No new measured results,
coverage, benchmarks, integration runs, or golden vectors are asserted here.
See [architecture](../Architecture/DEPENDENCY_PLATFORM_INTEGRATION.md),
[codec](../Architecture/DEPENDENCY_PLATFORM_CODEC.md),
[API](../API/DEPENDENCY_PLATFORM_INTEGRATION_CANDIDATE_API.md), and
[ADR 0023](../Decisions/0023-dependency-platform-integration.md).

## Review acceptance conditions

The review must explicitly decide whether standalone artifact publication,
dedicated repository records, logical-only upstream provenance, and deferred
Repository Service profile/typed reload support satisfy the milestone. If not,
revise the design first. Design approval alone does not authorize implementation.

## Future execution order

1. Record design approval and obtain explicit implementation authorization.
2. Independently freeze canonical schema/bytes, codec/profile/UUID/manifest/request
   vectors in a separate commit before production encoding; reuse old vectors.
3. Implement codec/models and bounded spool lifecycle, then fake-backed capability
   conformance. Do not begin PostgreSQL-specific validation before conformance passes.
4. Implement thin persistence/runtime bridges without frozen-contract edits.
5. Execute disposable PostgreSQL/runtime integration and failure/recovery tests.
6. Execute cross-platform quality, security and performance characterization.
7. Commit/push implementation, machine-readable evidence and report for engineering
   acceptance. Stop; do not start 5.0.6 or release.

## Independent validation layers

| Layer | Oracle / evidence required |
|---|---|
| Codec | Independently authored literal expected bytes and hashes; small-fixture equivalence to accepted View serialization plus LF |
| Identity/manifest | Independent binary-frame calculator; mutate every bound field and verify digest/identity behavior |
| Model/config | Boundary tables, zero/nil, invalid UTF-8, unknown versions, defensive copies and redacted formatting |
| Fake conformance | Observable port call trace, scoped publication state, injected receipt/metadata corruption and concurrency failures |
| PostgreSQL | Real frozen migrations and adapter, scoped durable reads, exact payload export and transaction visibility |
| Runtime | Actual public admission/ingest/read capabilities, drain/cancellation/resource lifecycle without pool internals |
| Recovery | New process/runtime instance, no retained spool or in-memory expected result used as success evidence |

Tests may inspect a fake's state and disposable database with test-only observer
connections. Production code must not acquire those privileges or issue SQL.

## Mandatory correctness matrix

- Base, empty, analyzed-empty and nonempty inventories, Unicode and escaping,
  long records, omission counters, uncertain boundaries and partial topology.
- Identical bytes across repeated clean processes and Windows/Ubuntu; collection
  order and evidence remain unchanged. Reuse 1/8 upstream-worker fixtures where
  available; publication itself does not claim multiworker execution.
- No analysis invocation by the codec or publisher; no Core.Normalize side effect.
- Payload quota boundaries, memory-spill boundary, disk-full, short writes,
  read errors, permission failure, close/remove failure and reopen-after-close.
- Single artifact; duplicate base/analyzed variants cannot be published together.
- No projections or dependency edges fabricated from logical source references.
- Active/archived/missing repository, unknown profile and explicit dedicated
  repository routing; frozen service profile/default fixtures remain unchanged.
- Scope mismatch for every operation, guessed IDs and same digest across scopes;
  no cross-scope existence leak or payload export.
- Exact stage digest/size/EOF verification on initial and idempotent attempts.
- Same request and content retry; changed bytes/profile/revision/metadata conflict;
  concurrent independent instances cannot overwrite or partially publish.
- Readers see either no published artifact or the complete committed publication.
  MakeCurrent=false preserves the repository current-scan pointer.
- Every reconciliation field mismatch separately: scope, repository, scan,
  profile, revision, reconstructed manifest hash, UUID, name/version, ID scheme, codec,
  media type, producer, payload size/hash; no scan-ID-only success shortcut.
- Cancellation before/after seal, Begin, stage, immediately before/after commit;
  nil context on every public operation; exactly one lease release.
- Lost Begin reply and lost Publish reply; published-exact success versus
  running/missing unknown outcome; no failure overwrite after possible commit.
- Export verifies into spool before caller writes; corrupt/truncated/reordered/
  extra chunks yield no caller bytes; writer failure yields no success receipt
  and may leave a valid prefix. Verify hash/size independently, not just port receipt.
- Manifest scheme/hash mismatches in Publish receipts fail. Production recovery
  cannot reread the stored publication certificate through the frozen read port;
  distinguish reconstructed-manifest proof from test-only SQL certificate checks.
- Repeated runtime start/ready/publish/export/drain/stop cycles and partial startup
  failure; integration never closes borrowed runtime/pools.

## Crash and durable recovery

Use only a disposable instance owned by the validation harness. Inject process
termination before Begin, after Begin, after staging, and after commit but before
reply. Separately stop/restart PostgreSQL after acknowledged publication. On
restart validate durable metadata, SHA-256 and every payload byte. Running scans
are reported as unknown/orphaned, not automatically resumed or marked succeeded.
Restore a disposable backup into another disposable instance and verify exact
bytes/profile/manifest. Do not use real/personal/production credentials or crash
the user's existing database. Test retention with the existing owner/harness;
publication code itself must not acquire retention capabilities.

## Security and dependency audit

Require no changed released 1.0.0 contract files or changed legacy vectors unless
a separately approved compatible defect has its own evidence. Assert no SQL,
pgx/pool, HTTP, network, command execution, source discovery or runtime startup
dependencies in the codec. Only the integration runtime bridge may import runtime
capability types. No schema migrations, authentication, API listeners, UI, AI,
freshness claims, or global repository-acyclicity claims.

Secret/path audit covers errors, formatting, logs, metrics, manifest, spool names
and reports. Existing repository-relative artifact facts are permitted; absolute
host/source/spool paths and source handles are not. Evidence may report redacted
environment descriptors and fixture identities, not private payload contents.

## Quality and performance evidence

Proposed implementation exit thresholds: >=85% new codec/integration statement
coverage; fake conformance first; full backend tests, vet, shuffled/repeated tests,
Windows and Ubuntu race validation with zero detected races; bounded fuzz runs
for config/metadata/framing and failure sequences. Record commands, Go/tool versions,
seeds, elapsed time, successful executions and failures. Never infer a pass from
a skipped/environment-blocked command.

Characterize the accepted 100k-node/1M-edge fixture and small/base/analyzed cases.
Measure encoding, sealing, stage, publish, verified export and recovery separately.
Prerequisite construction is outside integration timing but report its cost and
peak separately. Report output byte count, allocation bytes/count, sampled live
heap, sampling interval, RSS where available, spool bytes and cleanup latency.
Measure same fixture/config/revision on Windows and Ubuntu. Report warm-cache
repeats separately from cold filesystem measurements and document the cache method.

Record exact CPU, RAM/WSL limits, OS/kernel, Go version, PostgreSQL version/config,
filesystem/storage, free disk, client buffers, pool limits, TLS, worker configuration
and Git revision/version. Label timeout, host memory ceiling, disk quota and
correctness failures distinctly. Protect the host with explicit safety ceilings.

The existing Core 30-second/2-GiB exact synthetic gate remains unchanged and must
be rechecked after authorized implementation. It is not a platform publication
or hard memory guarantee. Integration latency/throughput/heap measurements are
characterization until separately approved targets exist. No invented threshold
or reclassification of the open hardening obligations.

## Required evidence package and final gate

Future report and machine-readable results must record fixture/input and output
hashes, codec/profile/vector revisions, tested platforms, commands/outcomes,
scope/conformance/recovery matrix, environment, counts, timing/memory methodology,
defects/fixes, skipped gates, limitations and actual commit. Include clear flags
for no typed reload, no service profile integration and no verified upstream
payload closure. Documentation must not call these implemented features.

Phase acceptance requires all mandatory correctness/security/quality gates or
an explicit engineering exception, reviewed characterization, and compatibility
confirmation. Commit/push evidence before the user's engineering review.
DIE-HARDEN-001/002 remain open. No candidate promotion, tag or next phase follows
automatically from a successful implementation run.
