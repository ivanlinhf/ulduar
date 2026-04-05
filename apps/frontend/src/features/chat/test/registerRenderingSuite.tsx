import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { markdownAssistantReply } from "./fixtures";
import type { AppTestContext } from "./testContext";

export function registerRenderingSuite(context: AppTestContext) {
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
}
