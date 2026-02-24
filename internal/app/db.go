package app

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"rego/internal/db"
	"rego/internal/logx"
)

func runDB(ctx context.Context, root string, args []string) error {
	if len(args) == 0 {
		fmt.Print(dbUsage())
		return nil
	}

	switch args[0] {
	case "status":
		return runDBStatus(ctx, root, args[1:])
	case "stop":
		return runDBStop(ctx, root, args[1:])
	case "shell":
		return runDBShell(ctx, root, args[1:])
	case "psql":
		return runDBPSQL(ctx, root, args[1:])
	case "help", "-h", "--help":
		fmt.Print(dbUsage())
		return nil
	default:
		return fmt.Errorf("unknown db command %q\n\n%s", args[0], dbUsage())
	}
}

func runDBStatus(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("db status", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	databaseURL := flags.String("database-url", "", "Postgres connection string (defaults to DATABASE_URL)")
	if err := flags.Parse(args); err != nil {
		return err
	}

	resolvedURL := resolveDatabaseURL(*databaseURL)
	if resolvedURL != "" {
		err := db.Ping(ctx, resolvedURL)
		status := "up"
		if err != nil {
			status = "down"
		}

		fmt.Printf("mode: external\nstatus: %s\nurl: %s\n", status, redactDatabaseURL(resolvedURL))
		if err != nil {
			return err
		}
		return nil
	}

	status, err := db.InspectManaged(ctx, root)
	if err != nil {
		return err
	}

	fmt.Printf("mode: managed\nversion: %s\ndata_dir: %s\n", status.Version, status.DataDir)
	switch {
	case !status.Detected:
		fmt.Printf("status: stopped\n")
	case status.Running:
		fmt.Printf("status: running\nurl: %s\n", status.URL)
	default:
		fmt.Printf("status: stale\nurl: %s\n", status.URL)
	}

	if psqlPath, pathErr := exec.LookPath("psql"); pathErr == nil {
		fmt.Printf("psql: %s\n", psqlPath)
	}

	return nil
}

func runDBShell(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("db shell", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	databaseURL := flags.String("database-url", "", "Postgres connection string (defaults to DATABASE_URL or managed embedded Postgres)")
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel).WithComponent("db")
	runtime, err := db.Start(ctx, db.Options{
		Root:        root,
		Logger:      logger,
		DatabaseURL: resolveDatabaseURL(*databaseURL),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Connected to %s\n", redactDatabaseURL(runtime.URL))
	err = runSQLShell(ctx, runtime.DB)
	return closeRuntime(err, runtime)
}

func runDBStop(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("db stop", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel).WithComponent("db")
	stopped, err := db.StopManaged(ctx, root, logger)
	if err != nil {
		return err
	}

	if !stopped {
		fmt.Println("Managed Postgres is already stopped.")
		return nil
	}

	fmt.Println("Managed Postgres stopped.")
	return nil
}

func runDBPSQL(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("db psql", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	databaseURL := flags.String("database-url", "", "Postgres connection string (defaults to DATABASE_URL or managed embedded Postgres)")
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel).WithComponent("db")
	runtime, err := db.Start(ctx, db.Options{
		Root:        root,
		Logger:      logger,
		DatabaseURL: resolveDatabaseURL(*databaseURL),
	})
	if err != nil {
		return err
	}

	psqlBinary, err := exec.LookPath("psql")
	if err != nil {
		fmt.Println("psql not found in PATH, falling back to `rego db shell`.")
		err = runSQLShell(ctx, runtime.DB)
		return closeRuntime(err, runtime)
	}

	cmd := exec.CommandContext(ctx, psqlBinary, "--dbname", runtime.URL)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return closeRuntime(fmt.Errorf("run psql shell: %w", err), runtime)
	}

	return closeRuntime(nil, runtime)
}

func resolveDatabaseURL(flagValue string) string {
	resolved := strings.TrimSpace(flagValue)
	if resolved != "" {
		return resolved
	}

	return strings.TrimSpace(os.Getenv("DATABASE_URL"))
}

func redactDatabaseURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid>"
	}

	if parsed.User != nil {
		parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	}

	return parsed.String()
}

func dbUsage() string {
	return `rego db - managed Postgres utilities

Commands:
  status              Show database status.
  stop                Stop the managed Postgres process.
  shell               Open the built-in SQL shell.
  psql                Open the system psql shell (falls back to shell).

Examples:
  go run ./cmd/rego db status
  go run ./cmd/rego db stop
  go run ./cmd/rego db shell
  go run ./cmd/rego db shell --database-url postgres://user:pass@localhost:5432/app?sslmode=disable
`
}
