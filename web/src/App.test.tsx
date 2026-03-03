import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { App } from "./App";

describe("App", () => {
  beforeEach(() => {
    const now = new Date().toISOString();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          status: "ok",
          time: now,
          database: { status: "up" }
        })
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the landing page", async () => {
    render(<App />);

    expect(screen.getByRole("heading", { name: /rego starter/i })).toBeInTheDocument();
    expect(screen.getByText(/go run \.\/cmd\/rego dev/i)).toBeInTheDocument();
    expect(await screen.findByText("OK")).toBeInTheDocument();
    expect(globalThis.fetch).toHaveBeenCalledWith("/api/healthz", expect.any(Object));
  });
});
