import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { App } from "./App";

describe("App", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ status: "ok", time: new Date().toISOString() })
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the starter heading", async () => {
    render(<App />);

    expect(screen.getByRole("heading", { name: /rego starter app/i })).toBeInTheDocument();
    expect(await screen.findByText("OK")).toBeInTheDocument();
  });
});
