package audit

import (
	"sync"
	"time"
)

const defaultMaxSize = 2000

// Entry records a single admin action.
type Entry struct {
	Time       time.Time `json:"time"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceId,omitempty"`
	User       string    `json:"user,omitempty"`
	RemoteAddr string    `json:"remoteAddr,omitempty"`
	Details    any       `json:"details,omitempty"`
}

// Log is an in-memory ring buffer of admin audit entries.
type Log struct {
	mu      sync.RWMutex
	entries []Entry
	maxSize int
}

func New(maxSize int) *Log {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	return &Log{maxSize: maxSize}
}

func (l *Log) Record(e Entry) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	l.mu.Lock()
	if len(l.entries) >= l.maxSize {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, e)
	l.mu.Unlock()
}

// List returns the most recent n entries, newest-last.
func (l *Log) List(limit int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := len(l.entries)
	if limit <= 0 || limit > n {
		limit = n
	}
	result := make([]Entry, limit)
	copy(result, l.entries[n-limit:])
	return result
}
