package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pg "github.com/fergusstrange/embedded-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"

	"rego/internal/logx"
)

const (
	defaultUsername = "rego"
	defaultPassword = "rego"
	defaultDatabase = "rego"

	defaultMaxOpenConns    = 24
	defaultMaxIdleConns    = 12
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultConnMaxLifetime = 45 * time.Minute

	embeddedVersion = pg.V18
)

type Options struct {
	Root        string
	Logger      *logx.Logger
	DatabaseURL string
}

type Runtime struct {
	URL string
	DB  *sql.DB

	logger   *logx.Logger
	embedded *pg.EmbeddedPostgres
}

func Start(ctx context.Context, options Options) (*Runtime, error) {
	logger := options.Logger
	if logger == nil {
		logger = logx.New(logx.InfoLevel).WithComponent("db")
	}

	root, err := resolveRoot(options.Root)
	if err != nil {
		return nil, err
	}

	databaseURL := strings.TrimSpace(options.DatabaseURL)
	if databaseURL == "" {
		databaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}

	runtime := &Runtime{logger: logger, URL: databaseURL}

	if runtime.URL == "" {
		embedded, embeddedURL, startErr := startEmbedded(ctx, root, logger)
		if startErr != nil {
			return nil, startErr
		}
		runtime.embedded = embedded
		runtime.URL = embeddedURL
	} else {
		logger.Info("using external postgres", "database_url", redactURL(runtime.URL))
	}

	connection, err := openConnection(ctx, runtime.URL)
	if err != nil {
		_ = runtime.stopEmbedded()
		return nil, err
	}
	runtime.DB = connection

	if err := runMigrations(ctx, runtime.DB); err != nil {
		_ = runtime.Close()
		return nil, err
	}

	logger.Info("database ready")
	return runtime, nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}

	var closeErr error

	if err := r.CloseConnections(); err != nil {
		closeErr = errors.Join(closeErr, err)
	}

	if err := r.stopEmbedded(); err != nil {
		closeErr = errors.Join(closeErr, err)
	}

	return closeErr
}

func (r *Runtime) CloseConnections() error {
	if r == nil || r.DB == nil {
		return nil
	}

	if err := r.DB.Close(); err != nil {
		return fmt.Errorf("close database connection pool: %w", err)
	}

	r.DB = nil
	return nil
}

func (r *Runtime) stopEmbedded() error {
	if r == nil || r.embedded == nil {
		return nil
	}

	err := r.embedded.Stop()
	r.embedded = nil

	if err != nil && !errors.Is(err, pg.ErrServerNotStarted) {
		return fmt.Errorf("stop embedded postgres: %w", err)
	}

	if r.logger != nil {
		r.logger.Info("embedded postgres stopped")
	}

	return nil
}

func resolveRoot(root string) (string, error) {
	if strings.TrimSpace(root) != "" {
		return root, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}

	return cwd, nil
}

func startEmbedded(ctx context.Context, root string, logger *logx.Logger) (*pg.EmbeddedPostgres, string, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return nil, "", err
	}

	if err := os.MkdirAll(paths.BaseDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create postgres workspace: %w", err)
	}

	if existingURL, reused := findRunningEmbedded(ctx, paths.DataDir, logger); reused {
		return nil, existingURL, nil
	}

	port, err := reservePort()
	if err != nil {
		return nil, "", err
	}

	config := pg.DefaultConfig().
		Version(embeddedVersion).
		Username(defaultUsername).
		Password(defaultPassword).
		Database(defaultDatabase).
		Port(port).
		RuntimePath(paths.RuntimeDir).
		DataPath(paths.DataDir).
		BinariesPath(paths.BinariesDir).
		CachePath(paths.CacheDir).
		StartTimeout(25 * time.Second).
		Logger(io.Discard)

	instance := pg.NewDatabase(config)
	logger.Info("starting embedded postgres", "version", string(embeddedVersion), "data_dir", paths.DataDir, "port", port)
	if err := instance.Start(); err != nil {
		return nil, "", fmt.Errorf("start embedded postgres: %w", err)
	}

	return instance, buildURL(port), nil
}

func findRunningEmbedded(ctx context.Context, dataDir string, logger *logx.Logger) (string, bool) {
	port, ok := readRunningPort(dataDir)
	if !ok {
		return "", false
	}

	existingURL := buildURL(port)
	if !canConnect(ctx, existingURL) {
		return "", false
	}

	logger.Info("reusing running embedded postgres", "data_dir", dataDir, "port", port)
	return existingURL, true
}

func readRunningPort(dataDir string) (uint32, bool) {
	pidPath := filepath.Join(dataDir, "postmaster.pid")
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 4 {
		return 0, false
	}

	port, err := strconv.Atoi(strings.TrimSpace(lines[3]))
	if err != nil || port <= 0 {
		return 0, false
	}

	return uint32(port), true
}

func canConnect(ctx context.Context, databaseURL string) bool {
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return false
	}
	defer connection.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()

	return connection.PingContext(pingCtx) == nil
}

func reservePort() (uint32, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve postgres port: %w", err)
	}
	defer listener.Close()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("resolve reserved postgres port")
	}

	if address.Port <= 0 {
		return 0, fmt.Errorf("invalid reserved postgres port: %d", address.Port)
	}

	return uint32(address.Port), nil
}

func buildURL(port uint32) string {
	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(defaultUsername, defaultPassword),
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
		Path:   defaultDatabase,
	}

	query := connectionURL.Query()
	query.Set("sslmode", "disable")
	query.Set("application_name", "rego")
	connectionURL.RawQuery = query.Encode()

	return connectionURL.String()
}

func openConnection(ctx context.Context, databaseURL string) (*sql.DB, error) {
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	connection.SetMaxOpenConns(defaultMaxOpenConns)
	connection.SetMaxIdleConns(defaultMaxIdleConns)
	connection.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	connection.SetConnMaxLifetime(defaultConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := connection.PingContext(pingCtx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return connection, nil
}

func redactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid>"
	}

	if parsed.User != nil {
		parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	}

	return parsed.String()
}
