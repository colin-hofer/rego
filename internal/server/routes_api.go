package server

import (
	"net/http"

	"rego/internal/logx"
)

func registerAPIRoutes(mux *http.ServeMux, logger *logx.Logger, database DatabasePinger) {
	if mux == nil {
		return
	}

	mux.HandleFunc("/api/healthz", handleHealth(logger, database))
}
