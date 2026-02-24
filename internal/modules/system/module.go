package system

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"rego/internal/logx"
)

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type Options struct {
	Logger   *logx.Logger
	Database DatabasePinger
}

type Module struct {
	logger   *logx.Logger
	database DatabasePinger
}

func New(options Options) *Module {
	logger := options.Logger
	if logger == nil {
		logger = logx.New(logx.InfoLevel).WithComponent("system")
	}

	return &Module{
		logger:   logger,
		database: options.Database,
	}
}

func (m *Module) Name() string {
	return "system"
}

func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/healthz", m.handleHealth)
}

func (m *Module) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type databaseHealth struct {
		Status string `json:"status"`
	}

	type healthResponse struct {
		Status   string         `json:"status"`
		Time     string         `json:"time"`
		Database databaseHealth `json:"database"`
	}

	statusCode := http.StatusOK
	databaseStatus := databaseHealth{Status: "disabled"}
	if m.database != nil {
		pingCtx, cancel := context.WithTimeout(req.Context(), 750*time.Millisecond)
		err := m.database.PingContext(pingCtx)
		cancel()

		if err != nil {
			databaseStatus.Status = "down"
			statusCode = http.StatusServiceUnavailable
			m.logger.Warn("database health check failed", "error", err)
		} else {
			databaseStatus.Status = "up"
		}
	}

	responseStatus := "ok"
	if statusCode != http.StatusOK {
		responseStatus = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := healthResponse{
		Status:   responseStatus,
		Time:     time.Now().UTC().Format(time.RFC3339),
		Database: databaseStatus,
	}
	_ = json.NewEncoder(w).Encode(response)
}
