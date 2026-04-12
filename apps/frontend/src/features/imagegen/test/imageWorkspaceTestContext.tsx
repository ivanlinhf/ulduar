import { render, screen } from "@testing-library/react";
import { beforeEach, expect, vi } from "vitest";

import App from "../../../App";
import { currentFrontendVersion } from "../../../lib/frontendUpdate";
import * as api from "../../../lib/api";

export function setupImageWorkspaceTestContext() {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedGetSession = vi.mocked(api.getSession);
  const mockedGetImageGenerationCapabilities = vi.mocked(api.getImageGenerationCapabilities);
  const mockedCreateMessage = vi.mocked(api.createMessage);
  const mockedCreateImageGeneration = vi.mocked(api.createImageGeneration);
  const mockedStreamRun = vi.mocked(api.streamRun);
  const mockedStreamImageGeneration = vi.mocked(api.streamImageGeneration);
  const mockedFetch = vi.fn<typeof fetch>();

  let imageStreamHandlers: Parameters<typeof api.streamImageGeneration>[2] | undefined;
  let localIdCounter = 0;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockedFetch);
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      writable: true,
      value: vi.fn(() => `blob:mock-${Math.random().toString(36).slice(2)}`),
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      writable: true,
      value: vi.fn(),
    });
    imageStreamHandlers = undefined;
    mockedFetch.mockReset();
    mockedFetch.mockResolvedValue(createVersionResponse(currentFrontendVersion));

    // First createSession call is the chat session; subsequent calls are image sessions.
    mockedCreateSession
      .mockResolvedValueOnce({
        sessionId: "11111111-1111-1111-1111-111111111111",
        status: "active",
        createdAt: "2026-03-31T10:00:00Z",
        lastMessageAt: "2026-03-31T10:00:00Z",
      })
      .mockResolvedValue({
        sessionId: "22222222-2222-2222-2222-222222222222",
        status: "active",
        createdAt: "2026-03-31T10:01:00Z",
        lastMessageAt: "2026-03-31T10:01:00Z",
      });

    mockedGetSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:01:00Z",
      messages: [],
    });

    mockedGetImageGenerationCapabilities.mockResolvedValue({
      modes: ["text_to_image", "image_edit"],
      resolutions: [
        { key: "1024x1024", width: 1024, height: 1024 },
        { key: "1792x1024", width: 1792, height: 1024 },
      ],
      maxReferenceImages: 4,
      outputImageCount: 1,
      providerName: "azure-foundry",
    });

    mockedStreamRun.mockImplementation((_sessionId, _runId, _handlers) => {
      return vi.fn();
    });

    mockedStreamImageGeneration.mockImplementation((_sessionId, _generationId, handlers) => {
      imageStreamHandlers = handlers;
      return vi.fn();
    });

    localIdCounter = 0;
    vi.spyOn(globalThis.crypto, "randomUUID").mockImplementation(() => {
      localIdCounter += 1;
      return `00000000-0000-4000-8000-${String(localIdCounter).padStart(12, "0")}`;
    });
  });

  function renderApp() {
    return render(<App />);
  }

  function mockSuccessfulCreateImageGeneration(
    overrides: Partial<api.CreateImageGenerationResponse> = {},
  ) {
    mockedCreateImageGeneration.mockResolvedValue({
      generationId: "gen-44444444-4444-4444-4444-444444444444",
      status: "pending",
      createdAt: "2026-03-31T10:01:00Z",
      ...overrides,
    });
  }

  async function waitForReady() {
    return screen.findByText("Ready for the next turn.");
  }

  async function waitForImageReady() {
    return screen.findByText("Ready to generate.");
  }

  function requireImageStreamHandlers() {
    expect(imageStreamHandlers).toBeTruthy();
    return imageStreamHandlers!;
  }

  return {
    mockSuccessfulCreateImageGeneration,
    mockedCreateImageGeneration,
    mockedCreateMessage,
    mockedCreateSession,
    mockedFetch,
    mockedGetImageGenerationCapabilities,
    mockedGetSession,
    mockedStreamImageGeneration,
    renderApp,
    requireImageStreamHandlers,
    waitForImageReady,
    waitForReady,
  };
}

export type ImageWorkspaceTestContext = ReturnType<typeof setupImageWorkspaceTestContext>;

function createVersionResponse(version: string, init: { ok?: boolean } = {}) {
  const status = init.ok === false ? 503 : 200;
  return new Response(JSON.stringify({ version }), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}
