package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"rego/internal/dev"
	"rego/internal/logx"
	"rego/internal/server"
	"rego/internal/web"
)

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided\n\n%s", usage())
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	switch args[0] {
	case "dev":
		return runDev(ctx, root, args[1:])
	case "serve":
		return runServe(ctx, root, args[1:])
	case "build":
		return runBuild(ctx, root, args[1:])
	case "test":
		return runTest(ctx, root, args[1:])
	case "help", "-h", "--help":
		fmt.Print(usage())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
	}
}

func runDev(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	addr := flags.String("addr", ":8080", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel)
	return dev.Run(ctx, root, *addr, logger)
}

func runServe(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	addr := flags.String("addr", ":8080", "HTTP listen address")
	devMode := flags.Bool("dev", false, "serve files from disk and enable live reload endpoints")
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel).WithComponent("http")
	httpServer, err := server.New(server.Options{
		Addr:   *addr,
		Root:   root,
		Dev:    *devMode,
		Logger: logger,
	})
	if err != nil {
		return err
	}

	return httpServer.ListenAndServe(ctx)
}

func runBuild(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	output := flags.String("output", "bin/rego", "output path for built Go binary")
	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel)

	if err := web.EnsureNodeModules(ctx, root, logger.WithComponent("npm")); err != nil {
		return err
	}

	builder := web.NewBuilder(root, logger.WithComponent("web"))
	if err := builder.Build(false); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(*output)), 0o755); err != nil {
		return fmt.Errorf("create build output directory: %w", err)
	}

	if err := runCommand(ctx, root, logger.WithComponent("go"), "go", "build", "-o", *output, "./cmd/rego"); err != nil {
		return err
	}

	logger.Info("build complete", "binary", *output)
	return nil
}

func runTest(ctx context.Context, root string, args []string) error {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	if err := flags.Parse(args); err != nil {
		return err
	}

	logger := newLogger(logx.InfoLevel)

	if err := runCommand(ctx, root, logger.WithComponent("go"), "go", "test", "./..."); err != nil {
		return err
	}

	if err := web.EnsureNodeModules(ctx, root, logger.WithComponent("npm")); err != nil {
		return err
	}

	webDir := filepath.Join(root, "web")
	if err := runCommand(ctx, webDir, logger.WithComponent("web-test"), "npm", "run", "test", "--", "--run"); err != nil {
		return err
	}

	logger.Info("all tests passed")
	return nil
}

func runCommand(ctx context.Context, workingDir string, logger *logx.Logger, command string, args ...string) error {
	logger.Info("running command", "command", strings.Join(append([]string{command}, args...), " "))

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed (%s): %w", strings.Join(append([]string{command}, args...), " "), err)
	}

	return nil
}

func newLogger(defaultLevel logx.Level) *logx.Logger {
	level := defaultLevel
	if parsedLevel, ok := logx.ParseLevel(os.Getenv("REGO_LOG_LEVEL")); ok {
		level = parsedLevel
	}
	return logx.New(level)
}

func usage() string {
	return `rego - Go-first React application toolkit

Commands:
  dev                 Run local development mode with hot reload.
  serve               Run the HTTP server (embedded assets by default).
  build               Build frontend assets and Go binary.
  test                Run backend and frontend tests.

Examples:
  go run ./cmd/rego dev
  go run ./cmd/rego serve --addr :8080
  go run ./cmd/rego build --output bin/rego
  go run ./cmd/rego test
`
}
