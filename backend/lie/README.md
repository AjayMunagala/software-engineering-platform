# Language Intelligence Engine

The LIE runner consumes the immutable `RepositorySnapshot 1.0.0` and
`LanguageInventory 1.0.0` artifacts. It registers independent language engines,
validates their identities, runs them in registration order, and publishes each
language artifact exactly once.

An empty engine registry is valid. The runner never reads repository files,
executes external tools, accesses the network, or modifies RIE artifacts; those
responsibilities belong to individual language engines and the artifact store.
