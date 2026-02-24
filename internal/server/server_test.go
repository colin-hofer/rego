package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"rego/internal/logx"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	sut := newTestServer(t)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)

	sut.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	body, _ := io.ReadAll(recorder.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("expected response body to include status=ok, got %s", string(body))
	}
}

func TestSPAFallbackAndAssetHandling(t *testing.T) {
	t.Parallel()

	sut := newTestServer(t)

	t.Run("spa routes return index", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/dashboard/alpha", nil)

		sut.Handler().ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}

		if !strings.Contains(recorder.Body.String(), "<div id=\"root\"></div>") {
			t.Fatalf("expected index html body, got %q", recorder.Body.String())
		}
	})

	t.Run("missing assets return 404", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/missing.js", nil)

		sut.Handler().ServeHTTP(recorder, req)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
		}
	})
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	static := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body><div id=\"root\"></div></body></html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('ok')")},
		"app.css":    &fstest.MapFile{Data: []byte("body{}")},
	}

	sut, err := New(Options{
		Addr:     ":0",
		Dev:      false,
		Logger:   logx.New(logx.ErrorLevel),
		StaticFS: static,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	return sut
}
