package session

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
    "yourapp/internal/models"
)

// Store is an in-memory session store keyed by an opaque id.
// For production, consider a persistent/replicated store (e.g., Redis/DB).
type Store struct {
    mu   sync.RWMutex
    data map[string]models.Session
}

func NewStore() *Store {
    return &Store{data: make(map[string]models.Session)}
}

var DefaultStore = NewStore()

// Create stores the session and returns a new opaque id.
func (s *Store) Create(sess models.Session) string {
    id := uuid.NewString()
    s.mu.Lock()
    s.data[id] = sess
    s.mu.Unlock()
    return id
}

// Get returns the session for id if present and not expired.
func (s *Store) Get(id string) (models.Session, bool) {
    s.mu.RLock()
    sess, ok := s.data[id]
    s.mu.RUnlock()
    if !ok {
        return models.Session{}, false
    }
    if !sess.Expiry.IsZero() && sess.Expiry.Before(time.Now()) {
        // Expired; delete lazily
        s.mu.Lock()
        delete(s.data, id)
        s.mu.Unlock()
        return models.Session{}, false
    }
    return sess, true
}

// Delete removes a session by id.
func (s *Store) Delete(id string) {
    s.mu.Lock()
    delete(s.data, id)
    s.mu.Unlock()
}

// StartSweeper launches a background goroutine that periodically removes
// expired sessions from the store. It stops when ctx is done.
func (s *Store) StartSweeper(ctx context.Context, interval time.Duration) {
    if interval <= 0 {
        interval = 5 * time.Minute
    }
    ticker := time.NewTicker(interval)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                now := time.Now()
                s.mu.Lock()
                for k, v := range s.data {
                    if !v.Expiry.IsZero() && v.Expiry.Before(now) {
                        delete(s.data, k)
                    }
                }
                s.mu.Unlock()
            }
        }
    }()
}

// SessionEntry is a snapshot of a single session in the store.
type SessionEntry struct {
    ID      string
    Session models.Session
}

// List returns a snapshot of all sessions.
func (s *Store) List() []SessionEntry {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]SessionEntry, 0, len(s.data))
    for k, v := range s.data {
        out = append(out, SessionEntry{ID: k, Session: v})
    }
    return out
}
