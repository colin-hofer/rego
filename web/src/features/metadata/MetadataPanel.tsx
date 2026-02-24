import { FormEvent, useEffect, useMemo, useState } from "react";
import { fetchMetadata, saveMetadata, type MetadataEntry } from "./api";

export function MetadataPanel() {
  const [entries, setEntries] = useState<MetadataEntry[]>([]);
  const [keyInput, setKeyInput] = useState("feature_flag");
  const [valueInput, setValueInput] = useState("enabled");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetchMetadata(controller.signal)
      .then((items) => {
        setEntries(items);
        setError(null);
      })
      .catch((loadError: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setError(loadError instanceof Error ? loadError.message : "failed to load metadata");
      });

    return () => {
      controller.abort();
    };
  }, []);

  const sortedEntries = useMemo(
    () => [...entries].sort((a, b) => a.key.localeCompare(b.key)),
    [entries]
  );

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const key = keyInput.trim();
    if (!key) {
      setError("Key is required.");
      return;
    }

    setSaving(true);
    try {
      const entry = await saveMetadata(key, valueInput);
      setEntries((current) => {
        const withoutKey = current.filter((item) => item.key !== entry.key);
        return [...withoutKey, entry];
      });
      setError(null);
    } catch (saveError: unknown) {
      setError(saveError instanceof Error ? saveError.message : "failed to save metadata");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="panel">
      <h2>Metadata module</h2>
      <p className="panel-copy">Example feature module with backend persistence and frontend UI.</p>

      <form className="metadata-form" onSubmit={handleSubmit}>
        <label>
          Key
          <input value={keyInput} onChange={(event) => setKeyInput(event.target.value)} placeholder="feature_flag" />
        </label>

        <label>
          Value
          <input value={valueInput} onChange={(event) => setValueInput(event.target.value)} placeholder="enabled" />
        </label>

        <button type="submit" disabled={saving}>
          {saving ? "Saving..." : "Upsert"}
        </button>
      </form>

      {error ? <p className="status-error">{error}</p> : null}

      <div className="metadata-list">
        {sortedEntries.map((entry) => (
          <article key={entry.key} className="metadata-item">
            <p className="metadata-key">{entry.key}</p>
            <p className="metadata-value">{entry.value}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
