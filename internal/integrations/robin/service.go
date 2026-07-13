// Package robin implements the Robin Powered workplace integration: meeting
// room status, live desk occupancy, location discovery and the diagnostic
// desk-data dump, plus their background schedulers.
package robin

import (
	"sync"
	"time"

	"strings"

	"companymaps/internal/progress"
	"companymaps/internal/store"
)

// Service owns every piece of Robin state: the store handle, the progress
// trackers polled by the admin Sync tab, the cached diagnostic dump and the
// next scheduled sync time.
type Service struct {
	DB *store.DB

	// Prog tracks the background meeting sync; DeskProg the desk-data
	// diagnostic; SuggestProg the strip-pattern suggestion scan.
	Prog        progress.Progress
	DeskProg    progress.Progress
	SuggestProg progress.Progress

	// suggestMu guards suggestResult, the outcome of the most recent completed
	// strip-pattern suggestion scan.
	suggestMu     sync.Mutex
	suggestResult []StripSuggestion

	// dumpMu guards the cached desk-data diagnostic bundle so the admin
	// "Download JSON bundle" button can export exactly what the last run
	// captured without re-hitting the Robin API.
	dumpMu    sync.Mutex
	dumpFiles []DumpFile
	dumpTime  string

	// nextSyncMu guards nextRobinSync, the wall-clock time of the next
	// scheduled automatic sync, surfaced in the admin Sync tab.
	nextSyncMu    sync.Mutex
	nextRobinSync time.Time
}

// setNextSync records the next scheduled sync time.
func (s *Service) setNextSync(dst *time.Time, t time.Time) {
	s.nextSyncMu.Lock()
	*dst = t
	s.nextSyncMu.Unlock()
}

// NextSync returns the next scheduled automatic sync time.
func (s *Service) NextSync() time.Time {
	s.nextSyncMu.Lock()
	defer s.nextSyncMu.Unlock()
	return s.nextRobinSync
}

// SuggestResult returns the suggestions from the most recent completed scan.
func (s *Service) SuggestResult() []StripSuggestion {
	s.suggestMu.Lock()
	defer s.suggestMu.Unlock()
	return s.suggestResult
}

// SetSuggestResult stores the outcome of a completed suggestion scan.
func (s *Service) SetSuggestResult(v []StripSuggestion) {
	s.suggestMu.Lock()
	s.suggestResult = v
	s.suggestMu.Unlock()
}

// Dump returns the cached diagnostic bundle and its capture time.
func (s *Service) Dump() ([]DumpFile, string) {
	s.dumpMu.Lock()
	defer s.dumpMu.Unlock()
	return s.dumpFiles, s.dumpTime
}

// SetDump stores a fresh diagnostic bundle with its capture time.
func (s *Service) SetDump(files []DumpFile, when string) {
	s.dumpMu.Lock()
	s.dumpFiles = files
	s.dumpTime = when
	s.dumpMu.Unlock()
}

// Configured reports whether a Robin access token has been saved.
func (s *Service) Configured() bool {
	return strings.TrimSpace(s.DB.GetRobinSetting("robintoken")) != ""
}
