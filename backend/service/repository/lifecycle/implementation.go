package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

const (
	registerFingerprintDomain = "repository-service-register/v1\x00"
	archiveFingerprintDomain  = "repository-service-archive/v1\x00"
)

// Service coordinates production repository lifecycle behavior only.
type Service struct {
	store    Store
	resolver SourceProofResolver
	clock    Clock
	config   Config
}

func New(store Store, resolver SourceProofResolver, clock Clock, configs ...Config) (*Service, error) {
	if store == nil || resolver == nil || clock == nil {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-repository-lifecycle", "invalid-dependencies", false, nil)
	}
	if len(configs) > 1 {
		return nil, fmt.Errorf("%w: at most one configuration is accepted", ErrInvalidConfig)
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Service{store: store, resolver: resolver, clock: clock, config: config}, nil
}

func (service *Service) RegisterRepository(ctx context.Context, request repository.RegisterRepositoryRequest) (repository.Repository, error) {
	if service == nil || ctx == nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInvalidInput, "register-repository", "invalid-request", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "register-repository", "context-ended", false, err)
	}
	resolution, err := service.resolver.Resolve(ctx, request.Scope(), request.SourceHandle())
	if err != nil {
		return repository.Repository{}, mapDependencyError(err, "register-repository", "source-unavailable", repository.ErrorSourceUnavailable)
	}
	if resolution == nil {
		return repository.Repository{}, repository.NewError(repository.ErrorSourceUnavailable, "register-repository", "invalid-source-resolution", false, nil)
	}
	proof := resolution.Proof()
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), service.config.SourceCloseTimeout)
	closeErr := resolution.Close(closeCtx)
	cancel()
	if proof.IsZero() {
		return repository.Repository{}, repository.NewError(repository.ErrorSourceUnavailable, "register-repository", "invalid-source-proof", false, nil)
	}
	if closeErr != nil {
		return repository.Repository{}, mapDependencyError(closeErr, "register-repository", "source-cleanup-failed", repository.ErrorSourceUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "register-repository", "context-ended", false, err)
	}
	now := service.clock.Now().UTC()
	if now.IsZero() {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "register-repository", "invalid-clock", false, nil)
	}
	value, err := repository.NewRepository(repository.RepositoryParams{
		RepositoryID: request.RepositoryID(), DisplayName: request.DisplayName(), SourceKind: proof.Kind(),
		FingerprintScheme: proof.FingerprintScheme(), Fingerprint: proof.Fingerprint(), State: repository.RepositoryActive,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "register-repository", "invalid-repository-model", false, nil)
	}
	fingerprint := registerFingerprint(request, proof)
	result, err := service.store.Register(ctx, newRegisterCommand(request.Scope(), request.RequestID(), fingerprint, value))
	if err != nil {
		return repository.Repository{}, mapDependencyError(err, "register-repository", "store-failed", repository.ErrorInternal)
	}
	if result.RepositoryID() != request.RepositoryID() {
		return repository.Repository{}, repository.NewError(repository.ErrorIntegrityFailure, "register-repository", "store-result-mismatch", false, nil)
	}
	return result, nil
}

func (service *Service) GetRepository(ctx context.Context, query repository.RepositoryQuery) (repository.Repository, error) {
	if service == nil || ctx == nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInvalidInput, "get-repository", "invalid-request", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "get-repository", "context-ended", false, err)
	}
	result, err := service.store.Get(ctx, query.Scope(), query.RepositoryID())
	if err != nil {
		return repository.Repository{}, mapDependencyError(err, "get-repository", "store-failed", repository.ErrorInternal)
	}
	if result.RepositoryID() != query.RepositoryID() {
		return repository.Repository{}, repository.NewError(repository.ErrorIntegrityFailure, "get-repository", "store-result-mismatch", false, nil)
	}
	return result, nil
}

func (service *Service) ListRepositories(ctx context.Context, request repository.RepositoryListRequest) (repository.RepositoryPage, error) {
	if service == nil || ctx == nil {
		return repository.RepositoryPage{}, repository.NewError(repository.ErrorInvalidInput, "list-repositories", "invalid-request", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.RepositoryPage{}, repository.NewError(repository.ErrorInternal, "list-repositories", "context-ended", false, err)
	}
	result, err := service.store.List(ctx, request.Scope(), request.PageSize(), request.Cursor())
	if err != nil {
		return repository.RepositoryPage{}, mapDependencyError(err, "list-repositories", "store-failed", repository.ErrorInternal)
	}
	return repository.NewRepositoryPage(result.Items(), result.NextCursor())
}

func (service *Service) ArchiveRepository(ctx context.Context, request repository.ArchiveRepositoryRequest) (repository.Repository, error) {
	if service == nil || ctx == nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInvalidInput, "archive-repository", "invalid-request", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "archive-repository", "context-ended", false, err)
	}
	now := service.clock.Now().UTC()
	if now.IsZero() {
		return repository.Repository{}, repository.NewError(repository.ErrorInternal, "archive-repository", "invalid-clock", false, nil)
	}
	fingerprint := archiveFingerprint(request)
	result, err := service.store.Archive(ctx, newArchiveCommand(request.Scope(), request.RequestID(), request.RepositoryID(), fingerprint, now))
	if err != nil {
		return repository.Repository{}, mapDependencyError(err, "archive-repository", "store-failed", repository.ErrorInternal)
	}
	if result.RepositoryID() != request.RepositoryID() || result.State() != repository.RepositoryArchived {
		return repository.Repository{}, repository.NewError(repository.ErrorIntegrityFailure, "archive-repository", "store-result-mismatch", false, nil)
	}
	return result, nil
}

func registerFingerprint(request repository.RegisterRepositoryRequest, proof SourceProof) repository.Digest {
	fields := []string{string(request.RepositoryID()), request.DisplayName(), proof.Kind(), proof.FingerprintScheme(), proof.Fingerprint().String(), proof.Revision()}
	return fingerprint(registerFingerprintDomain, fields...)
}

func archiveFingerprint(request repository.ArchiveRepositoryRequest) repository.Digest {
	return fingerprint(archiveFingerprintDomain, string(request.RepositoryID()))
}

func fingerprint(domain string, fields ...string) repository.Digest {
	value := append([]byte(nil), domain...)
	var length [4]byte
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		value = append(value, length[:]...)
		value = append(value, field...)
	}
	return sha256.Sum256(value)
}

func validName(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLower(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '/' || character == '.') {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
