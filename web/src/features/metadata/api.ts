export type MetadataEntry = {
  key: string;
  value: string;
  updated_at: string;
};

type MetadataListResponse = {
  entries: MetadataEntry[];
};

type MetadataUpsertResponse = {
  entry: MetadataEntry;
};

export async function fetchMetadata(signal?: AbortSignal): Promise<MetadataEntry[]> {
  const response = await fetch("/api/metadata", { signal });
  if (!response.ok) {
    throw new Error(`metadata request failed: ${response.status}`);
  }

  const payload = (await response.json()) as MetadataListResponse;
  return payload.entries ?? [];
}

export async function saveMetadata(key: string, value: string): Promise<MetadataEntry> {
  const response = await fetch("/api/metadata", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ key, value })
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `metadata save failed: ${response.status}`);
  }

  const payload = (await response.json()) as MetadataUpsertResponse;
  return payload.entry;
}
