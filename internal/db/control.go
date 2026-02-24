package db

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"rego/internal/logx"
)

func StopManaged(ctx context.Context, root string, logger *logx.Logger) (bool, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return false, err
	}

	status, err := InspectManaged(ctx, root)
	if err != nil {
		return false, err
	}
	if !status.Detected {
		return false, nil
	}

	var stopErrors error
	for _, pgCtlPath := range pgCtlCandidates(paths) {
		stopCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(stopCtx, pgCtlPath, "stop", "-w", "-D", paths.DataDir)
		output, runErr := cmd.CombinedOutput()
		cancel()

		if runErr == nil {
			if logger != nil {
				logger.Info("managed postgres stopped", "data_dir", paths.DataDir)
			}
			return true, nil
		}

		message := strings.TrimSpace(string(output))
		if message == "" {
			message = runErr.Error()
		}

		if strings.Contains(strings.ToLower(message), "pid file") && strings.Contains(strings.ToLower(message), "does not exist") {
			if logger != nil {
				logger.Info("managed postgres already stopped", "data_dir", paths.DataDir)
			}
			return true, nil
		}

		stopErrors = errors.Join(stopErrors, fmt.Errorf("%s: %s", pgCtlPath, message))
	}

	if stopErrors != nil {
		return false, fmt.Errorf("stop managed postgres failed: %w", stopErrors)
	}

	return false, fmt.Errorf("managed postgres is not bootstrapped yet; start it once with `rego serve` or `rego db shell`")
}

func pgCtlCandidates(paths Paths) []string {
	binaryName := "pg_ctl"
	if runtime.GOOS == "windows" {
		binaryName = "pg_ctl.exe"
	}

	candidates := make([]string, 0, 4)
	seen := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		if !PathExists(path) {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	add(filepath.Join(paths.BinariesDir, "bin", binaryName))

	matches, _ := filepath.Glob(filepath.Join(paths.BaseDir, "binaries", "*", "bin", binaryName))
	for _, match := range matches {
		add(match)
	}

	return candidates
}
