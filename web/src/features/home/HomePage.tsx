import { Link } from "react-router-dom";
import { HealthCard } from "../system/HealthCard";

export function HomePage() {
  return (
    <div className="feature-grid">
      <HealthCard />
      <section className="panel">
        <h2>Feature workflow</h2>
        <p className="panel-copy">
          Keep backend logic in <code>internal/modules</code> and frontend logic in <code>web/src/features</code>.
        </p>
        <p className="status-meta">
          Use <code>go run ./cmd/rego feature new orders</code> to scaffold a new module pair.
        </p>
        <Link className="panel-action" to="/metadata">
          Open metadata example
        </Link>
      </section>
    </div>
  );
}
