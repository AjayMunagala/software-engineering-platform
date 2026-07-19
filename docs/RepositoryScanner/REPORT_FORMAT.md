# RIE 1.0.0 Report Format

## Principles

The JSON report is the canonical presentation output. Deterministic detections retain location-aware evidence in their owning sections and immutable artifacts. Counts may be zero; unavailable intelligence is identified explicitly and is never guessed.

```json
{
  "schema_version": "1.0.0",
  "scan": {
    "id": "...",
    "started_at": "...",
    "finished_at": "...",
    "duration_ms": 12.5,
    "engines": []
  },
  "repository": { "name": "ERP", "root_path": "D:/Projects/ERP", "git": true },
  "statistics": { "files": 3248, "folders": 412 },
  "ignore": {},
  "languages": {},
  "frameworks": {},
  "build": {},
  "metadata": {},
  "summary": {
    "artifact": { "name": "repository-intelligence-summary", "version": "1.0.0" },
    "repository_metadata": { "name": "repository-metadata", "version": "1.0.0" },
    "sections": [],
    "capabilities": []
  },
  "metrics": { "files_per_second": 0 },
  "warnings": [],
  "errors": []
}
```

The report schema is additive within RIE 1.x. Engines consume typed artifacts, never presentation fields. Controllers, services, tests, coverage, and complete cross-engine diagnostics remain explicit unavailable capabilities until their owning intelligence engines exist.
