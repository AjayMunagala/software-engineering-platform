package lifecycle

import (
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

// SourceProof is durable, path-free repository identity evidence.
type SourceProof struct {
	kind, scheme, revision string
	digest                 repository.Digest
}

func NewSourceProof(kind, scheme string, digest repository.Digest, revision string) (SourceProof, error) {
	if !validName(kind, 64) || !validName(scheme, 128) || digest.IsZero() || (revision != "" && !validText(revision, 1024)) {
		return SourceProof{}, repository.NewError(repository.ErrorInvalidInput, "new-source-proof", "invalid-source-proof", false, nil)
	}
	return SourceProof{kind: kind, scheme: scheme, digest: digest, revision: revision}, nil
}

func (proof SourceProof) Kind() string                   { return proof.kind }
func (proof SourceProof) FingerprintScheme() string      { return proof.scheme }
func (proof SourceProof) Fingerprint() repository.Digest { return proof.digest }
func (proof SourceProof) Revision() string               { return proof.revision }
func (proof SourceProof) IsZero() bool {
	return proof.kind == "" || proof.scheme == "" || proof.digest.IsZero()
}

// RegisterCommand is the immutable atomic store input for registration.
type RegisterCommand struct {
	scope       repository.Scope
	requestID   repository.RequestID
	fingerprint repository.Digest
	value       repository.Repository
}

func newRegisterCommand(scope repository.Scope, requestID repository.RequestID, fingerprint repository.Digest, value repository.Repository) RegisterCommand {
	return RegisterCommand{scope: scope, requestID: requestID, fingerprint: fingerprint, value: value}
}
func (command RegisterCommand) Scope() repository.Scope                { return command.scope }
func (command RegisterCommand) RequestID() repository.RequestID        { return command.requestID }
func (command RegisterCommand) MutationFingerprint() repository.Digest { return command.fingerprint }
func (command RegisterCommand) Repository() repository.Repository      { return command.value }

// ArchiveCommand is the immutable atomic store input for archival.
type ArchiveCommand struct {
	scope        repository.Scope
	requestID    repository.RequestID
	repositoryID repository.RepositoryID
	fingerprint  repository.Digest
	at           time.Time
}

func newArchiveCommand(scope repository.Scope, requestID repository.RequestID, repositoryID repository.RepositoryID, fingerprint repository.Digest, at time.Time) ArchiveCommand {
	return ArchiveCommand{scope: scope, requestID: requestID, repositoryID: repositoryID, fingerprint: fingerprint, at: at}
}
func (command ArchiveCommand) Scope() repository.Scope                { return command.scope }
func (command ArchiveCommand) RequestID() repository.RequestID        { return command.requestID }
func (command ArchiveCommand) RepositoryID() repository.RepositoryID  { return command.repositoryID }
func (command ArchiveCommand) MutationFingerprint() repository.Digest { return command.fingerprint }
func (command ArchiveCommand) At() time.Time                          { return command.at }

// RepositoryList is a detached store result.
type RepositoryList struct {
	items []repository.Repository
	next  repository.Cursor
}

func NewRepositoryList(items []repository.Repository, next repository.Cursor) (RepositoryList, error) {
	page, err := repository.NewRepositoryPage(items, next)
	if err != nil {
		return RepositoryList{}, err
	}
	return RepositoryList{items: page.Items(), next: page.NextCursor()}, nil
}
func (list RepositoryList) Items() []repository.Repository {
	return append([]repository.Repository(nil), list.items...)
}
func (list RepositoryList) NextCursor() repository.Cursor { return list.next }
