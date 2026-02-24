package metadata

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"rego/internal/logx"
)

type Options struct {
	Logger *logx.Logger
	DB     *sql.DB
}

type Module struct {
	logger *logx.Logger
	db     *sql.DB
}

type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func New(options Options) *Module {
	logger := options.Logger
	if logger == nil {
		logger = logx.New(logx.InfoLevel).WithComponent("metadata")
	}

	return &Module{logger: logger, db: options.DB}
}

func (m *Module) Name() string {
	return "metadata"
}

func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/metadata", m.handleCollection)
}

func (m *Module) handleCollection(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		m.list(w, req)
	case http.MethodPut:
		m.upsert(w, req)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Module) list(w http.ResponseWriter, req *http.Request) {
	if m.db == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	rows, err := m.db.QueryContext(req.Context(), `
		SELECT key, value, updated_at
		FROM app_metadata
		ORDER BY key
	`)
	if err != nil {
		m.logger.Error("query metadata failed", "error", err)
		http.Error(w, "query metadata failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.Key, &entry.Value, &entry.UpdatedAt); err != nil {
			m.logger.Error("scan metadata failed", "error", err)
			http.Error(w, "scan metadata failed", http.StatusInternalServerError)
			return
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		m.logger.Error("iterate metadata failed", "error", err)
		http.Error(w, "iterate metadata failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (m *Module) upsert(w http.ResponseWriter, req *http.Request) {
	if m.db == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	var payload struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	payload.Key = strings.TrimSpace(payload.Key)
	if payload.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	entry, err := m.upsertEntry(req, payload.Key, payload.Value)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, errBadInput) {
			statusCode = http.StatusBadRequest
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

var errBadInput = errors.New("bad input")

func (m *Module) upsertEntry(req *http.Request, key string, value string) (Entry, error) {
	if len(key) > 256 {
		return Entry{}, fmt.Errorf("%w: key cannot exceed 256 characters", errBadInput)
	}

	row := m.db.QueryRowContext(req.Context(), `
		INSERT INTO app_metadata (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
		RETURNING key, value, updated_at
	`, key, value)

	var entry Entry
	if err := row.Scan(&entry.Key, &entry.Value, &entry.UpdatedAt); err != nil {
		m.logger.Error("upsert metadata failed", "error", err)
		return Entry{}, fmt.Errorf("save metadata failed")
	}

	return entry, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
