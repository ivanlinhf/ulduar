import { render, screen } from "@testing-library/react";
import { beforeEach, expect, vi } from "vitest";

import App from "../../../App";
import * as api from "../../../lib/api";

export function setupAppTestContext() {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedGetSession = vi.mocked(api.getSession);
  const mockedCreateMessage = vi.mocked(api.createMessage);
  const mockedStreamRun = vi.mocked(api.streamRun);

  let streamHandlers: Parameters<typeof api.streamRun>[2] | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    streamHandlers = undefined;

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
    mockSuccessfulCreateMessage,
    mockSessionMessages,
    mockedCreateMessage,
    mockedCreateSession,
    mockedGetSession,
    renderApp,
    requireStreamHandlers,
    waitForReady,
  };
}

export type AppTestContext = ReturnType<typeof setupAppTestContext>;
