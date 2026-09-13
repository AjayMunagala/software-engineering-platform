# Opt-in dependency publication (candidate 0.1.0)

This application bridge borrows frozen Persistence Port capabilities and admission.
It does not own repositories, engines, pools, migrations, runtime startup/shutdown,
or Repository Service profiles. `FromRuntime` only borrows public capabilities;
actual runtime integration validation remains gated.

Construct a host-owned private `SpoolFactory`, immutable allowlisted `Config`, and
`Service` with `New`. The host must register dedicated scope/repository records.
`Publish` accepts an already normalized immutable inventory. It serializes/seals,
stages with complete stream consumption, publishes one envelope with MakeCurrent
false, and reconciles durable metadata. No upstream persistence edges are invented.

`Get` checks scope, profile and complete envelope metadata. `Reconcile` compares an
exact expected publication; missing/running is unknown, not proof of failure.
An `Uncertain` error exposes its immutable `Expected()` value for later recovery.
The frozen read port has no certificate getter: manifest reconstruction is based
on scan/envelope metadata, not a new SQL query or an invented certificate read.

`Export` first verifies all stored bytes in a private spool before writing to the
caller. A failing caller writer can receive a valid partial prefix, never a success
receipt. Publish retries encode/check the complete input, but an already published
scan is reconciled rather than restaged (the frozen stage operation requires a
running scan). One publish/export per service instance is admitted; reads can overlap.

## Resource and security boundaries

- Default memory threshold 64 MiB; maximum payload 4 GiB; copy buffer 1 MiB.
- Host supplies an absolute private directory outside source repositories, with
  appropriate ACLs. Random exclusive files are confined through `os.Root`; Unix
  creation mode is 0600. The host owns Windows ACLs and crash-orphan cleanup.
- Only successfully created files are removed; live readers prevent removal.
- Finalization uses a bounded context (default five seconds). Blocking dependency
  calls must honor context. Filesystem calls, cloning and individual JSON records
  are not hard wall-clock or heap bounds. No abandoned copy goroutine is created.
- Revision is path-free UTF-8, at most 512 bytes, aligned with the frozen port;
  slash/backslash and control characters are rejected. No secrets belong here.
- Public formatting is redacted; raw dependency failures are translated.

Fake-capability tests cover publication, lost replies, retries, metadata corruption,
scope isolation, stream consumption, cancellation, spool ownership and cleanup.
They do not prove PostgreSQL durability or real runtime integration. Candidate
implementation is submitted for review, not accepted or released.
