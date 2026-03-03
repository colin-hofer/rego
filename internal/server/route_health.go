package server

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

func handleHealth(logger *logx.Logger, database DatabasePinger) http.HandlerFunc {
	if logger == nil {
		logger = logx.New(logx.InfoLevel).WithComponent("api")
	}

	return func(w http.ResponseWriter, req *http.Request) {
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
		if database != nil {
			pingCtx, cancel := context.WithTimeout(req.Context(), 750*time.Millisecond)
			err := database.PingContext(pingCtx)
			cancel()

			if err != nil {
				databaseStatus.Status = "down"
				statusCode = http.StatusServiceUnavailable
				logger.Warn("database health check failed", "error", err)
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
}
