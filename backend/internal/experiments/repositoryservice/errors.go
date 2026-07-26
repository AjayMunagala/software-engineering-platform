package repositoryservice

import "errors"

var (
	ErrInvalidConfig        = errors.New("invalid repository service spike configuration")
	ErrInvalidIdentity      = errors.New("invalid artifact identity input")
	ErrArtifactTooLarge     = errors.New("artifact exceeds configured limit")
	ErrArtifactClosed       = errors.New("materialized artifact is closed")
	ErrArtifactIntegrity    = errors.New("artifact integrity verification failed")
	ErrForbiddenSourceValue = errors.New("durable artifact contains a forbidden source value")
	ErrContextRequired      = errors.New("context is required")
	ErrEncodeRequired       = errors.New("artifact encoder is required")
	ErrFlightKeyRequired    = errors.New("single-flight key is required")
	ErrFlightFuncRequired   = errors.New("single-flight function is required")
	ErrPublicationAmbiguous = errors.New("publication outcome is ambiguous")
)
