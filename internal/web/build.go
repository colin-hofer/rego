package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"rego/internal/logx"
)

type Builder struct {
	root   string
	logger *logx.Logger
}

func NewBuilder(root string, logger *logx.Logger) *Builder {
	return &Builder{root: root, logger: logger}
}

func (b *Builder) Build(dev bool) error {
	mode := modeName(dev)
	start := time.Now()

	if err := prepareDist(b.root, !dev); err != nil {
		return err
	}

	if err := copyIndexHTML(b.root); err != nil {
		return err
	}

	result := api.Build(buildOptions(b.root, dev))
	if err := checkBuildResult(result); err != nil {
		return err
	}

	logBuildOutcome(b.logger, mode, start, result.Warnings)
	return nil
}

type DevBuilder struct {
	root   string
	logger *logx.Logger

	mu  sync.Mutex
	ctx api.BuildContext
}

func NewDevBuilder(root string, logger *logx.Logger) (*DevBuilder, error) {
	builder := &DevBuilder{root: root, logger: logger}

	ctx, ctxErr := api.Context(buildOptions(root, true))
	if ctxErr != nil {
		return nil, fmt.Errorf("create esbuild context:\n%s", formatMessages(ctxErr.Errors))
	}
	builder.ctx = ctx

	return builder, nil
}

func (b *DevBuilder) Rebuild() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	start := time.Now()

	if err := prepareDist(b.root, false); err != nil {
		return err
	}

	if err := copyIndexHTML(b.root); err != nil {
		return err
	}

	result := b.ctx.Rebuild()
	if err := checkBuildResult(result); err != nil {
		return err
	}

	logBuildOutcome(b.logger, "development", start, result.Warnings)
	return nil
}

func (b *DevBuilder) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ctx == nil {
		return
	}

	b.ctx.Dispose()
	b.ctx = nil
}

func modeName(dev bool) string {
	mode := "production"
	if dev {
		mode = "development"
	}
	return mode
}

func prepareDist(root string, clean bool) error {
	distDir := filepath.Join(root, "web", "dist")

	if clean {
		if err := os.RemoveAll(distDir); err != nil {
			return fmt.Errorf("clear dist directory: %w", err)
		}
	}

	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create dist directory: %w", err)
	}

	return nil
}

func copyIndexHTML(root string) error {
	webDir := filepath.Join(root, "web")
	if err := copyFile(filepath.Join(webDir, "index.html"), filepath.Join(webDir, "dist", "index.html")); err != nil {
		return fmt.Errorf("copy index.html: %w", err)
	}
	return nil
}

func buildOptions(root string, dev bool) api.BuildOptions {
	sourceMap := api.SourceMapNone
	if dev {
		sourceMap = api.SourceMapInline
	}

	return api.BuildOptions{
		AbsWorkingDir: root,
		EntryPoints:   []string{"web/src/main.tsx"},
		Bundle:        true,
		Outfile:       "web/dist/app.js",
		Platform:      api.PlatformBrowser,
		Format:        api.FormatESModule,
		Target:        api.ES2022,
		Sourcemap:     sourceMap,
		JSX:           api.JSXAutomatic,
		Define: map[string]string{
			"process.env.NODE_ENV": strconv.Quote(modeName(dev)),
		},
		AssetNames: "assets/[name]-[hash]",
		Loader: map[string]api.Loader{
			".svg":  api.LoaderFile,
			".png":  api.LoaderFile,
			".jpg":  api.LoaderFile,
			".jpeg": api.LoaderFile,
			".gif":  api.LoaderFile,
			".webp": api.LoaderFile,
		},
		MinifyWhitespace:  !dev,
		MinifyIdentifiers: !dev,
		MinifySyntax:      !dev,
		LogLevel:          api.LogLevelSilent,
		Write:             true,
	}
}

func checkBuildResult(result api.BuildResult) error {
	if len(result.Errors) > 0 {
		return fmt.Errorf("esbuild failed:\n%s", formatMessages(result.Errors))
	}
	return nil
}

func logBuildOutcome(logger *logx.Logger, mode string, start time.Time, warnings []api.Message) {
	for _, warning := range warnings {
		logger.Warn("esbuild warning", "message", formatMessage(warning))
	}

	logger.Info("frontend built", "mode", mode, "duration", time.Since(start).Round(time.Millisecond))
}

func copyFile(src string, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func formatMessages(messages []api.Message) string {
	formatted := make([]string, 0, len(messages))
	for _, msg := range messages {
		formatted = append(formatted, formatMessage(msg))
	}
	return strings.Join(formatted, "\n")
}

func formatMessage(msg api.Message) string {
	if msg.Location == nil {
		return msg.Text
	}
	return fmt.Sprintf("%s:%d:%d: %s", msg.Location.File, msg.Location.Line, msg.Location.Column, msg.Text)
}
