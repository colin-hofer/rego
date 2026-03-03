import { HealthCard } from "../system/HealthCard";

export function HomePage() {
  return (
    <main className="page">
      <div className="glow glow-a" />
      <div className="glow glow-b" />

      <section className="hero">
        <p className="eyebrow">Go + React + Postgres</p>
        <h1>Rego starter</h1>
        <p className="lede">
          Lean baseline for prototyping. Keep backend routes in <code>internal/server</code>, keep UI code in{" "}
          <code>web/src/features</code>, and ship fast.
        </p>

        <section className="app-content">
          <div className="feature-grid">
            <HealthCard />

            <section className="panel">
              <h2>Quick start</h2>
              <p className="panel-copy">Core flow for daily development.</p>
              <p className="status-meta">
                <code>go run ./cmd/rego dev</code>
              </p>
              <p className="status-meta">
                <code>go run ./cmd/rego test</code>
              </p>
              <p className="status-meta">
                <code>go run ./cmd/rego build --output bin/rego</code>
              </p>
            </section>
          </div>
        </section>
      </section>
    </main>
  );
}
