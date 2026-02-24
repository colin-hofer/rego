export type HealthPayload = {
  status: string;
  time: string;
  database?: {
    status: string;
  };
};

export async function fetchHealth(signal?: AbortSignal): Promise<HealthPayload> {
  const response = await fetch("/api/healthz", { signal });
  if (!response.ok) {
    throw new Error(`unexpected status: ${response.status}`);
  }

  return (await response.json()) as HealthPayload;
}
