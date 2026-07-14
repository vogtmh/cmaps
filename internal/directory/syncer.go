package directory

import (
	"log/slog"
	"sync"
	"time"

	"companymaps/internal/progress"
	"companymaps/internal/store"
)

// Syncer owns the directory-sync engine state for the LDAP and EntraID
// sources: progress trackers, the most recent sync diagnostics and the next
// scheduled run times surfaced in the admin Sync tab.
type Syncer struct {
	DB *store.DB

	// Log is the structured logger for sync diagnostics; nil falls back to the
	// process default logger.
	Log *slog.Logger

	// AvatarDir is the filesystem path of the avatar cache, used to stamp
	// HasAvatar onto mirrored users.
	AvatarDir string

	// DemoDirectory regenerates the bundled demo employees for the built-in
	// demo source; EnsureDemoAvatars materialises their avatar files for the
	// current identifier mode. Both are injected by the composition root
	// because the sample data lives in the web/app layer.
	DemoDirectory     func() []store.DirectoryUser
	EnsureDemoAvatars func([]store.DirectoryUser)

	// LdapProg tracks a manual AD sync; EntraProg a manual EntraID sync.
	LdapProg  progress.Progress
	EntraProg progress.Progress

	// syncDebugMu guards syncDebug, the diagnostics of the most recent AD sync.
	syncDebugMu sync.Mutex
	syncDebug   ADSyncDebug

	// nextSyncMu guards the wall-clock times of the next scheduled automatic
	// syncs. In-memory only (they reset on restart, which is correct since the
	// schedulers re-arm on boot).
	nextSyncMu    sync.Mutex
	nextLdapSync  time.Time
	nextEntraSync time.Time
}

// logger returns the configured structured logger, or the process default.
func (s *Syncer) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// SetSyncDebug stores the most recent sync diagnostics (concurrency-safe).
func (s *Syncer) SetSyncDebug(d ADSyncDebug) {
	s.syncDebugMu.Lock()
	s.syncDebug = d
	s.syncDebugMu.Unlock()
}

// SyncDebug returns the most recent sync diagnostics (concurrency-safe).
func (s *Syncer) SyncDebug() ADSyncDebug {
	s.syncDebugMu.Lock()
	defer s.syncDebugMu.Unlock()
	return s.syncDebug
}

// SetNextLdapSync records the next scheduled AD sync time.
func (s *Syncer) SetNextLdapSync(t time.Time) {
	s.nextSyncMu.Lock()
	s.nextLdapSync = t
	s.nextSyncMu.Unlock()
}

// NextLdapSync returns the next scheduled AD sync time.
func (s *Syncer) NextLdapSync() time.Time {
	s.nextSyncMu.Lock()
	defer s.nextSyncMu.Unlock()
	return s.nextLdapSync
}

// SetNextEntraSync records the next scheduled EntraID sync time.
func (s *Syncer) SetNextEntraSync(t time.Time) {
	s.nextSyncMu.Lock()
	s.nextEntraSync = t
	s.nextSyncMu.Unlock()
}

// NextEntraSync returns the next scheduled EntraID sync time.
func (s *Syncer) NextEntraSync() time.Time {
	s.nextSyncMu.Lock()
	defer s.nextSyncMu.Unlock()
	return s.nextEntraSync
}
