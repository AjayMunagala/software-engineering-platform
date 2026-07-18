# Non-Functional Requirements

| Area | Requirement |
| --- | --- |
| Determinism | The same unchanged repository produces equivalent output. |
| Safety | Read-only operation; no code execution, network transmission, or secret disclosure. |
| Explainability | Every detection identifies its source files and detection rule. |
| Performance | Stream traversal; do not load every file into memory. |
| Resilience | Finish with warnings where possible instead of failing on a single unreadable file. |
| Privacy | Reports omit file contents and redact known secret-bearing values. |
| Portability | Support Windows paths first; keep path handling cross-platform. |
| Testability | Detection rules are isolated and covered with fixture repositories. |
