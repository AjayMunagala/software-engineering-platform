# RIE Technical Debt Register

This register contains deliberate, non-blocking work deferred from RIE 1.0.0. An item is implemented only when evidence justifies the added complexity.

| ID | Item | Current decision | Trigger for reconsideration |
|---|---|---|---|
| RIE-DEBT-001 | Shared `ManifestInventory` | Deferred. Framework and Build perform separate bounded reads. | Repeated parsing becomes a measured performance or consistency problem. |
| RIE-DEBT-002 | Framework definition registry | Definitions remain close to manifest parsers. | Supported frameworks or manifest types make maintenance materially difficult. |
| RIE-DEBT-003 | Generated or JSON detector registry | Build uses a simple Go registry. | Non-Go contributors or runtime extension becomes a validated requirement. |
| RIE-DEBT-004 | `RunContext.Entries` compatibility bridge | Retained only for Discovery-to-Ignore transfer. New engines consume artifacts. | RIE pipeline major version permits replacing the bridge with a raw discovery artifact. |
| RIE-DEBT-005 | Cross-engine `DiagnosticsInventory` | Complete diagnostics are explicitly unavailable in Summary. | A future consumer requires immutable diagnostic aggregation. |
| RIE-DEBT-006 | Rich toolchain semantics | Constraints are preserved exactly as declared. | Version comparison or environment compatibility becomes an approved responsibility. |
| RIE-DEBT-007 | Git worktree external metadata | RIE reads bounded metadata only from an in-root `.git` directory. | A safe authorization model for external worktree Git directories is designed. |
| RIE-DEBT-008 | Race-detector CI | Local Windows runtime has CGO disabled. | CI provides a supported CGO/race-enabled Go environment. |

None of these items changes the RIE 1.0.0 correctness boundary. They must not be used as justification for starting LIE features inside RIE.
