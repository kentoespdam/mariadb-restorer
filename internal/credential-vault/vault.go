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

// profileAADSuffix is the AAD suffix used when sealing/unsealing passwords
// (must match the suffix used in sealPassword in the profiles package).
const profileAADSuffix = "|mariadb-restorer-v1"

// minSealedLen is the minimum length of a sealed password blob:
// 16-byte salt + 12-byte nonce + 16-byte minimum ciphertext (1 byte + GCM tag).
const minSealedLen = 44

// UnsealPassword decrypts a sealed password blob using the vault master passphrase.
// The sealed format must match sealPassword in the profiles package:
// salt(16) ‖ nonce(12) ‖ ciphertext ‖ gcm_tag(16).
// AAD is profileName + "|" profileAADSuffix.
func UnsealPassword(sealed []byte, passphrase, profileName string) (string, error) {
	if len(sealed) < minSealedLen {
		return "", fmt.Errorf("sealed password too short: %d bytes", len(sealed))
	}
	salt := sealed[:saltLen]
	key := DeriveKey(passphrase, salt)
	vault := NewVault(key)
	aad := []byte(profileName + profileAADSuffix)
	plaintext, err := vault.Open(sealed[saltLen:], aad)
	if err != nil {
		return "", fmt.Errorf("unseal password: %w", err)
	}
	return string(plaintext), nil
}

const (
	saltLen    = 16
	nonceLen   = 12 // AES-GCM standard 96-bit nonce
	keyLen     = 32 // AES-256
	argonTime  = 3
	argonMem   = 64 * 1024 // 64 MiB
	argonThrds = 4
)

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
