import { useEffect, useState } from "react";

type HealthPayload = {
  status: string;
  time: string;
};

export function App() {
  const [health, setHealth] = useState<HealthPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadHealth() {
      try {
        const response = await fetch("/api/healthz");
        if (!response.ok) {
          throw new Error(`unexpected status: ${response.status}`);
        }

        const payload = (await response.json()) as HealthPayload;
        if (!cancelled) {
          setHealth(payload);
          setError(null);
        }
      } catch (loadError) {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : "request failed");
        }
      }
    }

    loadHealth();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="page">
      <div className="glow glow-a" />
      <div className="glow glow-b" />

      <section className="hero">
        <p className="eyebrow">Go-Centric React Starter</p>
        <h1>Rego starter app</h1>
        <p className="lede">
          Backend routing, build orchestration, dev hot reload, and production asset serving all run through Go.
        </p>

        <div className="status-card">
          <p className="status-label">Backend health</p>
          {health ? (
            <>
              <p className="status-ok">{health.status.toUpperCase()}</p>
              <p className="status-meta">{new Date(health.time).toLocaleString()}</p>
            </>
          ) : (
            <p className="status-meta">Checking...</p>
          )}
          {error ? <p className="status-error">{error}</p> : null}
        </div>
      </section>
    </main>
  );
}
