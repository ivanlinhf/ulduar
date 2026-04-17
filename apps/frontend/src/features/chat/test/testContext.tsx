import { render, screen } from "@testing-library/react";
import { beforeEach, expect, vi } from "vitest";

import App from "../../../App";
import { currentFrontendVersion } from "../../../lib/frontendUpdate";
import * as api from "../../../lib/api";

export function setupAppTestContext() {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedGetSession = vi.mocked(api.getSession);
  const mockedGetImageGenerationCapabilities = vi.mocked(api.getImageGenerationCapabilities);
  const mockedGetPresentationGenerationCapabilities = vi.mocked(
    api.getPresentationGenerationCapabilities,
  );
  const mockedCreateMessage = vi.mocked(api.createMessage);
  const mockedStreamRun = vi.mocked(api.streamRun);
  const mockedFetch = vi.fn<typeof fetch>();

  let streamHandlers: Parameters<typeof api.streamRun>[2] | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockedFetch);
    streamHandlers = undefined;
    mockedFetch.mockReset();
    mockedFetch.mockResolvedValue(createVersionResponse(currentFrontendVersion));

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
    mockedGetPresentationGenerationCapabilities.mockResolvedValue({
      inputMediaTypes: ["application/pdf"],
      outputMediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      providerName: "azure-openai",
    });
    mockedStreamRun.mockImplementation((_sessionId, _runId, handlers) => {
      streamHandlers = handlers;
      return vi.fn();
    });

    vi.spyOn(globalThis.crypto, "randomUUID")
      .mockReturnValueOnce("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
      .mockReturnValueOnce("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
      .mockReturnValue("cccccccc-cccc-cccc-cccc-cccccccccccc");
  });

  function renderApp() {
    return render(<App />);
  }

  function mockSuccessfulCreateMessage(overrides: Partial<api.CreateMessageResponse> = {}) {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
      ...overrides,
    });
  }

  function mockSessionMessages(messages: api.SessionDetailResponse["messages"]) {
    mockedGetSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:01:00Z",
      messages,
    });
  }

  async function waitForReady() {
    return screen.findByText("Ready for the next turn.");
  }

  function requireStreamHandlers() {
    expect(streamHandlers).toBeTruthy();
    return streamHandlers!;
  }

  return {
    mockFrontendVersion,
    mockSuccessfulCreateMessage,
    mockSessionMessages,
    mockedCreateMessage,
    mockedCreateSession,
    mockedFetch,
    mockedGetImageGenerationCapabilities,
    mockedGetPresentationGenerationCapabilities,
    mockedGetSession,
    renderApp,
    requireStreamHandlers,
    waitForReady,
  };

  function mockFrontendVersion(version: string, init: { ok?: boolean } = {}) {
    mockedFetch.mockResolvedValue(createVersionResponse(version, init));
  }
}

export type AppTestContext = ReturnType<typeof setupAppTestContext>;

function createVersionResponse(version: string, init: { ok?: boolean } = {}) {
  const status = init.ok === false ? 503 : 200;
  return new Response(JSON.stringify({ version }), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}
