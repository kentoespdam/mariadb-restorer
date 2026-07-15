package tuiprofiles

import (
	"fmt"
	"strconv"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (e *EditorScreen) save() error {
	name := e.inputs[fieldName].Value()
	if name == "" {
		return fmt.Errorf("name is required")
	}

	host := e.inputs[fieldHost].Value()
	portStr := e.inputs[fieldPort].Value()
	port := 3306
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port: %s", portStr)
		}
		if p < 1 || p > 65535 {
			return fmt.Errorf("port %d out of range (1-65535)", p)
		}
		port = p
	}

	profile := &credentialvault.Profile{
		Name:     name,
		Host:     host,
		Port:     port,
		User:     e.inputs[fieldUser].Value(),
		Database: e.inputs[fieldDatabase].Value(),
	}

	store, err := base.OpenProfileStore(e.dataDir + "/mariadb-restorer.db")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// If renaming, remove old row so Save INSERTs fresh.
	if e.origName != "" && e.origName != name {
		_ = store.Delete(e.origName)
	}

	// Save non-secret fields first.
	if err := store.Save(profile); err != nil {
		return err
	}

	// Handle password sealing.
	pwd := e.inputs[fieldPassword].Value()
	passphrase := e.inputs[fieldPassphrase].Value()

	switch {
	case e.clearPwd:
		// User pressed Ctrl-X — remove vaulted password.
		if err := store.SetPassword(name, nil); err != nil {
			return err
		}
		e.clearPwd = false
		return nil
	case pwd != "" && passphrase == "":
		return fmt.Errorf("master passphrase required to seal password")
	case pwd == "" && passphrase != "":
		return fmt.Errorf("enter a password or leave passphrase empty")
	case pwd != "" && passphrase != "":
		sealed, err := sealPassword(pwd, passphrase, name)
		if err != nil {
			return err
		}
		return store.SetPassword(name, sealed)
	}
	return nil
}

// sealPassword derives an AES-256 key from passphrase + random salt via Argon2id,
// then encrypts the password with AES-256-GCM. Returns salt ‖ nonce ‖ ciphertext.
func sealPassword(pwd, passphrase, profileName string) ([]byte, error) {
	salt, err := credentialvault.GenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	key := credentialvault.DeriveKey(passphrase, salt)
	vault := credentialvault.NewVault(key)
	aad := []byte(profileName + "|mariadb-restorer-v1")
	sealed, err := vault.Seal([]byte(pwd), aad)
	if err != nil {
		return nil, fmt.Errorf("seal password: %w", err)
	}
	// Layout: 16-byte salt ‖ nonce ‖ ciphertext ‖ tag
	return append(salt, sealed...), nil
}
