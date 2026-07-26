# Runtime Infrastructure 1.0.0 Known Limitations

- Health is callable only; HTTP and Kubernetes projections are future
  transport adapters.
- The release defines an exporter-neutral metric sink but does not bundle a
  Prometheus, OpenTelemetry, or vendor exporter.
- Persistence operation histograms and byte counters require later
  orchestration instrumentation; this release provides runtime, health,
  schema, and pool metrics.
- Process signal registration belongs to the future command host.
- Configuration-file watching and live mutation are intentionally unsupported;
  configuration is immutable after startup.
- Automatic migrations are intentionally unsupported.
- Local and CI profiles may disable TLS only for loopback or Unix-socket
  development. Staging and production require `verify-full`.
- No API, UI, authentication, authorization, business logic, or AI
  orchestration is part of this subsystem.
