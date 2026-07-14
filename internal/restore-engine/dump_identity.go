package restoreengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const identityHeadBytes = 4096 // bytes from the start to hash

// ComputeIdentity computes a fast fingerprint of a dump file:
// SHA-256 of the first 4KB + total file size.
// This is NOT a full-file checksum — it detects regeneration and swap,
// but NOT same-size in-place content edits.
func ComputeIdentity(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open dump: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("stat dump: %w", err)
	}
	size := fi.Size()

	h := sha256.New()
	if _, err := io.CopyN(h, f, int64(identityHeadBytes)); err != nil && err != io.EOF {
		return "", 0, fmt.Errorf("read head: %w", err)
	}
	hash := hex.EncodeToString(h.Sum(nil))

	identity := fmt.Sprintf("%s-%d", hash[:16], size)
	return identity, size, nil
}

// IdentityPrefix returns a short human-readable prefix of the identity.
func IdentityPrefix(identity string) string {
	if len(identity) < 8 {
		return identity
	}
	return identity[:7]
}
