package agent

import (
	"sync"
	"time"
)

// SessionStore manages conversation sessions.
type SessionStore interface {
	Get(id string) (*Session, bool)
	GetOrCreate(id string, tokenBudget int) *Session
	Delete(id string)
	// Persist saves session state to durable storage. No-op for in-memory stores.
	Persist(sess *Session)
}

// MemorySessionStore is an in-memory session store with TTL-based eviction.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewMemorySessionStore creates a store that evicts sessions after ttl of inactivity.
func NewMemorySessionStore(ttl time.Duration) *MemorySessionStore {
	store := &MemorySessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}

	// Background eviction every minute
	go store.evictLoop()

	return store
}

func (s *MemorySessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}

	// Check expiry
	if time.Since(sess.LastUsed) > s.ttl {
		return nil, false
	}

	return sess, true
}

func (s *MemorySessionStore) GetOrCreate(id string, tokenBudget int) *Session {
	// Try read path first
	if sess, ok := s.Get(id); ok {
		return sess
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if sess, ok := s.sessions[id]; ok && time.Since(sess.LastUsed) <= s.ttl {
		return sess
	}

	sess := NewSession(id, tokenBudget)
	s.sessions[id] = sess
	return sess
}

func (s *MemorySessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *MemorySessionStore) Persist(_ *Session) {
	// No-op for in-memory store
}

func (s *MemorySessionStore) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for id, sess := range s.sessions {
			if time.Since(sess.LastUsed) > s.ttl {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}
