package agent

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
	"github.com/atroo/rancher-ai-assistant/backend/internal/vfs"
)

// SQLiteSessionStore persists sessions to SQLite while caching active
// sessions in memory for fast access.
type SQLiteSessionStore struct {
	db    *storage.DB
	cache sync.Map // map[string]*Session
	ttl   time.Duration
}

// NewSQLiteSessionStore creates a persistent session store.
func NewSQLiteSessionStore(db *storage.DB, ttl time.Duration) *SQLiteSessionStore {
	store := &SQLiteSessionStore{db: db, ttl: ttl}
	go store.evictLoop()
	return store
}

func (s *SQLiteSessionStore) Get(id string) (*Session, bool) {
	// Check cache first
	if v, ok := s.cache.Load(id); ok {
		sess := v.(*Session)
		if time.Since(sess.LastUsed) <= s.ttl {
			return sess, true
		}
		s.cache.Delete(id)
	}

	// Try loading from SQLite
	row, err := s.db.LoadSession(id)
	if err != nil || row == nil {
		return nil, false
	}

	if time.Since(row.LastUsedAt) > s.ttl {
		s.db.DeleteSession(id)
		return nil, false
	}

	// Reconstruct session
	sess := s.deserializeSession(row)
	s.cache.Store(id, sess)
	return sess, true
}

func (s *SQLiteSessionStore) GetOrCreate(id string, tokenBudget int) *Session {
	if sess, ok := s.Get(id); ok {
		return sess
	}

	sess := NewSession(id, tokenBudget)
	s.cache.Store(id, sess)
	s.persistSession(sess)
	return sess
}

func (s *SQLiteSessionStore) Delete(id string) {
	s.cache.Delete(id)
	s.db.DeleteSession(id)
}

// Persist saves the current session state to SQLite.
// Should be called after each agent turn completes.
func (s *SQLiteSessionStore) Persist(sess *Session) {
	s.persistSession(sess)
}

func (s *SQLiteSessionStore) persistSession(sess *Session) {
	messagesJSON, _ := json.Marshal(sess.History())
	vfsJSON, _ := json.Marshal(sess.VFS.Export())

	row := &storage.SessionRow{
		ID:         sess.ID,
		Messages:   string(messagesJSON),
		VFSData:    string(vfsJSON),
		TokensUsed: sess.Budget.Used(),
		CreatedAt:  sess.CreatedAt,
		LastUsedAt: sess.LastUsed,
	}

	if err := s.db.SaveSession(row); err != nil {
		slog.Error("failed to persist session", "session", sess.ID, "error", err)
	}
}

func (s *SQLiteSessionStore) deserializeSession(row *storage.SessionRow) *Session {
	sess := &Session{
		ID:        row.ID,
		VFS:       vfs.New(),
		Budget:    NewTokenBudget(0),
		CreatedAt: row.CreatedAt,
		LastUsed:  row.LastUsedAt,
	}

	// Restore messages
	var messages []Message
	if err := json.Unmarshal([]byte(row.Messages), &messages); err == nil {
		sess.messages = messages
	}

	// Restore VFS
	var vfsData map[string]vfs.ExportedFile
	if err := json.Unmarshal([]byte(row.VFSData), &vfsData); err == nil {
		sess.VFS.Import(vfsData)
	}

	// Restore token usage
	sess.Budget.Record(row.TokensUsed)

	return sess
}

func (s *SQLiteSessionStore) evictLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Evict from cache
		s.cache.Range(func(key, value any) bool {
			sess := value.(*Session)
			if time.Since(sess.LastUsed) > s.ttl {
				s.cache.Delete(key)
			}
			return true
		})

		// Evict from SQLite
		count, err := s.db.DeleteExpiredSessions(s.ttl)
		if err != nil {
			slog.Error("failed to evict expired sessions", "error", err)
		} else if count > 0 {
			slog.Info("evicted expired sessions", "count", count)
		}
	}
}
