package vfs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Store is an in-memory virtual filesystem scoped to a single conversation.
// Tool results are stored here so only compact summaries enter the LLM context.
type Store struct {
	mu    sync.RWMutex
	files map[string]*File
	seq   int
}

// File represents a stored result in the virtual filesystem.
type File struct {
	Path        string `json:"path"`
	Source      string `json:"source"`      // tool name that created it
	Description string `json:"description"` // human-readable summary
	Size        int    `json:"size"`        // raw byte count
	LineCount   int    `json:"lineCount"`
	content     string // raw content (not serialized to LLM)
}

// ExportedFile is the serializable form of a File (includes content).
type ExportedFile struct {
	Path        string `json:"path"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// New creates an empty virtual filesystem.
func New() *Store {
	return &Store{
		files: make(map[string]*File),
	}
}

// Export serializes all files (including content) for persistence.
func (s *Store) Export() map[string]ExportedFile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]ExportedFile, len(s.files))
	for path, f := range s.files {
		out[path] = ExportedFile{
			Path:        f.Path,
			Source:      f.Source,
			Description: f.Description,
			Content:     f.content,
		}
	}
	return out
}

// Import restores files from a previously exported state.
func (s *Store) Import(data map[string]ExportedFile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for path, ef := range data {
		lineCount := strings.Count(ef.Content, "\n") + 1
		s.files[path] = &File{
			Path:        ef.Path,
			Source:      ef.Source,
			Description: ef.Description,
			Size:        len(ef.Content),
			LineCount:   lineCount,
			content:     ef.Content,
		}
		s.seq++ // keep seq moving to avoid path collisions
	}
}

// Write stores content under an auto-generated path and returns the file metadata.
func (s *Store) Write(source, description, content string) *File {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	path := fmt.Sprintf("/results/%s/%d.json", source, s.seq)

	lineCount := strings.Count(content, "\n") + 1

	f := &File{
		Path:        path,
		Source:      source,
		Description: description,
		Size:        len(content),
		LineCount:   lineCount,
		content:     content,
	}

	s.files[path] = f
	return f
}

// List returns metadata for all files.
func (s *Store) List() []*File {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*File, 0, len(s.files))
	for _, f := range s.files {
		result = append(result, f)
	}
	return result
}

// Read returns the content of a file, optionally limited by offset/limit (line-based).
func (s *Store) Read(path string, offset, limit int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.files[path]
	if !ok {
		return "", fmt.Errorf("file not found: %s", path)
	}

	lines := strings.Split(f.content, "\n")

	// Apply offset
	if offset > 0 {
		if offset >= len(lines) {
			return "", fmt.Errorf("offset %d exceeds line count %d", offset, len(lines))
		}
		lines = lines[offset:]
	}

	// Apply limit
	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	return strings.Join(lines, "\n"), nil
}

// Search searches file content using a regex pattern or plain substring.
// Returns matching lines with line numbers.
func (s *Store) Search(path, pattern string, maxResults int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.files[path]
	if !ok {
		return "", fmt.Errorf("file not found: %s", path)
	}

	if maxResults <= 0 {
		maxResults = 50
	}

	// Try regex first, fall back to substring
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Use plain substring match
		re = nil
	}

	lines := strings.Split(f.content, "\n")
	var matches []string
	for i, line := range lines {
		matched := false
		if re != nil {
			matched = re.MatchString(line)
		} else {
			matched = strings.Contains(line, pattern)
		}

		if matched {
			matches = append(matches, fmt.Sprintf("L%d: %s", i+1, line))
			if len(matches) >= maxResults {
				break
			}
		}
	}

	if len(matches) == 0 {
		return "No matches found.", nil
	}

	return strings.Join(matches, "\n"), nil
}

// QueryJSON extracts values from stored JSON using a simple dot-path expression.
// Supports paths like "data.result" or "data.result[*].metric.__name__".
// This avoids needing jq while giving the LLM structured access.
func (s *Store) QueryJSON(path, jsonPath string, maxResults int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.files[path]
	if !ok {
		return "", fmt.Errorf("file not found: %s", path)
	}

	if maxResults <= 0 {
		maxResults = 50
	}

	var root interface{}
	if err := json.Unmarshal([]byte(f.content), &root); err != nil {
		return "", fmt.Errorf("file is not valid JSON: %w", err)
	}

	results := extractPath(root, strings.Split(jsonPath, "."), maxResults)

	out, _ := json.MarshalIndent(results, "", "  ")
	// Truncate if too large
	if len(out) > 16*1024 {
		out = out[:16*1024]
		out = append(out, []byte("\n...(truncated)")...)
	}
	return string(out), nil
}

// extractPath navigates a JSON structure following dot-separated keys.
// Supports [*] to iterate arrays and [N] for specific indices.
func extractPath(node interface{}, parts []string, maxResults int) interface{} {
	if len(parts) == 0 || node == nil {
		return node
	}

	part := parts[0]
	rest := parts[1:]

	switch v := node.(type) {
	case map[string]interface{}:
		if child, ok := v[part]; ok {
			return extractPath(child, rest, maxResults)
		}
		return nil

	case []interface{}:
		if part == "[*]" || part == "*" {
			var results []interface{}
			for _, item := range v {
				if len(results) >= maxResults {
					break
				}
				r := extractPath(item, rest, maxResults-len(results))
				if r != nil {
					results = append(results, r)
				}
			}
			return results
		}
		// Try numeric index
		var idx int
		if _, err := fmt.Sscanf(part, "[%d]", &idx); err == nil {
			if idx >= 0 && idx < len(v) {
				return extractPath(v[idx], rest, maxResults)
			}
		}
		return nil
	}

	return nil
}
