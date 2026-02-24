import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { App } from "./App";

describe("App", () => {
  beforeEach(() => {
    const now = new Date().toISOString();
    window.history.pushState({}, "", "/");

    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();
        if (url.includes("/api/metadata")) {
          return {
            ok: true,
            json: async () => ({
              entries: [{ key: "schema_version", value: "1", updated_at: now }]
            })
          };
        }

        return {
          ok: true,
          json: async () => ({
            status: "ok",
            time: now,
            database: { status: "up" }
          })
        };
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the dashboard", async () => {
    render(<App />);

    const navigation = screen.getByRole("navigation", { name: /app navigation/i });
    expect(screen.getByRole("heading", { name: /rego starter app/i })).toBeInTheDocument();
    expect(within(navigation).getByRole("link", { name: /^dashboard$/i })).toBeInTheDocument();
    expect(within(navigation).getByRole("link", { name: /^metadata$/i })).toBeInTheDocument();
    expect(await screen.findByText("OK")).toBeInTheDocument();
  });

  it("renders metadata route content", async () => {
    window.history.pushState({}, "", "/metadata");

    render(<App />);

    expect(await screen.findByText("schema_version")).toBeInTheDocument();
  });
});
