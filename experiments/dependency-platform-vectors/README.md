# Independent Phase 5.0.5 vectors

Specification-only calculator and independent verifier, isolated from `backend/`.
See [freeze record and full schema table](../../docs/API/DEPENDENCY_PLATFORM_GOLDEN_VECTORS.md).

From repository root:

```powershell
pwsh -NoProfile -File experiments/dependency-platform-vectors/calculate.ps1
node experiments/dependency-platform-vectors/verify.mjs
```

Both commands are read-only except stdout. `vectors.json` contains literal
expected values; neither command overwrites it. Payload base64 is authoritative
binary content and must equal the UTF-8 string including one final LF. JSON file
line endings are not payload line endings. Do not parse/re-serialize payload
integers through JavaScript Number (wire cases exceed 2^53).

No database, runtime, source execution, production codec, backend test harness,
or external dependency is used. Production encoding awaits vector review.
