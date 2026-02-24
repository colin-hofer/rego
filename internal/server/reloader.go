package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Reloader struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func NewReloader() *Reloader {
	return &Reloader{clients: make(map[chan string]struct{})}
}

func (r *Reloader) Events(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := make(chan string, 8)
	r.addClient(client)
	defer r.removeClient(client)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-req.Context().Done():
			return
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case reason := <-client:
			safeReason := strings.ReplaceAll(reason, "\n", " ")
			_, _ = fmt.Fprintf(w, "event: reload\ndata: %s\n\n", safeReason)
			flusher.Flush()
		}
	}
}

func (r *Reloader) Notify(reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "refresh"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for ch := range r.clients {
		select {
		case ch <- reason:
		default:
		}
	}
}

func (r *Reloader) addClient(ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[ch] = struct{}{}
}

func (r *Reloader) removeClient(ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, ch)
}
