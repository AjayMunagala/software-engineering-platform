package postgres

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

func digestParts(parts ...string) []byte {
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	return hash.Sum(nil)
}

func digestBytes(value persistence.Digest) []byte {
	return append([]byte(nil), value[:]...)
}

func parseDigest(value []byte) (persistence.Digest, error) {
	if len(value) != sha256.Size {
		return persistence.Digest{}, fmt.Errorf("digest length")
	}
	var digest persistence.Digest
	copy(digest[:], value)
	return digest, nil
}

func validateUUID(value string) error {
	var identifier pgtype.UUID
	if err := identifier.Scan(value); err != nil || !identifier.Valid {
		return fmt.Errorf("UUID")
	}
	return nil
}

func correlationUUID(requestID persistence.RequestID) string {
	digest := sha256.Sum256([]byte(requestID))
	digest[6] = (digest[6] & 0x0f) | 0x50
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

func encodeCursor(offset int) persistence.Cursor {
	if offset <= 0 {
		return ""
	}
	return persistence.Cursor(base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset))))
}

func decodeCursor(cursor persistence.Cursor) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(cursor))
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("cursor")
	}
	return offset, nil
}
