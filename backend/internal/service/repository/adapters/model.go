package adapters

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

type artifactSpec struct {
	name, version, scheme     string
	producer, producerVersion string
	dependencies              []artifactRef
}

type artifactRef struct{ name, version string }

type encodedArtifact struct {
	spec  artifactSpec
	value rie.Artifact
}

// sealedPayload is a reopenable, permission-restricted exact-byte spool.
// Its deployment-local path is private and is removed exactly once.
type sealedPayload struct {
	path   string
	digest repository.Digest
	size   uint64
	mu     sync.Mutex
	closed bool
}

func (payload *sealedPayload) Open(ctx context.Context) (io.ReadCloser, error) {
	if err := contextError(ctx, "open-materialized-artifact"); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, serviceError(repository.ErrorMaterializationFailed, "open-materialized-artifact", "artifact-closed", ErrArtifactClosed)
	}
	payload.mu.Lock()
	defer payload.mu.Unlock()
	if payload.closed {
		return nil, serviceError(repository.ErrorMaterializationFailed, "open-materialized-artifact", "artifact-closed", ErrArtifactClosed)
	}
	file, err := os.Open(payload.path)
	if err != nil {
		return nil, serviceError(repository.ErrorMaterializationFailed, "open-materialized-artifact", "spool-unavailable", err)
	}
	return file, nil
}

func (payload *sealedPayload) close() error {
	if payload == nil {
		return nil
	}
	payload.mu.Lock()
	defer payload.mu.Unlock()
	if payload.closed {
		return nil
	}
	payload.closed = true
	if err := os.Remove(payload.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type session struct {
	adapter      *Adapter
	source       AuthorizedSource
	root         string
	fingerprint  repository.Digest
	revision     string
	profile      repository.AnalysisProfile
	repositoryID repository.RepositoryID
	mu           sync.Mutex
	payloads     []*sealedPayload
	analyzed     bool
	closed       bool
}

func (current *session) SourceFingerprint() repository.Digest { return current.fingerprint }
func (current *session) SourceRevision() string               { return current.revision }

type metadataView struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	EngineName    string `json:"engine_name"`
	EngineVersion string `json:"engine_version"`
}

func metadata(value rie.ArtifactMetadata) metadataView {
	return metadataView{Name: value.Name, Version: value.Version, EngineName: value.EngineName, EngineVersion: value.EngineVersion}
}

func dependencies(values []artifactRef) ([]scan.ArtifactDependency, error) {
	result := make([]scan.ArtifactDependency, len(values))
	for index, value := range values {
		dependency, err := scan.NewArtifactDependency(value.name, value.version, index)
		if err != nil {
			return nil, err
		}
		result[index] = dependency
	}
	return result, nil
}
