import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import type { SessionDetailResponse } from "../../../lib/api";
import { markdownAssistantReply } from "./fixtures";
import type { AppTestContext } from "./testContext";

export function registerRenderingSuite(context: AppTestContext) {
  let originalClipboardDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(window.navigator, "clipboard");
  });

  afterEach(() => {
    if (originalClipboardDescriptor) {
      Object.defineProperty(window.navigator, "clipboard", originalClipboardDescriptor);
    } else {
      Reflect.deleteProperty(window.navigator, "clipboard");
    }
  });

  it("renders common markdown and gfm content in assistant output", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Use markdown");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: markdownAssistantReply,
      });
      streamHandlers.onRunCompleted?.({
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
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Use nested markdown");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "```ts\nconst answer = 42",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(assistantMessage).toHaveTextContent("const answer = 42");

    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "\n```",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(assistantMessage?.querySelector("pre code")).toHaveTextContent("const answer = 42");
  });

  it("renders a fallback token badge when only input and output counts are available", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Show token counts");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
        inputTokens: 45,
        outputTokens: 78,
      });
    });

    expect(screen.getByText("in 45 / out 78")).toBeInTheDocument();
  });

  it("renders literal html-like assistant text visibly", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Show literal tags");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "<div>Hello</div>\n<svg viewBox=\"0 0 10 10\" />",
      });
      streamHandlers.onRunCompleted?.({
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

  it("shows a transient web-search status while the stream is searching and clears it on completion", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Search the web");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onToolStatus?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        toolName: "web_search",
        toolPhase: "searching",
      });
    });

    expect(screen.getByText("Searching the web...")).toBeInTheDocument();

    await act(async () => {
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
      });
    });

    await waitFor(() => {
      expect(screen.queryByText("Searching the web...")).not.toBeInTheDocument();
    });
    expect(context.mockedGetSession).toHaveBeenCalledTimes(1);
  });

  it("clears a transient web-search status when the run fails", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Search the web");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onToolStatus?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        toolName: "web_search",
        toolPhase: "searching",
      });
      streamHandlers.onRunFailed?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        error: "Run failed",
      });
    });

    expect(screen.queryByText("Searching the web...")).not.toBeInTheDocument();
  });

  it("renders a deduplicated Sources section from persisted assistant citations", async () => {
    context.mockSuccessfulCreateMessage();
    context.mockSessionMessages([
      {
        messageId: "33333333-3333-3333-3333-333333333333",
        role: "assistant",
        status: "completed",
        modelName: "gpt-5",
        createdAt: "2026-03-31T10:01:00Z",
        content: {
          parts: [
            {
              type: "text",
              text: "Answer with citations",
              citations: [
                { url: " https://example.com/docs " },
                { title: "Example Docs", url: "https://example.com/docs" },
                { title: "Ignored Empty URL", url: "   " },
                { title: "Second Source", url: "https://example.com/other" },
              ],
            },
          ],
        },
        attachments: [],
      },
    ]);

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Show sources");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onToolStatus?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        toolName: "web_search",
        toolPhase: "searching",
      });
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Answer with citations",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
      });
    });

    const sources = await screen.findByRole("region", { name: "Sources" });
    const links = within(sources).getAllByRole("link");
    expect(links).toHaveLength(2);
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      "https://example.com/docs",
      "https://example.com/other",
    ]);
    expect(within(sources).getByRole("link", { name: /Example Docs/ })).toHaveAttribute("href", "https://example.com/docs");
    expect(within(sources).getByRole("link", { name: /Example Docs/ })).toHaveAttribute("target", "_blank");
    expect(within(sources).getByRole("link", { name: /Example Docs/ })).toHaveAttribute("rel", "noreferrer noopener");
    expect(within(sources).getByRole("link", { name: /Second Source/ })).toHaveAttribute("href", "https://example.com/other");
    expect(context.mockedGetSession).toHaveBeenCalledTimes(1);
  });

  it("does not apply an old session citation refresh after starting a new chat", async () => {
    let resolveFirstSession: ((value: SessionDetailResponse) => void) | undefined;

    context.mockedCreateSession
      .mockResolvedValueOnce({
        sessionId: "11111111-1111-1111-1111-111111111111",
        status: "active",
        createdAt: "2026-03-31T10:00:00Z",
        lastMessageAt: "2026-03-31T10:00:00Z",
      })
      .mockResolvedValueOnce({
        sessionId: "99999999-9999-9999-9999-999999999999",
        status: "active",
        createdAt: "2026-03-31T10:02:00Z",
        lastMessageAt: "2026-03-31T10:02:00Z",
      });
    context.mockSuccessfulCreateMessage();
    context.mockedGetSession.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirstSession = resolve;
        }),
    );

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "First run");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    let streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onToolStatus?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        toolName: "web_search",
        toolPhase: "searching",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
      });
    });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New Chat" }));
    await screen.findByText("Ready for the next turn.");

    await userEvent.type(screen.getByLabelText("Message"), "Second run");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    streamHandlers = context.requireStreamHandlers();
    expect(screen.getByText("Second run")).toBeInTheDocument();

    await act(async () => {
      resolveFirstSession?.({
        sessionId: "11111111-1111-1111-1111-111111111111",
        status: "active",
        createdAt: "2026-03-31T10:00:00Z",
        lastMessageAt: "2026-03-31T10:01:00Z",
        messages: [
          {
            messageId: "33333333-3333-3333-3333-333333333333",
            role: "assistant",
            status: "completed",
            createdAt: "2026-03-31T10:01:00Z",
            content: {
              parts: [
                {
                  type: "text",
                  text: "Old session answer",
                  citations: [{ title: "Old Source", url: "https://example.com/old" }],
                },
              ],
            },
            attachments: [],
          },
        ],
      });
      await Promise.resolve();
    });

    expect(screen.queryByRole("region", { name: "Sources" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Old Source/ })).not.toBeInTheDocument();
    expect(context.mockedGetSession).toHaveBeenCalledTimes(1);

    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Fresh assistant reply",
      });
    });

    expect(screen.getByText("Fresh assistant reply")).toBeInTheDocument();
  });

  it("does not render a Sources section during standard non-search chat rendering", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Normal reply");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Plain assistant reply",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
      });
    });

    expect(screen.getByText("Plain assistant reply")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Sources" })).not.toBeInTheDocument();
    expect(screen.queryByText("Searching the web...")).not.toBeInTheDocument();
    expect(context.mockedGetSession).not.toHaveBeenCalled();
  });

  it("renders assistant soft line breaks with normal markdown paragraph whitespace", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "User first line{Enter}User second line");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });
    expect(context.mockedCreateMessage).toHaveBeenCalledWith({
      sessionId: "11111111-1111-1111-1111-111111111111",
      text: "User first line\nUser second line",
      attachments: [],
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "First line\nsecond line",
      });
      streamHandlers.onRunCompleted?.({
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
    context.mockSuccessfulCreateMessage();

    const scrollSpy = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
    const { container } = context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Keep streaming");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    scrollSpy.mockClear();

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
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
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: " Second chunk",
      });
    });

    expect(scrollSpy).not.toHaveBeenCalled();
  });

  it("updates assistant state when the stream fails", async () => {
    context.mockSuccessfulCreateMessage({
      createdAt: "2026-03-31T10:02:00Z",
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Summarize");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Partial answer",
      });
      streamHandlers.onRunFailed?.({
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
    context.mockSuccessfulCreateMessage({
      createdAt: "2026-03-31T10:03:00Z",
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "What happened?");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.requireStreamHandlers()).toBeTruthy();
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onTransportError?.("Streaming connection closed before completion");
    });

    expect(screen.getByText("Streaming connection closed before completion")).toBeInTheDocument();
  });

  it("renders assistant-only copy controls and disables copy until text is available", async () => {
    context.mockSuccessfulCreateMessage();
    setClipboard({
      writeText: vi.fn().mockResolvedValue(undefined),
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Copy later");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(within(assistantMessage!).getByRole("toolbar", { name: "Assistant message actions" })).toBeInTheDocument();
    expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeDisabled();

    const userMessage = screen.getByText("You").closest("article");
    expect(userMessage).not.toBeNull();
    expect(within(userMessage!).queryByRole("toolbar")).not.toBeInTheDocument();

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Assistant text",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeEnabled();
  });

  it("copies the current assistant message text and shows success feedback", async () => {
    context.mockSuccessfulCreateMessage();

    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({
      writeText,
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Copy now");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Copy this exact text",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();

    await userEvent.click(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" }));

    expect(writeText).toHaveBeenCalledWith("Copy this exact text");
    expect(within(assistantMessage!).getByText("Copied")).toBeInTheDocument();
    expect(within(assistantMessage!).getByRole("button", { name: "Copied assistant message" })).toBeInTheDocument();
  });

  it("restarts copied feedback timing when the assistant message is copied again", async () => {
    context.mockSuccessfulCreateMessage();

    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({
      writeText,
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Copy twice");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Copy this twice",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();

    vi.useFakeTimers();
    try {
      const copyButton = within(assistantMessage!).getByRole("button", { name: "Copy assistant message" });
      await act(async () => {
        fireEvent.click(copyButton);
      });

      expect(writeText).toHaveBeenCalledTimes(1);
      expect(within(assistantMessage!).getByText("Copied")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1500);
      });

      await act(async () => {
        fireEvent.click(within(assistantMessage!).getByRole("button", { name: "Copied assistant message" }));
      });

      expect(writeText).toHaveBeenCalledTimes(2);

      act(() => {
        vi.advanceTimersByTime(1500);
      });

      expect(within(assistantMessage!).getByText("Copied")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(499);
      });

      expect(within(assistantMessage!).getByText("Copied")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(within(assistantMessage!).queryByText("Copied")).not.toBeInTheDocument();
      expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps the assistant message usable when clipboard copy fails", async () => {
    context.mockSuccessfulCreateMessage();

    const writeText = vi.fn().mockRejectedValue(new Error("copy failed"));
    setClipboard({
      writeText,
    });

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Copy failure");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Still visible after failure",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();

    await userEvent.click(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" }));

    expect(writeText).toHaveBeenCalledWith("Still visible after failure");
    expect(within(assistantMessage!).getByText("Still visible after failure")).toBeInTheDocument();
    expect(within(assistantMessage!).queryByText("Copied")).not.toBeInTheDocument();
    expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeEnabled();
  });

  it("keeps assistant copy disabled when clipboard API is unavailable", async () => {
    context.mockSuccessfulCreateMessage();
    setClipboard(undefined);

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Copy unsupported");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    const assistantMessage = screen.getByText("Assistant").closest("article");
    expect(assistantMessage).not.toBeNull();
    expect(within(assistantMessage!).getByRole("toolbar", { name: "Assistant message actions" })).toBeInTheDocument();
    expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeDisabled();

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Assistant text",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(within(assistantMessage!).getByRole("button", { name: "Copy assistant message" })).toBeDisabled();
  });
}

function setClipboard(clipboard: Pick<Clipboard, "writeText"> | undefined) {
  Object.defineProperty(window.navigator, "clipboard", {
    configurable: true,
    value: clipboard,
  });
}
