package main

import (
	"fmt"
	"sync"
	"time"
)

// syncProgress tracks a long-running background sync (Robin or LDAP) so the
// admin Sync tab can show a live, determinate progress bar plus a streaming log.
// It is safe for concurrent use: the worker goroutine writes while the HTTP
// poll handler reads snapshots.
type syncProgress struct {
	mu         sync.Mutex
	running    bool
	done       bool
	cur        int
	total      int
	stage      string
	log        []string
	summary    string
	errMsg     string
	startedAt  time.Time
	finishedAt time.Time
}

// start marks the job as running and clears any previous state. It returns false
// if a job is already in flight (the caller should not launch a second worker).
func (p *syncProgress) start(total int, stage string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return false
	}
	p.running = true
	p.done = false
	p.cur = 0
	p.total = total
	p.stage = stage
	p.log = nil
	p.summary = ""
	p.errMsg = ""
	p.startedAt = time.Now()
	p.finishedAt = time.Time{}
	return true
}

// setTotal updates the expected number of steps once it is known (e.g. after the
// Robin room list has been fetched).
func (p *syncProgress) setTotal(total int) {
	p.mu.Lock()
	p.total = total
	p.mu.Unlock()
}

// logf appends a timestamped line to the live log.
func (p *syncProgress) logf(format string, a ...interface{}) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, a...)
	p.mu.Lock()
	p.log = append(p.log, line)
	p.mu.Unlock()
}

// stage updates the current activity label without advancing the counter.
func (p *syncProgress) setStage(stage string) {
	p.mu.Lock()
	p.stage = stage
	p.mu.Unlock()
}

// step advances the progress counter by one and optionally updates the stage.
func (p *syncProgress) step(stage string) {
	p.mu.Lock()
	p.cur++
	if stage != "" {
		p.stage = stage
	}
	p.mu.Unlock()
}

// finish records the terminal state (success summary or error message).
func (p *syncProgress) finish(summary, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	p.done = true
	p.stage = ""
	p.summary = summary
	p.errMsg = errMsg
	p.finishedAt = time.Now()
}

// snapshot returns a JSON-serializable copy of the current progress.
func (p *syncProgress) snapshot() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := 0.0
	if !p.startedAt.IsZero() {
		end := p.finishedAt
		if end.IsZero() {
			end = time.Now()
		}
		elapsed = end.Sub(p.startedAt).Seconds()
	}
	logCopy := make([]string, len(p.log))
	copy(logCopy, p.log)
	return map[string]interface{}{
		"running": p.running,
		"done":    p.done,
		"cur":     p.cur,
		"total":   p.total,
		"stage":   p.stage,
		"log":     logCopy,
		"summary": p.summary,
		"error":   p.errMsg,
		"elapsed": elapsed,
	}
}
