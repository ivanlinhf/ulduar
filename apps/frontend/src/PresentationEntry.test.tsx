import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { APIError } from "./lib/api";
import App from "./App";
import * as api from "./lib/api";
import { currentFrontendVersion } from "./lib/frontendUpdate";

vi.mock("./lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./lib/api")>();
  return {
    ...actual,
    createSession: vi.fn(),
    getSession: vi.fn(),
    getImageGenerationCapabilities: vi.fn(),
    getPresentationGenerationCapabilities: vi.fn(),
    createMessage: vi.fn(),
    streamRun: vi.fn(),
  };
});

vi.mock("./lib/config", () => ({
  apiBaseURL: "http://localhost:8080",
  isImageGenerationEnabled: false,
  isPresentationGenerationEnabled: true,
  frontendConfig: {
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: false,
    isPresentationGenerationEnabled: true,
  },
  createFrontendConfig: vi.fn(() => ({
    apiBaseURL: "http://localhost:8080",
    isImageGenerationEnabled: false,
    isPresentationGenerationEnabled: true,
  })),
}));

describe("App presentation entry", () => {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedGetSession = vi.mocked(api.getSession);
  const mockedGetImageGenerationCapabilities = vi.mocked(api.getImageGenerationCapabilities);
  const mockedGetPresentationGenerationCapabilities = vi.mocked(
    api.getPresentationGenerationCapabilities,
  );
  const mockedStreamRun = vi.mocked(api.streamRun);
  const mockedFetch = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockedFetch);
    mockedFetch.mockResolvedValue(
      new Response(JSON.stringify({ version: currentFrontendVersion }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    mockedCreateSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:00:00Z",
    });
    mockedGetSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:01:00Z",
      messages: [],
    });
    mockedGetImageGenerationCapabilities.mockResolvedValue({
      modes: ["text_to_image"],
      resolutions: [{ key: "1024x1024", width: 1024, height: 1024 }],
      maxReferenceImages: 4,
      outputImageCount: 1,
      providerName: "azure-foundry",
    });
    mockedStreamRun.mockImplementation(() => vi.fn());
  });

  it("enters presentation workspace mode when the feature is enabled and available", async () => {
    mockedGetPresentationGenerationCapabilities.mockResolvedValue({
      inputMediaTypes: ["application/pdf"],
      outputMediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      providerName: "azure-openai",
    });

    const { container } = render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New Presentation" }));

    await waitFor(() => {
      expect(container.querySelector('[data-workspace="presentation"]')).toBeTruthy();
    });
  });

  it("shows New Presentation disabled when capabilities bootstrap returns 503", async () => {
    mockedGetPresentationGenerationCapabilities.mockRejectedValue(
      new APIError(503, "presentation generation is not configured"),
    );

    const { container } = render(<App />);
    await screen.findByText("Ready for the next turn.");

    await waitFor(() => {
      expect(
        container.querySelector('[data-presentation-generation-bootstrap-state="unavailable-503"]'),
      ).toBeTruthy();
    });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const item = screen.getByRole("menuitem", { name: "New Presentation" });

    expect(item).toHaveAttribute("aria-disabled", "true");
    await userEvent.click(item);
    expect(container.querySelector('[data-workspace="presentation"]')).toBeFalsy();
  });
});
