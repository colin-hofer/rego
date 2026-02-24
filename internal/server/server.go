package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"rego/internal/logx"
	webassets "rego/web"
)

type Options struct {
	Addr     string
	Root     string
	Dev      bool
	Logger   *logx.Logger
	StaticFS fs.FS
}

type Server struct {
	addr       string
	dev        bool
	logger     *logx.Logger
	staticFS   fs.FS
	fileServer http.Handler
	reloader   *Reloader
	handler    http.Handler
}

func New(options Options) (*Server, error) {
	logger := options.Logger
	if logger == nil {
		logger = logx.New(logx.InfoLevel)
	}

	if strings.TrimSpace(options.Addr) == "" {
		options.Addr = ":8080"
	}

	if strings.TrimSpace(options.Root) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve cwd: %w", err)
		}
		options.Root = cwd
	}

	var staticFS fs.FS
	if options.StaticFS != nil {
		staticFS = options.StaticFS
	} else if options.Dev {
		staticFS = os.DirFS(filepath.Join(options.Root, "web", "dist"))
	} else {
		embedded, err := webassets.DistFS()
		if err != nil {
			return nil, fmt.Errorf("load embedded web assets: %w", err)
		}
		staticFS = embedded
	}

	s := &Server{
		addr:       options.Addr,
		dev:        options.Dev,
		logger:     logger,
		staticFS:   staticFS,
		fileServer: http.FileServer(http.FS(staticFS)),
		reloader:   NewReloader(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", s.handleHealth)
	if s.dev {
		mux.HandleFunc("/_dev/events", s.handleDevEvents)
		mux.HandleFunc("/_dev/reload", s.handleDevReload)
		mux.HandleFunc("/_dev/livereload.js", s.handleLiveReloadScript)
	}
	mux.HandleFunc("/", s.handleSPA)

	s.handler = s.recoverMiddleware(s.requestLogMiddleware(mux))
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              s.addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	s.logger.Info("http server started", "addr", s.addr, "dev", s.dev)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleDevEvents(w http.ResponseWriter, req *http.Request) {
	if !s.dev {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.reloader.Events(w, req)
}

func (s *Server) handleDevReload(w http.ResponseWriter, req *http.Request) {
	if !s.dev {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(req.Body).Decode(&payload)

	s.reloader.Notify(payload.Reason)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleLiveReloadScript(w http.ResponseWriter, req *http.Request) {
	if !s.dev {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte(liveReloadScript))
}

func (s *Server) handleSPA(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relPath := path.Clean(strings.TrimPrefix(req.URL.Path, "/"))
	if relPath == "." || relPath == "/" {
		s.serveIndex(w, req)
		return
	}

	if fileExists(s.staticFS, relPath) {
		s.fileServer.ServeHTTP(w, req)
		return
	}

	if strings.Contains(path.Base(relPath), ".") {
		http.NotFound(w, req)
		return
	}

	s.serveIndex(w, req)
}

func (s *Server) serveIndex(w http.ResponseWriter, req *http.Request) {
	index, err := fs.ReadFile(s.staticFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}

	content := string(index)
	if s.dev {
		content = injectLiveReload(content)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
}

func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, req)

		duration := time.Since(start).Round(time.Millisecond)
		level := s.logger.Info
		if recorder.statusCode >= http.StatusInternalServerError {
			level = s.logger.Error
		} else if recorder.statusCode >= http.StatusBadRequest {
			level = s.logger.Warn
		}

		level("request",
			"method", req.Method,
			"path", req.URL.Path,
			"status", recorder.statusCode,
			"bytes", recorder.bytes,
			"duration", duration,
		)
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered", "panic", recovered)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, req)
	})
}

func fileExists(fileSystem fs.FS, name string) bool {
	stat, err := fs.Stat(fileSystem, name)
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

func injectLiveReload(index string) string {
	const snippet = `<script src="/_dev/livereload.js"></script>`
	if strings.Contains(index, snippet) {
		return index
	}

	if strings.Contains(index, "</body>") {
		return strings.Replace(index, "</body>", snippet+"\n</body>", 1)
	}

	return index + "\n" + snippet
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	count, err := r.ResponseWriter.Write(body)
	r.bytes += count
	return count, err
}

const liveReloadScript = `(() => {
  const endpoint = "/_dev/events";
  const events = new EventSource(endpoint);

  events.addEventListener("reload", () => {
    window.location.reload();
  });

  events.onerror = () => {
    // EventSource automatically reconnects.
  };
})();
`
