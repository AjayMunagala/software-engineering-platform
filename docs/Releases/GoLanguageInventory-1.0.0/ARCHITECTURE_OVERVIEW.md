# Architecture Overview

```text
RepositorySnapshot 1.0.0 ─┐
                          ├── Go Language Engine 1.0.0
LanguageInventory 1.0.0 ──┘          │
                                     ▼
                       GoLanguageInventory 1.0.0
```

The Go engine depends on the lowest frozen artifacts containing the facts it
needs. It does not consume `RunContext`, reports, framework inventories, build
inventories, or future semantic artifacts.

Processing stages:

```text
candidate verification
    → rooted bounded reads
    → SHA-256 attribution
    → go/parser syntax tree
    → deterministic fact extraction
    → package/symbol aggregation
    → diagnostic limiting
    → deep immutable artifact
```

Files are parsed independently with `go/parser` and
`parser.SkipObjectResolution`. Workers write to predetermined outcome positions;
the collector sorts all externally visible facts before publication.

The artifact store remains single-assignment. Phase 2.2 consumes this frozen
artifact read-only and publishes separate semantic results.
