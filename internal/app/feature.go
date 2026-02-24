package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validFeatureName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func runFeature(_ context.Context, root string, args []string) error {
	if len(args) == 0 {
		fmt.Print(featureUsage())
		return nil
	}

	switch args[0] {
	case "new":
		if len(args) < 2 {
			return fmt.Errorf("missing feature name\n\n%s", featureUsage())
		}
		return scaffoldFeature(root, args[1])
	case "help", "-h", "--help":
		fmt.Print(featureUsage())
		return nil
	default:
		return fmt.Errorf("unknown feature command %q\n\n%s", args[0], featureUsage())
	}
}

func scaffoldFeature(root string, rawName string) error {
	name := strings.TrimSpace(strings.ToLower(rawName))
	if !validFeatureName.MatchString(name) {
		return fmt.Errorf("invalid feature name %q (use lowercase letters, numbers, underscores; start with a letter)", rawName)
	}

	componentName := pascalCase(name) + "Panel"

	backendDir := filepath.Join(root, "internal", "modules", name)
	frontendDir := filepath.Join(root, "web", "src", "features", name)

	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		return fmt.Errorf("create backend feature directory: %w", err)
	}
	if err := os.MkdirAll(frontendDir, 0o755); err != nil {
		return fmt.Errorf("create frontend feature directory: %w", err)
	}

	backendFile := filepath.Join(backendDir, "module.go")
	frontendComponentFile := filepath.Join(frontendDir, componentName+".tsx")
	frontendAPIFile := filepath.Join(frontendDir, "api.ts")

	if err := writeIfMissing(backendFile, renderBackendModuleTemplate(name)); err != nil {
		return err
	}
	if err := writeIfMissing(frontendComponentFile, renderFrontendComponentTemplate(componentName)); err != nil {
		return err
	}
	if err := writeIfMissing(frontendAPIFile, renderFrontendAPITemplate(name)); err != nil {
		return err
	}

	fmt.Printf("Feature %q scaffolded.\n", name)
	fmt.Printf("- Register backend module in internal/app/modules.go\n")
	fmt.Printf("- Register frontend route in web/src/app/routes.tsx\n")
	return nil
}

func writeIfMissing(path string, contents string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check file %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	return nil
}

func pascalCase(name string) string {
	parts := strings.Split(name, "_")
	builder := strings.Builder{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			builder.WriteString(part[1:])
		}
	}
	if builder.Len() == 0 {
		return "Feature"
	}
	return builder.String()
}

func renderBackendModuleTemplate(name string) string {
	return fmt.Sprintf(`package %s

import (
	"encoding/json"
	"net/http"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return %q
}

func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/%s", m.handleRoot)
}

func (m *Module) handleRoot(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"feature": %q,
		"status":  "ready",
	})
}
`, name, name, name, name)
}

func renderFrontendComponentTemplate(componentName string) string {
	return fmt.Sprintf(`import { useEffect, useState } from "react";
import { fetchStatus } from "./api";

export function %s() {
	const [status, setStatus] = useState("loading");

	useEffect(() => {
		let cancelled = false;
		fetchStatus()
			.then((nextStatus) => {
				if (!cancelled) {
					setStatus(nextStatus);
				}
			})
			.catch(() => {
				if (!cancelled) {
					setStatus("error");
				}
			});

		return () => {
			cancelled = true;
		};
	}, []);

	return (
		<section className="panel">
			<h2>%s</h2>
			<p className="panel-copy">Scaffolded feature panel.</p>
			<p className="status-meta">Status: {status}</p>
		</section>
	);
}
`, componentName, componentName)
}

func renderFrontendAPITemplate(name string) string {
	return fmt.Sprintf(`type StatusResponse = {
	status: string;
};

export async function fetchStatus(): Promise<string> {
	const response = await fetch("/api/%s");
	if (!response.ok) {
		throw new Error("request failed");
	}

	const payload = (await response.json()) as StatusResponse;
	return payload.status;
}
`, name)
}

func featureUsage() string {
	return `rego feature - scaffold backend/frontend feature modules

Commands:
  new <name>          Create backend and frontend feature stubs.

Examples:
  go run ./cmd/rego feature new billing
  go run ./cmd/rego feature new user_profile
`
}
