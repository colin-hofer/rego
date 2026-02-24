package dev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"rego/internal/logx"
	"rego/internal/web"
)

func Run(ctx context.Context, root string, addr string, serverEnv map[string]string, logger *logx.Logger) error {
	devLogger := logger.WithComponent("dev")
	goLogger := logger.WithComponent("go")
	reloadLogger := logger.WithComponent("reload")

	if err := web.EnsureNodeModules(ctx, root, logger.WithComponent("npm")); err != nil {
		return err
	}

	devBuilder, err := web.NewDevBuilder(root, logger.WithComponent("web"))
	if err != nil {
		return err
	}
	defer devBuilder.Close()

	if err := devBuilder.Rebuild(); err != nil {
		return err
	}

	binaryPath := filepath.Join(root, ".tmp", "rego-dev")
	if err := buildBackend(ctx, root, binaryPath, goLogger); err != nil {
		return err
	}

	process := newManagedProcess(root, binaryPath, addr, serverEnv, logger.WithComponent("server"))
	if err := process.Start(); err != nil {
		return err
	}
	defer process.Stop(3 * time.Second)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	if err := addRecursiveWatches(watcher, root); err != nil {
		return fmt.Errorf("configure watcher: %w", err)
	}

	devLogger.Info("dev mode started", "addr", addr)

	goChanges := make(chan struct{}, 1)
	webChanges := make(chan struct{}, 1)

	var actionMu sync.Mutex

	go runDebounced(ctx, goChanges, 450*time.Millisecond, func() {
		actionMu.Lock()
		defer actionMu.Unlock()

		if err := buildBackend(ctx, root, binaryPath, goLogger); err != nil {
			logger.Error("go rebuild failed", "error", err)
			return
		}

		if err := process.Restart(); err != nil {
			logger.Error("backend restart failed", "error", err)
			return
		}

		notifyReload(addr, "backend reloaded", reloadLogger)
	})

	go runDebounced(ctx, webChanges, 250*time.Millisecond, func() {
		actionMu.Lock()
		defer actionMu.Unlock()

		if err := devBuilder.Rebuild(); err != nil {
			logger.Error("frontend build failed", "error", err)
			return
		}

		notifyReload(addr, "frontend rebuilt", reloadLogger)
	})

	for {
		select {
		case <-ctx.Done():
			devLogger.Info("dev mode stopping")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}

			relPath, ok := relPath(root, event.Name)
			if !ok || shouldSkipPath(relPath) {
				continue
			}

			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = addRecursiveWatchesFrom(watcher, root, event.Name)
					continue
				}
			}

			if isGoChange(relPath) {
				notify(goChanges)
			}
			if isWebChange(relPath) {
				notify(webChanges)
			}
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			devLogger.Warn("watcher error", "error", watchErr)
		}
	}
}

func buildBackend(ctx context.Context, root string, outputPath string, logger *logx.Logger) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./cmd/rego")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	logger.Info("backend built", "duration", time.Since(start).Round(time.Millisecond))
	return nil
}

func addRecursiveWatches(watcher *fsnotify.Watcher, root string) error {
	return addRecursiveWatchesFrom(watcher, root, root)
}

func addRecursiveWatchesFrom(watcher *fsnotify.Watcher, moduleRoot string, walkRoot string) error {
	return filepath.WalkDir(walkRoot, func(currentPath string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		rel, ok := relPath(moduleRoot, currentPath)
		if ok && shouldSkipPath(rel) {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !dirEntry.IsDir() {
			return nil
		}

		if err := watcher.Add(currentPath); err != nil {
			return fmt.Errorf("watch %s: %w", currentPath, err)
		}
		return nil
	})
}

func relPath(root string, absolutePath string) (string, bool) {
	relative, err := filepath.Rel(root, absolutePath)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func shouldSkipPath(path string) bool {
	path = filepath.ToSlash(strings.TrimPrefix(path, "./"))
	if path == "." || path == "" {
		return false
	}

	prefixes := []string{
		".git",
		".tmp",
		"bin",
		"web/node_modules",
		"web/dist",
	}

	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	return false
}

func isGoChange(path string) bool {
	if shouldSkipPath(path) {
		return false
	}
	return strings.HasSuffix(path, ".go") || path == "go.mod" || path == "go.sum"
}

func isWebChange(path string) bool {
	if !strings.HasPrefix(path, "web/") || shouldSkipPath(path) {
		return false
	}

	switch path {
	case "web/index.html", "web/package.json", "web/package-lock.json", "web/tsconfig.json", "web/vitest.config.ts":
		return true
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".css", ".html", ".json":
		return true
	default:
		return false
	}
}

func notifyReload(addr string, reason string, logger *logx.Logger) {
	endpoint := strings.TrimRight(devBaseURL(addr), "/") + "/_dev/reload"
	payload, _ := json.Marshal(map[string]string{"reason": reason})
	client := &http.Client{Timeout: 800 * time.Millisecond}

	for range 6 {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			logger.Debug("invalid reload request", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	logger.Debug("reload endpoint unreachable", "endpoint", endpoint)
}

func devBaseURL(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, ":") {
		return "http://127.0.0.1" + trimmed
	}
	return "http://" + trimmed
}

func runDebounced(ctx context.Context, trigger <-chan struct{}, delay time.Duration, fn func()) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	pending := false

	for {
		select {
		case <-ctx.Done():
			if pending && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-trigger:
			if pending && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(delay)
			pending = true
		case <-timer.C:
			pending = false
			fn()
		}
	}
}

func notify(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

type managedProcess struct {
	mu         sync.Mutex
	root       string
	binaryPath string
	addr       string
	env        []string
	logger     *logx.Logger
	cmd        *exec.Cmd
	done       chan error
}

func newManagedProcess(root string, binaryPath string, addr string, env map[string]string, logger *logx.Logger) *managedProcess {
	return &managedProcess{
		root:       root,
		binaryPath: binaryPath,
		addr:       addr,
		env:        formatEnvironment(env),
		logger:     logger,
	}
}

func (p *managedProcess) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return fmt.Errorf("process is already running")
	}

	cmd := exec.Command(p.binaryPath, "serve", "--dev", "--addr", p.addr)
	cmd.Dir = p.root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(p.env) > 0 {
		cmd.Env = mergeEnvironment(os.Environ(), p.env)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start backend process: %w", err)
	}

	done := make(chan error, 1)
	p.cmd = cmd
	p.done = done

	p.logger.Info("backend started", "pid", cmd.Process.Pid, "addr", p.addr)

	go func() {
		err := cmd.Wait()
		done <- err
		if err != nil {
			p.logger.Warn("backend process exited", "error", err)
			return
		}
		p.logger.Warn("backend process exited")
	}()

	return nil
}

func formatEnvironment(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}

	return env
}

func mergeEnvironment(base []string, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}

	overrideKeys := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			overrideKeys[key] = struct{}{}
		}
	}

	merged := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, exists := overrideKeys[key]; exists {
				continue
			}
		}
		merged = append(merged, entry)
	}

	merged = append(merged, overrides...)
	return merged
}

func (p *managedProcess) Stop(timeout time.Duration) error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	p.cmd = nil
	p.done = nil
	p.mu.Unlock()

	if cmd == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	default:
	}

	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return nil
	}
}

func (p *managedProcess) Restart() error {
	if err := p.Stop(3 * time.Second); err != nil {
		return err
	}
	return p.Start()
}
