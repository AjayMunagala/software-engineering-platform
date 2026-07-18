# API Specification — Future Direction

The first API boundary will expose read-only scanner operations:

```text
POST /v1/repository-scans
GET  /v1/repository-scans/{scanId}
GET  /v1/repository-scans/{scanId}/report
```

Requests must identify a local path through the host-side interface; raw remote path submission is not sufficient authorization. API details, authentication, persistence, and asynchronous jobs remain deferred until the scanner architecture is implemented.
