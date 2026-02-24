package web

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"rego/internal/logx"
)

func EnsureNodeModules(ctx context.Context, root string, logger *logx.Logger) error {
	webDir := filepath.Join(root, "web")
	if _, err := os.Stat(filepath.Join(webDir, "package.json")); err != nil {
		return fmt.Errorf("missing web/package.json: %w", err)
	}

	nodeModulesDir := filepath.Join(webDir, "node_modules")
	if _, err := os.Stat(nodeModulesDir); err == nil {
		return nil
	}

	logger.Info("installing frontend dependencies")
	cmd := exec.CommandContext(ctx, "npm", "install")
	cmd.Dir = webDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	return nil
}
