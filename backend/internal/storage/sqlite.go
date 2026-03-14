package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/atroo/rancher-ai-assistant/backend/internal/embedding"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database for persistent session and memory storage.
type DB struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	s := &DB{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			messages TEXT NOT NULL DEFAULT '[]',
			vfs_data TEXT NOT NULL DEFAULT '{}',
			tokens_used INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			last_used_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			summary TEXT NOT NULL,
			details TEXT NOT NULL DEFAULT '',
			severity TEXT NOT NULL DEFAULT 'info',
			namespace TEXT NOT NULL DEFAULT '',
			resource TEXT NOT NULL DEFAULT '',
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			occurrence_count INTEGER NOT NULL DEFAULT 1,
			resolved INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_category ON memory(category)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_namespace ON memory(namespace)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_last_seen ON memory(last_seen_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_resolved ON memory(resolved)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_last_used ON sessions(last_used_at)`,
		// v2: add embedding column for semantic search
		`ALTER TABLE memory ADD COLUMN embedding TEXT NOT NULL DEFAULT ''`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			// ALTER TABLE ADD COLUMN fails if column already exists — that's fine
			if !isColumnExistsError(err) {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
	}

	return nil
}

// Close closes the database.
func (s *DB) Close() error {
	return s.db.Close()
}

// --- Session persistence ---

// SessionRow is the serialized form of a session.
type SessionRow struct {
	ID         string    `json:"id"`
	Messages   string    `json:"messages"`    // JSON-encoded []Message
	VFSData    string    `json:"vfs_data"`    // JSON-encoded VFS state
	TokensUsed int       `json:"tokens_used"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// SaveSession upserts a session.
func (s *DB) SaveSession(row *SessionRow) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, messages, vfs_data, tokens_used, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			messages = excluded.messages,
			vfs_data = excluded.vfs_data,
			tokens_used = excluded.tokens_used,
			last_used_at = excluded.last_used_at
	`, row.ID, row.Messages, row.VFSData, row.TokensUsed,
		row.CreatedAt.Format(time.RFC3339),
		row.LastUsedAt.Format(time.RFC3339))
	return err
}

// LoadSession retrieves a session by ID.
func (s *DB) LoadSession(id string) (*SessionRow, error) {
	row := &SessionRow{ID: id}
	var createdAt, lastUsedAt string

	err := s.db.QueryRow(
		`SELECT messages, vfs_data, tokens_used, created_at, last_used_at FROM sessions WHERE id = ?`,
		id,
	).Scan(&row.Messages, &row.VFSData, &row.TokensUsed, &createdAt, &lastUsedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	row.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	row.LastUsedAt, _ = time.Parse(time.RFC3339, lastUsedAt)
	return row, nil
}

// DeleteSession removes a session.
func (s *DB) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteExpiredSessions removes sessions older than the given TTL.
func (s *DB) DeleteExpiredSessions(ttl time.Duration) (int64, error) {
	cutoff := time.Now().Add(-ttl).Format(time.RFC3339)
	result, err := s.db.Exec(`DELETE FROM sessions WHERE last_used_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// --- Long-term memory ---

// MemoryEntry represents a stored observation about the cluster.
type MemoryEntry struct {
	ID              int       `json:"id"`
	Category        string    `json:"category"`         // "error_pattern", "performance", "scaling", "security", "config_drift"
	Summary         string    `json:"summary"`          // One-line description
	Details         string    `json:"details"`          // Longer explanation, evidence
	Severity        string    `json:"severity"`         // "info", "warning", "critical"
	Namespace       string    `json:"namespace"`        // Scoping
	Resource        string    `json:"resource"`         // e.g., "deployment/payments"
	FirstSeenAt     time.Time `json:"firstSeenAt"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
	OccurrenceCount int       `json:"occurrenceCount"`
	Resolved        bool      `json:"resolved"`
	Embedding       []float32 `json:"-"`                // vector embedding of summary (not serialized to JSON)
	Similarity      float64   `json:"similarity,omitempty"` // populated by semantic search
}

// StoreMemory inserts or updates a memory entry.
// If a similar entry exists (same category + summary), it increments occurrence count.
func (s *DB) StoreMemory(entry *MemoryEntry) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	embeddingJSON := encodeEmbedding(entry.Embedding)

	// Check for existing similar entry
	var existingID int
	err := s.db.QueryRow(
		`SELECT id FROM memory WHERE category = ? AND summary = ? AND resolved = 0 LIMIT 1`,
		entry.Category, entry.Summary,
	).Scan(&existingID)

	if err == nil {
		// Update existing — also update embedding if provided
		q := `UPDATE memory SET details = ?, severity = ?, last_seen_at = ?, occurrence_count = occurrence_count + 1`
		args := []any{entry.Details, entry.Severity, now}
		if len(entry.Embedding) > 0 {
			q += `, embedding = ?`
			args = append(args, embeddingJSON)
		}
		q += ` WHERE id = ?`
		args = append(args, existingID)
		_, err := s.db.Exec(q, args...)
		return int64(existingID), err
	}

	// Insert new
	if entry.FirstSeenAt.IsZero() {
		entry.FirstSeenAt = time.Now()
	}
	if entry.LastSeenAt.IsZero() {
		entry.LastSeenAt = time.Now()
	}

	result, err := s.db.Exec(`
		INSERT INTO memory (category, summary, details, severity, namespace, resource, first_seen_at, last_seen_at, occurrence_count, resolved, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.Category, entry.Summary, entry.Details, entry.Severity,
		entry.Namespace, entry.Resource,
		entry.FirstSeenAt.Format(time.RFC3339),
		entry.LastSeenAt.Format(time.RFC3339),
		entry.OccurrenceCount, entry.Resolved, embeddingJSON)

	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// SearchMemory queries memory entries by text search and optional filters.
func (s *DB) SearchMemory(query string, category string, namespace string, includeResolved bool, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	where := "1=1"
	args := []any{}

	if query != "" {
		where += " AND (summary LIKE ? OR details LIKE ?)"
		pattern := "%" + query + "%"
		args = append(args, pattern, pattern)
	}
	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}
	if namespace != "" {
		where += " AND namespace = ?"
		args = append(args, namespace)
	}
	if !includeResolved {
		where += " AND resolved = 0"
	}

	args = append(args, limit)

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, category, summary, details, severity, namespace, resource,
		       first_seen_at, last_seen_at, occurrence_count, resolved
		FROM memory
		WHERE %s
		ORDER BY last_seen_at DESC
		LIMIT ?
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var firstSeen, lastSeen string
		var resolved int

		if err := rows.Scan(&e.ID, &e.Category, &e.Summary, &e.Details, &e.Severity,
			&e.Namespace, &e.Resource, &firstSeen, &lastSeen, &e.OccurrenceCount, &resolved); err != nil {
			return nil, err
		}

		e.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeen)
		e.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeen)
		e.Resolved = resolved != 0
		entries = append(entries, e)
	}

	return entries, nil
}

// ResolveMemory marks a memory entry as resolved.
func (s *DB) ResolveMemory(id int) error {
	_, err := s.db.Exec(`UPDATE memory SET resolved = 1 WHERE id = ?`, id)
	return err
}

// RecentMemorySummary returns the most recent unresolved entries for system prompt injection.
func (s *DB) RecentMemorySummary(limit int) ([]MemoryEntry, error) {
	return s.SearchMemory("", "", "", false, limit)
}

// MemoryStats returns counts by category.
func (s *DB) MemoryStats() (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT category, COUNT(*) FROM memory WHERE resolved = 0 GROUP BY category
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var cat string
		var count int
		if err := rows.Scan(&cat, &count); err != nil {
			return nil, err
		}
		stats[cat] = count
	}
	return stats, nil
}

// SemanticSearchMemory performs vector similarity search on memory entries.
// Loads all unresolved entries with embeddings, computes cosine similarity against
// the query vector, and returns the top results above the threshold.
// For the expected scale (hundreds to low thousands of entries), brute-force is fine.
func (s *DB) SemanticSearchMemory(queryVec []float32, category string, namespace string, includeResolved bool, limit int, threshold float64) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if threshold <= 0 {
		threshold = 0.3
	}

	where := "embedding != ''"
	args := []any{}

	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}
	if namespace != "" {
		where += " AND namespace = ?"
		args = append(args, namespace)
	}
	if !includeResolved {
		where += " AND resolved = 0"
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, category, summary, details, severity, namespace, resource,
		       first_seen_at, last_seen_at, occurrence_count, resolved, embedding
		FROM memory
		WHERE %s
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		entry MemoryEntry
		score float64
	}
	var candidates []scored

	for rows.Next() {
		var e MemoryEntry
		var firstSeen, lastSeen, embStr string
		var resolved int

		if err := rows.Scan(&e.ID, &e.Category, &e.Summary, &e.Details, &e.Severity,
			&e.Namespace, &e.Resource, &firstSeen, &lastSeen, &e.OccurrenceCount, &resolved, &embStr); err != nil {
			return nil, err
		}

		e.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeen)
		e.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeen)
		e.Resolved = resolved != 0
		e.Embedding = decodeEmbedding(embStr)

		if len(e.Embedding) == 0 {
			continue
		}

		sim := embedding.CosineSimilarity(queryVec, e.Embedding)
		if sim >= threshold {
			e.Similarity = sim
			candidates = append(candidates, scored{entry: e, score: sim})
		}
	}

	// Sort by similarity descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	entries := make([]MemoryEntry, len(candidates))
	for i, c := range candidates {
		c.entry.Embedding = nil // don't include raw vectors in results
		entries[i] = c.entry
	}
	return entries, nil
}

// DeleteMemory removes a memory entry by ID.
func (s *DB) DeleteMemory(id int) error {
	result, err := s.db.Exec(`DELETE FROM memory WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory entry %d not found", id)
	}
	return nil
}

// UnresolveMemory marks a memory entry as unresolved.
func (s *DB) UnresolveMemory(id int) error {
	_, err := s.db.Exec(`UPDATE memory SET resolved = 0 WHERE id = ?`, id)
	return err
}

// --- Embedding helpers ---

func encodeEmbedding(vec []float32) string {
	if len(vec) == 0 {
		return ""
	}
	data, _ := json.Marshal(vec)
	return string(data)
}

func decodeEmbedding(s string) []float32 {
	if s == "" {
		return nil
	}
	var vec []float32
	json.Unmarshal([]byte(s), &vec)
	return vec
}

func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite returns "duplicate column name: X" when ALTER TABLE ADD COLUMN
	// is called on an existing column
	msg := err.Error()
	return len(msg) > 0 && (contains(msg, "duplicate column") || contains(msg, "already exists"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MarshalJSON is a helper to encode any value to a JSON string.
func MarshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
