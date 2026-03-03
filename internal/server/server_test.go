package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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
	if !strings.Contains(string(body), `"database":{"status":"disabled"}`) {
		t.Fatalf("expected response body to include database status, got %s", string(body))
	}
}

func TestHealthEndpointReportsDegradedWhenDatabaseFails(t *testing.T) {
	t.Parallel()

	sut, err := New(Options{
		Addr:     ":0",
		Dev:      false,
		Logger:   logx.New(logx.ErrorLevel),
		StaticFS: testStaticFS(),
		Database: failingPinger{},
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)

	sut.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"degraded"`) {
		t.Fatalf("expected degraded status, got %s", body)
	}
	if !strings.Contains(body, `"database":{"status":"down"}`) {
		t.Fatalf("expected database=down, got %s", body)
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

func TestDevEventsEndpointStreams(t *testing.T) {
	t.Parallel()

	static := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body><div id=\"root\"></div></body></html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('ok')")},
		"app.css":    &fstest.MapFile{Data: []byte("body{}")},
	}

	sut, err := New(Options{
		Addr:     ":0",
		Dev:      true,
		Logger:   logx.New(logx.ErrorLevel),
		StaticFS: static,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_dev/events", nil)

	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		sut.Handler().ServeHTTP(recorder, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event stream response")
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}

	if !strings.Contains(recorder.Body.String(), ": connected") {
		t.Fatalf("expected initial stream frame, got %q", recorder.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	sut, err := New(Options{
		Addr:     ":0",
		Dev:      false,
		Logger:   logx.New(logx.ErrorLevel),
		StaticFS: testStaticFS(),
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	return sut
}

func testStaticFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body><div id=\"root\"></div></body></html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('ok')")},
		"app.css":    &fstest.MapFile{Data: []byte("body{}")},
	}
}

type failingPinger struct{}

func (failingPinger) PingContext(context.Context) error {
	return errors.New("boom")
}
