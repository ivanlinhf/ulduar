import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App";
import * as api from "./lib/api";

vi.mock("./lib/api", () => ({
  createSession: vi.fn(),
  createMessage: vi.fn(),
  streamRun: vi.fn(),
}));

describe("App", () => {
  const mockedCreateSession = vi.mocked(api.createSession);
  const mockedCreateMessage = vi.mocked(api.createMessage);
  const mockedStreamRun = vi.mocked(api.streamRun);

  let streamHandlers:
    | Parameters<typeof api.streamRun>[2]
    | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    streamHandlers = undefined;

    mockedCreateSession.mockResolvedValue({
      sessionId: "11111111-1111-1111-1111-111111111111",
      status: "active",
      createdAt: "2026-03-31T10:00:00Z",
      lastMessageAt: "2026-03-31T10:00:00Z",
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

  it("creates a session on load", async () => {
    render(<App />);

    expect(mockedCreateSession).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Ready for the next turn.")).toBeInTheDocument();
    expect(screen.getByText("11111111")).toBeInTheDocument();
  });

  it("sends a message and renders streamed assistant output", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Explain this");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(mockedCreateMessage).toHaveBeenCalledWith({
        sessionId: "11111111-1111-1111-1111-111111111111",
        text: "Explain this",
        attachments: [],
      });
    });

    await act(async () => {
      streamHandlers?.onRunStarted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        responseId: "resp_123",
        modelName: "gpt-5",
      });
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Assistant reply",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(screen.getByText("Explain this")).toBeInTheDocument();
    expect(screen.getByText("Assistant reply")).toBeInTheDocument();
    expect(screen.getByText("gpt-5")).toBeInTheDocument();
  });

  it("renders markdown emphasis in assistant output", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Use markdown");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "This is **bold** and *italic* text",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(assistantMessage).toHaveTextContent("This is bold and italic text");
    expect(screen.getByText("bold", { selector: "strong" })).toBeInTheDocument();
    expect(screen.getByText("italic", { selector: "em" })).toBeInTheDocument();
  });

  it("supports escaped asterisks and nested emphasis in assistant output", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Use nested markdown");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Use \\*literal\\* with **bold and *nested italic*** text",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(assistantMessage).toHaveTextContent("Use *literal* with bold and nested italic text");
    expect(assistantMessage?.querySelector("strong")).toHaveTextContent("bold and nested italic");
    expect(assistantMessage?.querySelector("em")).toHaveTextContent("nested italic");
  });

  it("auto-scrolls streamed responses until the user scrolls away", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    const scrollSpy = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
    const { container } = render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Keep streaming");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    scrollSpy.mockClear();

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "First chunk",
      });
    });

    expect(scrollSpy).toHaveBeenCalled();

    const messageList = container.querySelector(".message-list");
    expect(messageList).not.toBeNull();

    Object.defineProperty(messageList, "scrollHeight", {
      configurable: true,
      value: 1000,
    });
    Object.defineProperty(messageList, "clientHeight", {
      configurable: true,
      value: 300,
    });
    Object.defineProperty(messageList, "scrollTop", {
      configurable: true,
      value: 500,
    });

    scrollSpy.mockClear();

    await act(async () => {
      fireEvent.scroll(messageList!);
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: " Second chunk",
      });
    });

    expect(scrollSpy).not.toHaveBeenCalled();
  });

  it("shows the send shortcut hint and submits on Shift+Enter", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    expect(screen.getByText("Shift + Enter to send")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Message"), "Shortcut send{Shift>}{Enter}{/Shift}");

    await waitFor(() => {
      expect(mockedCreateMessage).toHaveBeenCalledWith({
        sessionId: "11111111-1111-1111-1111-111111111111",
        text: "Shortcut send",
        attachments: [],
      });
    });
  });

  it("keeps plain Enter available for multiline messages", async () => {
    render(<App />);
    await screen.findByText("Ready for the next turn.");

    const composer = screen.getByLabelText("Message");
    await userEvent.type(composer, "First line{Enter}Second line");

    expect(mockedCreateMessage).not.toHaveBeenCalled();
    expect(composer).toHaveValue("First line\nSecond line");
  });

  it("updates assistant state when the stream fails", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:02:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Summarize");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Partial answer",
      });
      streamHandlers?.onRunFailed?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        error: "provider stream failed",
        errorCode: "provider_stream_failed",
      });
    });

    expect(screen.getByText("Partial answer")).toBeInTheDocument();
    expect(screen.getByText("provider stream failed")).toBeInTheDocument();
    expect(screen.getAllByText("failed").length).toBeGreaterThan(0);
  });

  it("marks the assistant message failed on transport error", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:03:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "What happened?");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    await act(async () => {
      streamHandlers?.onTransportError?.("Streaming connection closed before completion");
    });

    expect(screen.getByText("Streaming connection closed before completion")).toBeInTheDocument();
  });
});
