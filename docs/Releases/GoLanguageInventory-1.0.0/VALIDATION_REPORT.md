# Validation Report

The canonical evidence and per-repository measurements are maintained in:

[LIE Phase 2.1.10 Real Repository Validation](../../Validation/LIE_PHASE_2_1_10_REAL_REPOSITORY_VALIDATION.md)

Final decision: approved after the project owner accepted the warm-cache,
reference-CI performance definition on 2026-07-20.

Summary:

- Five pinned public repositories plus one controlled malformed repository.
- 19,380 real Go files parsed with zero unexpected failures.
- Deterministic output in every validation run.
- No crashes, panics, or boundary regressions.
- One medium receiver-extraction defect fixed and revalidated.
- Memory and approved performance gates passed.
