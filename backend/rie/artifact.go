package rie

import (
	"fmt"
	"sync"
)

// Artifact is an immutable, versioned output produced by one engine and
// consumed by later engines. Implementations own their domain-specific API.
type Artifact interface {
	ArtifactName() string
	ArtifactVersion() string
}

// ArtifactStore owns the typed artifacts produced during one pipeline run.
type ArtifactStore struct {
	mu     sync.RWMutex
	values map[string]Artifact
}

// NewArtifactStore creates an empty per-run artifact store.
func NewArtifactStore() *ArtifactStore {
	return &ArtifactStore{values: make(map[string]Artifact)}
}

// Put records an artifact exactly once during a pipeline run.
func (store *ArtifactStore) Put(artifact Artifact) error {
	if artifact == nil {
		return ErrNilArtifact
	}
	name := artifact.ArtifactName()
	if name == "" {
		return ErrArtifactNameRequired
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.values[name]; exists {
		return fmt.Errorf("%w: %s", ErrArtifactAlreadyExists, name)
	}
	store.values[name] = artifact
	return nil
}

// Get returns an artifact by its stable name.
func (store *ArtifactStore) Get(name string) (Artifact, bool) {
	if store == nil {
		return nil, false
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	artifact, exists := store.values[name]
	return artifact, exists
}

// Reset removes artifacts before a new pipeline run reuses the context.
func (store *ArtifactStore) Reset() {
	if store == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	clear(store.values)
}

// ArtifactAs retrieves and type-checks a domain artifact.
func ArtifactAs[T Artifact](store *ArtifactStore, name string) (T, bool) {
	var zero T
	artifact, exists := store.Get(name)
	if !exists {
		return zero, false
	}
	typed, ok := artifact.(T)
	return typed, ok
}
