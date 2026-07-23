# GoSemanticInventory 1.0.0 Known Limitations

- No compiler-equivalent GOOS, GOARCH, build-tag, cgo, generated-source, or
  toolchain selection. Mutually exclusive declarations remain explicit.
- No ambient module-cache, export-data, standard-library-index, or network
  importer. Unavailable dependencies remain external/unresolved/unknown.
- Every execution is a full rebuild; incremental invalidation is deferred.
- The default one-million relationship limit can omit later-priority
  references or interface checks on Kubernetes-scale repositories. The exact
  omission count is published.
- Kubernetes-scale analysis remains memory intensive: the stabilized candidate
  measured 3.88–3.92 GiB peak heap on the reference workstation.
- Parser and type-check calls are synchronous bounded units, so cancellation is
  cooperative rather than instantaneous.
- Windows filesystem/filter-driver variability prevents a reproducible local
  cold-cache pipeline gate. Filesystem-inclusive observations are recorded
  separately from engine performance.
- No call graph, control flow, data flow, architecture inference, bug
  reasoning, patch generation, or validation execution is included.
