package credentialvault

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// Profile represents a named connection profile.
// Non-secret settings stored as-is; password sealed via Vault.
type Profile struct {
	Name           string
	Host           string
	Port           int
	User           string
	Database       string
	SealedPassword []byte // nil if no password saved; ciphertext otherwise
}

const profileSchema = `
CREATE TABLE IF NOT EXISTS profiles (
    name            TEXT PRIMARY KEY,
    host            TEXT NOT NULL DEFAULT 'localhost',
    port            INTEGER NOT NULL DEFAULT 3306,
    user            TEXT NOT NULL DEFAULT 'root',
    database_name   TEXT NOT NULL DEFAULT '',
    sealed_password BLOB
)`

// ProfileStore manages connection profile rows in SQLite.
type ProfileStore struct {
	db *sql.DB
}

// NewProfileStore opens or creates the profile table in the given SQLite database.
func NewProfileStore(db *sql.DB) (*ProfileStore, error) {
	if _, err := db.Exec(profileSchema); err != nil {
		return nil, fmt.Errorf("create profile schema: %w", err)
	}
	return &ProfileStore{db: db}, nil
}

// Get retrieves a profile by name. Returns nil, nil if not found.
func (s *ProfileStore) Get(name string) (*Profile, error) {
	row := s.db.QueryRow(`SELECT name, host, port, user, database_name, sealed_password
		FROM profiles WHERE name = ?`, name)

	var p Profile
	var sp sql.Null[[]byte]
	err := row.Scan(&p.Name, &p.Host, &p.Port, &p.User, &p.Database, &sp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	if sp.Valid {
		p.SealedPassword = sp.V
	}
	return &p, nil
}

// Save creates or updates a profile without touching sealed_password.
// Use SetPassword to set/update the vaulted password separately.
func (s *ProfileStore) Save(p *Profile) error {
	// First try UPDATE for non-secret fields only.
	res, err := s.db.Exec(`UPDATE profiles SET
		host = ?, port = ?, user = ?, database_name = ?
		WHERE name = ?`,
		p.Host, p.Port, p.User, p.Database, p.Name)
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil // updated existing profile
	}

	// Profile doesn't exist yet — INSERT with nil sealed_password.
	_, err = s.db.Exec(`INSERT INTO profiles (name, host, port, user, database_name, sealed_password)
		VALUES (?, ?, ?, ?, ?, NULL)`,
		p.Name, p.Host, p.Port, p.User, p.Database)
	if err != nil {
		return fmt.Errorf("create profile: %w", err)
	}
	return nil
}

// SetPassword seals a password into the profile's vault column.
func (s *ProfileStore) SetPassword(name string, sealed []byte) error {
	res, err := s.db.Exec(`UPDATE profiles SET sealed_password = ? WHERE name = ?`, sealed, name)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("profile %q not found", name)
	}
	return nil
}

// List returns all profiles without sealed passwords.
func (s *ProfileStore) List() ([]*Profile, error) {
	rows, err := s.db.Query(`SELECT name, host, port, user, database_name, sealed_password FROM profiles`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	var result []*Profile
	for rows.Next() {
		var p Profile
		var sp sql.Null[[]byte]
		if err := rows.Scan(&p.Name, &p.Host, &p.Port, &p.User, &p.Database, &sp); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		if sp.Valid {
			p.SealedPassword = sp.V
		}
		result = append(result, &p)
	}
	return result, rows.Err()
}

// Delete removes a profile. Does NOT require the Master Passphrase.
func (s *ProfileStore) Delete(name string) error {
	res, err := s.db.Exec("DELETE FROM profiles WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("profile %q not found", name)
	}
	return nil
}
