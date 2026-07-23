# GoPackageIdentityInventory 1.0.0 Release Notes

## Status

Released on 2026-07-23 under the annotated tag
`go-package-identity-inventory/v1.0.0`.

`GoPackageIdentityInventory` proves repository-local Go import identity from
snapshot-authorized `go.mod`, `go.work`, local replacement, nested-module, and
verified vendor evidence. It performs no command execution, network access,
module-cache access, downloads, or repository writes.

The 1.0 contract includes immutable typed accessors, a detached JSON view,
strict stable enum strings, deterministic proof precedence and ordering, and
the `go-package-proof-id/v1` stable-ID scheme.

Reference validation on Windows amd64 with Go 1.26.2 processed 10,000 proof
fixtures in 55.0–58.3 ms and approximately 20.0 MB allocated. Package coverage
is 86.9%; targeted and full-backend race gates pass with zero races.
