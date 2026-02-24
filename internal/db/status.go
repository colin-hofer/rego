package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Paths struct {
	BaseDir     string
	RuntimeDir  string
	DataDir     string
	BinariesDir string
	CacheDir    string
}

type ManagedStatus struct {
	Detected bool
	Running  bool
	URL      string
	Port     uint32
	Version  string
	DataDir  string
}

func ResolvePaths(root string) (Paths, error) {
	resolvedRoot, err := resolveRoot(root)
	if err != nil {
		return Paths{}, err
	}

	baseDir := filepath.Join(resolvedRoot, ".tmp", "postgres")
	return Paths{
		BaseDir:     baseDir,
		RuntimeDir:  filepath.Join(baseDir, "runtime"),
		DataDir:     filepath.Join(baseDir, "data"),
		BinariesDir: filepath.Join(baseDir, "binaries", string(embeddedVersion)),
		CacheDir:    filepath.Join(baseDir, "cache"),
	}, nil
}

func InspectManaged(ctx context.Context, root string) (ManagedStatus, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return ManagedStatus{}, err
	}

	status := ManagedStatus{
		Version: string(embeddedVersion),
		DataDir: paths.DataDir,
	}

	port, ok := readRunningPort(paths.DataDir)
	if !ok {
		return status, nil
	}

	status.Detected = true
	status.Port = port
	status.URL = buildURL(port)
	status.Running = canConnect(ctx, status.URL)

	return status, nil
}

func Ping(ctx context.Context, databaseURL string) error {
	connection, err := sql.Open("pgx", strings.TrimSpace(databaseURL))
	if err != nil {
		return fmt.Errorf("open postgres connection: %w", err)
	}
	defer connection.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := connection.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	return nil
}

func PathExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
