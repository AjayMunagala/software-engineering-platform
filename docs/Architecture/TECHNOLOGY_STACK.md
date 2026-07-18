# Technology Stack — Initial Direction

| Area | Decision | Reason |
| --- | --- | --- |
| Desktop/web UI | React + TypeScript | Mature UI ecosystem and typed client code. |
| Core backend | Go | Fast, portable tooling and strong filesystem/concurrency support. |
| AI services | Python | Strong ecosystem for model and data tooling. |
| Parser | Tree-sitter | Incremental, multi-language syntax parsing. |
| Relational storage | PostgreSQL | Durable structured metadata and audit records. |
| Vector search | Qdrant | Semantic retrieval when the knowledge layer is introduced. |
| Packaging | Docker on Linux | Reproducible services and deployment. |

These are initial decisions, not implementation requirements for Version 1. The scanner should avoid premature coupling to databases, vector stores, models, or containers.
