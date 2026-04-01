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

  it("renders common markdown and gfm content in assistant output", async () => {
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
        delta: [
          "# Plan",
          "",
          "Intro with **bold**, *italic*, ~~strikethrough~~, and a [Docs](https://example.com) link.",
          "",
          "1. First item",
          "2. Second item",
          "",
          "- Bullet one",
          "- Bullet two",
          "",
          "> Helpful note",
          "",
          "- [x] Done",
          "- [ ] Pending",
          "",
          "| Name | Value |",
          "| --- | --- |",
          "| Alpha | 1 |",
          "",
          "```ts",
          "const answer = 42;",
          "```",
        ].join("\n"),
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(screen.getByRole("heading", { level: 1, name: "Plan" })).toBeInTheDocument();
    expect(screen.getByText("First item")).toBeInTheDocument();
    expect(screen.getByText("Bullet one")).toBeInTheDocument();
    expect(assistantMessage?.querySelector("strong")).toHaveTextContent("bold");
    expect(assistantMessage?.querySelector("em")).toHaveTextContent("italic");
    expect(assistantMessage?.querySelector("del")).toHaveTextContent("strikethrough");
    expect(screen.getByRole("link", { name: "Docs" })).toHaveAttribute("href", "https://example.com");
    expect(screen.getByRole("link", { name: "Docs" })).toHaveAttribute("target", "_blank");
    expect(screen.getByRole("link", { name: "Docs" })).toHaveAttribute("rel", "noreferrer noopener");
    expect(assistantMessage?.querySelector("blockquote")).toHaveTextContent("Helpful note");
    expect(assistantMessage?.querySelector("pre code")).toHaveTextContent("const answer = 42;");
    expect(assistantMessage?.querySelector("table")).toHaveTextContent("Alpha");
    const taskItems = assistantMessage?.querySelectorAll('input[type="checkbox"]');
    expect(taskItems).toHaveLength(2);
    expect(taskItems?.[0]).toBeChecked();
    expect(taskItems?.[1]).not.toBeChecked();
  });

  it("keeps rendering when streamed markdown is incomplete", async () => {
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
        delta: "```ts\nconst answer = 42",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(assistantMessage).toHaveTextContent("const answer = 42");

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "\n```",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(assistantMessage?.querySelector("pre code")).toHaveTextContent("const answer = 42");
  });

  it("renders literal html-like assistant text visibly", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Show literal tags");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "<div>Hello</div>\n<svg viewBox=\"0 0 10 10\" />",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(assistantMessage).toHaveTextContent("<div>Hello</div>");
    expect(assistantMessage).toHaveTextContent('<svg viewBox="0 0 10 10" />');
  });

  it("renders assistant soft line breaks with normal markdown paragraph whitespace", async () => {
    mockedCreateMessage.mockResolvedValue({
      runId: "44444444-4444-4444-4444-444444444444",
      userMessageId: "22222222-2222-2222-2222-222222222222",
      assistantMessageId: "33333333-3333-3333-3333-333333333333",
      createdAt: "2026-03-31T10:01:00Z",
    });

    render(<App />);
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "User first line{Enter}User second line");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(streamHandlers).toBeTruthy();
    });
    expect(mockedCreateMessage).toHaveBeenCalledWith({
      sessionId: "11111111-1111-1111-1111-111111111111",
      text: "User first line\nUser second line",
      attachments: [],
    });

    await act(async () => {
      streamHandlers?.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "First line\nsecond line",
      });
      streamHandlers?.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const userMessage = screen.getByText("You").closest("article");
    const userParagraph = userMessage?.querySelector(".message-body > p");
    expect(userParagraph).not.toBeNull();

    const assistantMessage = screen.getByText("Assistant").closest("article");
    const assistantParagraph = assistantMessage?.querySelector(".message-markdown p");
    expect(assistantParagraph).not.toBeNull();
    expect(assistantParagraph).toHaveTextContent("First line second line");
    expect(assistantMessage?.querySelectorAll(".message-markdown p")).toHaveLength(1);
    expect(assistantMessage?.querySelector(".message-markdown br")).toBeNull();
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
