# Supported Feature Matrix

| Capability | 1.0.0 status | Evidence |
|---|---|---|
| Case-insensitive `.go` selection | Supported | Repository-relative file record |
| Package discovery | Supported | Directory plus declared package name |
| Regular and external-test packages | Supported | Independent package identities |
| Imports and aliases | Supported | Default, named, blank, and dot aliases |
| Named structs | Supported | Top-level symbol with exact range |
| Named interfaces | Supported | Top-level symbol with exact range |
| Package functions | Supported | Top-level symbol with exact range |
| Receiver methods | Supported | Base, pointer, and generic receiver facts |
| Package constants and variables | Supported | One symbol per declared name |
| Uppercase `.GO` files | Supported | Same deterministic selection as RIE |
| Test-file identification | Supported | `_test.go` suffix |
| Content attribution | Supported | Byte count and SHA-256 digest |
| Failed/skipped file outcomes | Supported | Structured status and diagnostics |
| Deterministic ordering and IDs | Supported | Stable path/offset/kind/name rules |
| Deep artifact immutability | Supported | Recursive defensive copies |
| Type and identifier resolution | Not supported | Phase 2.2 semantic artifact |
| Call/data-flow graphs | Not supported | Later intelligence engines |
| Repository execution or network | Forbidden | Security boundary |
