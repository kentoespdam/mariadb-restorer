// Package credentialvault provides encrypted storage for MariaDB connection passwords.
package credentialvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen    = 16
	nonceLen   = 12 // AES-GCM standard 96-bit nonce
	keyLen     = 32 // AES-256
	argonTime  = 3
	argonMem   = 64 * 1024 // 64 MiB
	argonThrds = 4
)

// PHCString returns the PHC-formatted string for Argon2id parameters.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>
func PHCString(salt []byte) string {
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%x", argonMem, argonTime, argonThrds, salt)
}

// DeriveKey derives a 32-byte AES-256 key from a passphrase and salt using Argon2id.
func DeriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMem, argonThrds, keyLen)
}

// GenerateSalt generates a random 16-byte salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// Vault handles AES-256-GCM seal/unseal operations for profile passwords.
type Vault struct {
	key []byte // KEK derived from Master Passphrase
}

// NewVault creates a vault from a derived key.
func NewVault(key []byte) *Vault {
	return &Vault{key: key}
}

// Seal encrypts plaintext with AES-256-GCM using the vault key and AAD.
// Returns nonce ‖ ciphertext ‖ tag.
func (v *Vault) Seal(plaintext []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	return append(nonce, ciphertext...), nil
}

// Open decrypts ciphertext with AES-256-GCM. Expects nonce ‖ ciphertext ‖ tag.
func (v *Vault) Open(data []byte, aad []byte) ([]byte, error) {
	if len(data) < nonceLen {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := data[:nonceLen]
	ciphertext := data[nonceLen:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err) // AAD mismatch or wrong key
	}
	return plaintext, nil
}
