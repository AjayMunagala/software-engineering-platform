# Dependency canonical codec (candidate 0.1.0)

`New(maximumBytes)` constructs an opt-in encoder. `Encode(ctx, inventory, writer)`
consumes an accepted immutable inventory without running analysis and returns a
size/SHA-256 receipt only after complete output. Zero maximum selects 4 GiB.

Output follows the independently frozen Phase 5.0.5 literals: ordered compact JSON,
Go JSON escaping, integer-preserving records, and exactly one final LF included
in the digest. Invalid UTF-8, unsupported metadata, cancellation, short writes,
and output limits fail without a success receipt. The writer can receive a partial
prefix on failure; callers must discard it. There is no decoder or rehydration API.

Collections are accessed one at a time and records encoded individually. Released
accessor copies and individual JSON records remain allocation/uninterruptible
units; this is not a hard process-memory ceiling. DIE-HARDEN-001/002 remain open.

Tests compare literal bytes/digests from `experiments/dependency-platform-vectors`.
Wire-only synthetic vectors test record encoding, not construction of otherwise
invalid normalized inventories. Run from backend:

```text
go test ./die/codec -cover
go test -race ./die/codec
```
