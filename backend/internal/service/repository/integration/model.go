package integration

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	runtimeapp "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	serviceadapters "github.com/AjayMunagala/software-engineering-platform/backend/internal/service/repository/adapters"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/lifecycle"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const (
	ManifestScheme         = "repository-service-manifest/v1"
	physicalArtifactDomain = "repository-service-storage-artifact-id/v1\x00"
	stageRequestDomain     = "repository-service-stage-request/v1\x00"
)

type Bundle struct{ service repository.Service }

func (bundle *Bundle) Service() repository.Service {
	if bundle == nil {
		return nil
	}
	return bundle.service
}

type composedService struct {
	lifecycle *lifecycle.Service
	scans     *scan.Service
}

func (service *composedService) RegisterRepository(ctx context.Context, request repository.RegisterRepositoryRequest) (repository.Repository, error) {
	return service.lifecycle.RegisterRepository(ctx, request)
}
func (service *composedService) GetRepository(ctx context.Context, query repository.RepositoryQuery) (repository.Repository, error) {
	return service.lifecycle.GetRepository(ctx, query)
}
func (service *composedService) ListRepositories(ctx context.Context, request repository.RepositoryListRequest) (repository.RepositoryPage, error) {
	return service.lifecycle.ListRepositories(ctx, request)
}
func (service *composedService) ArchiveRepository(ctx context.Context, request repository.ArchiveRepositoryRequest) (repository.Repository, error) {
	return service.lifecycle.ArchiveRepository(ctx, request)
}
func (service *composedService) ExecuteScan(ctx context.Context, request repository.ExecuteScanRequest) (repository.ScanResult, error) {
	return service.scans.ExecuteScan(ctx, request)
}
func (service *composedService) GetScan(ctx context.Context, query repository.ScanQuery) (repository.Scan, error) {
	return service.scans.GetScan(ctx, query)
}
func (service *composedService) ListScans(ctx context.Context, request repository.ScanListRequest) (repository.ScanPage, error) {
	return service.scans.ListScans(ctx, request)
}
func (service *composedService) CancelScan(ctx context.Context, request repository.CancelScanRequest) (repository.Scan, error) {
	return service.scans.CancelScan(ctx, request)
}
func (service *composedService) GetArtifact(ctx context.Context, query repository.ArtifactQuery) (repository.Artifact, error) {
	return service.scans.GetArtifact(ctx, query)
}
func (service *composedService) ListArtifacts(ctx context.Context, request repository.ArtifactListRequest) (repository.ArtifactPage, error) {
	return service.scans.ListArtifacts(ctx, request)
}
func (service *composedService) ExportArtifact(ctx context.Context, request repository.ExportArtifactRequest, writer io.Writer) (repository.ExportReceipt, error) {
	return service.scans.ExportArtifact(ctx, request, writer)
}

type Admission struct{ runtime Runtime }
type workLease struct{ work runtimeapp.Work }

func (lease *workLease) Context() context.Context { return lease.work.Context() }
func (lease *workLease) Done()                    { lease.work.Done() }

type SourceProofAdapter struct {
	resolver serviceadapters.SourceResolver
}
type sourceResolution struct {
	source serviceadapters.AuthorizedSource
	proof  lifecycle.SourceProof
}

func (value *sourceResolution) Proof() lifecycle.SourceProof { return value.proof }
func (value *sourceResolution) Close(ctx context.Context) error {
	if value == nil || value.source == nil {
		return nil
	}
	return value.source.Close(ctx)
}

type Store struct {
	repositories persistence.RepositoryStore
	ingest       runtimeIngest
	read         runtimeRead
	contract     *persistence.Contract
	profiles     *repository.ProfileRegistry
	config       Config
}

type runtimeIngest interface {
	persistence.ScanStore
	persistence.PayloadStager
	persistence.PublicationStore
}
type runtimeRead interface {
	persistence.ArtifactReader
	persistence.IntegrityVerifier
}

func toPersistenceDigest(value repository.Digest) persistence.Digest {
	return persistence.Digest(value)
}
func toRepositoryDigest(value persistence.Digest) repository.Digest { return repository.Digest(value) }

func PhysicalArtifactID(public repository.ArtifactID) (persistence.ArtifactID, error) {
	if public == "" {
		return "", integrity("map-artifact-id", "identifier-incompatible")
	}
	hash := sha256.New()
	_, _ = io.WriteString(hash, physicalArtifactDomain)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(public)))
	_, _ = hash.Write(length[:])
	_, _ = io.WriteString(hash, string(public))
	value := hash.Sum(nil)[:16]
	value[6] = (value[6] & 0x0f) | 0x80
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value)
	return persistence.ArtifactID(fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:])), nil
}

func stageRequestID(parent repository.RequestID, artifact repository.ArtifactID) persistence.RequestID {
	hash := sha256.Sum256([]byte(stageRequestDomain + string(parent) + "\x00" + string(artifact)))
	return persistence.RequestID("stage-" + hex.EncodeToString(hash[:16]))
}

func CanonicalManifest(value repository.Scan, artifacts []scan.PublicationArtifact) ([]byte, error) {
	inputs := make([]manifestArtifact, len(artifacts))
	for index := range artifacts {
		inputs[index] = artifacts[index]
	}
	return canonicalManifest(value, inputs)
}

type manifestArtifact interface {
	Metadata() repository.Artifact
	Dependencies() []scan.ArtifactDependency
}

func canonicalManifest(value repository.Scan, artifacts []manifestArtifact) ([]byte, error) {
	if value.ScanID() == "" || len(artifacts) == 0 || len(artifacts) > 64 {
		return nil, integrity("build-manifest", "manifest-build-failed")
	}
	copyArtifacts := append([]manifestArtifact(nil), artifacts...)
	buffer := make([]byte, 0, 1024)
	buffer = append(buffer, []byte(ManifestScheme)...)
	buffer = append(buffer, 0)
	field := func(text string) {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(text)))
		buffer = append(buffer, n[:]...)
		buffer = append(buffer, text...)
	}
	field(string(value.RepositoryID()))
	field(string(value.ScanID()))
	field(value.Profile().Name())
	field(value.Profile().Version())
	profileDigest := value.Profile().Digest()
	buffer = append(buffer, profileDigest[:]...)
	field(value.SourceRevision())
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(copyArtifacts)))
	buffer = append(buffer, count[:]...)
	seen := map[repository.ArtifactID]struct{}{}
	byName := map[string]repository.ArtifactID{}
	for _, item := range copyArtifacts {
		if item.Metadata().ArtifactID() == "" {
			return nil, integrity("build-manifest", "manifest-build-failed")
		}
		seen[item.Metadata().ArtifactID()] = struct{}{}
		byName[item.Metadata().Name()+"\x00"+item.Metadata().Version()] = item.Metadata().ArtifactID()
	}
	for index := 1; index < len(copyArtifacts); index++ {
		left, right := copyArtifacts[index-1].Metadata(), copyArtifacts[index].Metadata()
		leftKey := left.Name() + "\x00" + left.Version() + "\x00" + left.StableIDScheme()
		rightKey := right.Name() + "\x00" + right.Version() + "\x00" + right.StableIDScheme()
		if leftKey >= rightKey {
			return nil, integrity("build-manifest", "manifest-build-failed")
		}
	}
	for _, item := range copyArtifacts {
		metadata := item.Metadata()
		field(string(metadata.ArtifactID()))
		field(metadata.Name())
		field(metadata.Version())
		field(metadata.StableIDScheme())
		field(metadata.CodecName())
		field(metadata.CodecVersion())
		field(metadata.MediaType())
		payloadDigest := metadata.PayloadDigest()
		buffer = append(buffer, payloadDigest[:]...)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], metadata.PayloadSize())
		buffer = append(buffer, size[:]...)
		field(metadata.ProducerName())
		field(metadata.ProducerVersion())
		deps := item.Dependencies()
		var dc [4]byte
		binary.BigEndian.PutUint32(dc[:], uint32(len(deps)))
		buffer = append(buffer, dc[:]...)
		for ordinal, dep := range deps {
			if dep.Ordinal() != ordinal {
				return nil, integrity("build-manifest", "manifest-build-failed")
			}
			source, exists := byName[dep.Name()+"\x00"+dep.Version()]
			if !exists {
				return nil, integrity("build-manifest", "manifest-build-failed")
			}
			if _, ok := seen[source]; !ok || source == metadata.ArtifactID() {
				return nil, integrity("build-manifest", "manifest-build-failed")
			}
			binary.BigEndian.PutUint32(dc[:], uint32(ordinal))
			buffer = append(buffer, dc[:]...)
			field(string(source))
			field(dep.Name())
			field(dep.Version())
		}
	}
	return buffer, nil
}

func ManifestDigest(value repository.Scan, artifacts []scan.PublicationArtifact) (repository.Digest, error) {
	bytes, err := CanonicalManifest(value, artifacts)
	if err != nil {
		return repository.Digest{}, err
	}
	return repository.DigestBytes(bytes), nil
}
