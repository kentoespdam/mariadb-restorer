package base

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
)

// ProfileStoreCloser wraps a ProfileStore with a Close method.
type ProfileStoreCloser struct {
	*credentialvault.ProfileStore
	db *sql.DB
}

// Close shuts down the underlying database connection.
func (c *ProfileStoreCloser) Close() error {
	return c.db.Close()
}

// OpenProfileStore opens or creates the profile store at the given db path.
func OpenProfileStore(dbPath string) (*ProfileStoreCloser, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	for _, p := range []string{
		"PRAGMA synchronous=FULL",
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	store, err := credentialvault.NewProfileStore(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init profile store: %w", err)
	}
	return &ProfileStoreCloser{ProfileStore: store, db: db}, nil
}
