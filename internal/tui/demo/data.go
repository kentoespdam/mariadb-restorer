// Package demo provides synthetic data for the TUI demo mode.
// In demo mode, all data is in-memory — no SQLite or Data Directory access.
package demo

import (
	"time"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
)

// SyntheticProfiles returns pre-populated profiles for demo mode.
func SyntheticProfiles() []*credentialvault.Profile {
	return []*credentialvault.Profile{
		{
			Name:           "staging",
			Host:           "staging-db.example.com",
			Port:           3306,
			User:           "app_user",
			Database:       "app_staging",
			SealedPassword: []byte{1, 2, 3, 4}, // non-nil => vaulted indicator
		},
		{
			Name:     "prod",
			Host:     "prod-db.example.com",
			Port:     3306,
			User:     "admin",
			Database: "app_prod",
		},
		{
			Name:     "dev",
			Host:     "localhost",
			Port:     3307,
			User:     "root",
			Database: "app_dev",
		},
		{
			Name:     "analytics",
			Host:     "analytics-db.example.com",
			Port:     3306,
			User:     "etl_user",
			Database: "analytics",
		},
	}
}

// SyntheticCheckpoints returns fake restore history for demo mode.
// One completed, one resumable, one failed.
func SyntheticCheckpoints() []*restoreengine.Checkpoint {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)

	return []*restoreengine.Checkpoint{
		{
			DumpPath:       "/backups/prod-2026-07-13.sql.gz",
			DumpSizeBytes:  1024 * 1024 * 1024 * 5, // 5 GB
			DumpIdentity:   "a1b2c3d4-5368709120",
			ByteOffset:     1024 * 1024 * 1024 * 5, // 5 GB (completed)
			StatementsDone: 45231,
			UpdatedAt:      twoHoursAgo,
		},
		{
			DumpPath:       "/backups/staging-2026-07-14.sql.gz",
			DumpSizeBytes:  1024 * 1024 * 500, // 500 MB
			DumpIdentity:   "e5f6g7h8-524288000",
			ByteOffset:     1024 * 1024 * 300, // 300 MB (resumable)
			StatementsDone: 12894,
			UpdatedAt:      oneHourAgo,
		},
		{
			DumpPath:       "/backups/analytics-2026-07-12.sql.gz",
			DumpSizeBytes:  1024 * 1024 * 200, // 200 MB
			DumpIdentity:   "i9j0k1l2-209715200",
			ByteOffset:     85000000, // 85 MB (resumable, earlier failure)
			StatementsDone: 3421,
			UpdatedAt:      now.Add(-3 * time.Hour),
		},
	}
}

// ProgressSequence returns a sequence of ProgressMsg values that simulate a
// restore progressing from start to finish. Returns 20 tick events over ~5 GB.
type DemoProgressTick struct {
	ByteOffset     int64
	StatementsDone int64
	BatchCount     int64
}

// ProgressSequence generates simulated progress ticks.
func ProgressSequence(demoBytes int64) []DemoProgressTick {
	const ticks = 20
	size := demoBytes
	if size <= 0 {
		size = 5 * 1024 * 1024 * 1024 // 5 GB
	}
	step := size / ticks
	var seq []DemoProgressTick
	for i := 0; i < ticks; i++ {
		seq = append(seq, DemoProgressTick{
			ByteOffset:     step * int64(i+1),
			StatementsDone: int64((i + 1) * 1500),
			BatchCount:     int64(i + 1),
		})
	}
	return seq
}
