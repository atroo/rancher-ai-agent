package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// MemoryHandler serves REST endpoints for managing long-term memory entries.
type MemoryHandler struct {
	db *storage.DB
}

func NewMemoryHandler(db *storage.DB) *MemoryHandler {
	return &MemoryHandler{db: db}
}

func (h *MemoryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/memories", h.handleList)
	mux.HandleFunc("GET /api/v1/memories/stats", h.handleStats)
	mux.HandleFunc("DELETE /api/v1/memories/{id}", h.handleDelete)
	mux.HandleFunc("PATCH /api/v1/memories/{id}", h.handleUpdate)
}

// handleList returns memory entries with optional query params:
//   ?category=error_pattern&namespace=default&resolved=true&q=search+text&limit=50
func (h *MemoryHandler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	category := q.Get("category")
	namespace := q.Get("namespace")
	search := q.Get("q")
	includeResolved := q.Get("resolved") == "true"

	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}

	entries, err := h.db.SearchMemory(search, category, namespace, includeResolved, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if entries == nil {
		entries = []storage.MemoryEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// handleStats returns memory counts by category.
func (h *MemoryHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.MemoryStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// handleDelete removes a memory entry by ID.
func (h *MemoryHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.db.DeleteMemory(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleUpdate patches a memory entry. Supports {"resolved": true/false}.
func (h *MemoryHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var patch struct {
		Resolved *bool `json:"resolved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if patch.Resolved != nil {
		if *patch.Resolved {
			err = h.db.ResolveMemory(id)
		} else {
			err = h.db.UnresolveMemory(id)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
