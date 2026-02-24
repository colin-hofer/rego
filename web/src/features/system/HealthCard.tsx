import { useEffect, useState } from "react";
import { fetchHealth, type HealthPayload } from "./api";

export function HealthCard() {
  const [health, setHealth] = useState<HealthPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    fetchHealth(controller.signal)
      .then((payload) => {
        setHealth(payload);
        setError(null);
      })
      .catch((loadError: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setError(loadError instanceof Error ? loadError.message : "request failed");
      });

    return () => {
      controller.abort();
    };
  }, []);

  return (
    <section className="panel">
      <h2>System status</h2>
      <p className="panel-copy">Quick runtime checks from the backend system module.</p>

      {health ? (
        <>
          <p className="status-ok">{health.status.toUpperCase()}</p>
          <p className="status-meta">Database {health.database?.status ? health.database.status.toUpperCase() : "UNKNOWN"}</p>
          <p className="status-meta">{new Date(health.time).toLocaleString()}</p>
        </>
      ) : (
        <p className="status-meta">Checking...</p>
      )}

      {error ? <p className="status-error">{error}</p> : null}
    </section>
  );
}
