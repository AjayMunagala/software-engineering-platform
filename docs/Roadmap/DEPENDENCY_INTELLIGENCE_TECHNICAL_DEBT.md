# Dependency Intelligence Required Hardening Register

## Status

- Phase 5.0.2: engineering accepted on 2026-09-08
- Production release: not authorized

These findings are non-blocking for the accepted neutral graph artifact/core,
but they are mandatory gates before Dependency Intelligence `1.0.0`.

| ID | Required hardening | Current behavior | Closure evidence |
|---|---|---|---|
| DIE-HARDEN-001 | Bound intermediate evidence accumulation | Published evidence is capped, but duplicate node, containment, and dependency evidence is normalized and accumulated before the final configured cap. | Adversarial duplicate-evidence tests prove bounded intermediate memory, exact occurrence/omission accounting, cancellation, race safety, and deterministic bytes. |
| DIE-HARDEN-002 | Reject nil analysis contexts | `Normalize` dereferences the supplied context before an explicit nil check. | Nil-context tests prove a stable `invalid_input` error and no panic without changing cancellation/deadline behavior. |

Neither item authorizes Phase 5.0.3, SCC/cycle/impact algorithms, platform
integration, or production release. Their implementation must remain within a
separately authorized milestone or the final stabilization gate.
