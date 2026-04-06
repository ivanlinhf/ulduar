import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import type { AppTestContext } from "./testContext";

export function registerComposerSuite(context: AppTestContext) {
  it("shows the send shortcut hint and submits on Shift+Enter", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    expect(screen.getByText("Shift + Enter to send")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Message"), "Shortcut send{Shift>}{Enter}{/Shift}");

    await waitFor(() => {
      expect(context.mockedCreateMessage).toHaveBeenCalledWith({
        sessionId: "11111111-1111-1111-1111-111111111111",
        text: "Shortcut send",
        attachments: [],
      });
    });
  });

  it("opens the expanded composer with the current text and syncs it back on outside click", async () => {
    const user = userEvent.setup();
    const { container } = context.renderApp();
    await context.waitForReady();

    const inlineComposer = screen.getByLabelText("Message");
    await user.type(inlineComposer, "Long draft");
    await user.click(screen.getByRole("button", { name: "Expand message editor" }));

    const expandedComposer = screen.getByLabelText("Expanded message");
    expect(expandedComposer).toHaveValue("Long draft");
    expect(expandedComposer).toHaveFocus();
    expect(expandedComposer).toHaveProperty("selectionStart", "Long draft".length);
    expect(expandedComposer).toHaveProperty("selectionEnd", "Long draft".length);

    await user.type(expandedComposer, " with more detail");
    fireEvent.mouseDown(container.querySelector(".composer-dialog-backdrop")!);

    const inlineComposerAfterClose = screen.getByLabelText("Message");
    expect(screen.queryByRole("dialog", { name: "Expanded message editor" })).not.toBeInTheDocument();
    expect(inlineComposerAfterClose).toHaveValue("Long draft with more detail");
    expect(inlineComposerAfterClose).toHaveFocus();
    expect(inlineComposerAfterClose).toHaveProperty("selectionStart", "Long draft with more detail".length);
    expect(inlineComposerAfterClose).toHaveProperty("selectionEnd", "Long draft with more detail".length);
  });

  it("traps focus inside the expanded composer and makes the background inert", async () => {
    const user = userEvent.setup();
    const { container } = context.renderApp();
    await context.waitForReady();

    await user.type(screen.getByLabelText("Message"), "Focus trap");
    await user.click(screen.getByRole("button", { name: "Expand message editor" }));

    const appFrame = container.querySelector(".app-frame");
    const expandedComposer = screen.getByLabelText("Expanded message");
    const dialogSendButton = screen.getByRole("button", { name: "Send" });

    expect(appFrame).toHaveAttribute("inert");
    expect(appFrame).toHaveAttribute("aria-hidden", "true");
    expect(expandedComposer).toHaveFocus();

    await user.tab();
    expect(dialogSendButton).toHaveFocus();

    await user.tab();
    expect(expandedComposer).toHaveFocus();

    await user.tab({ shift: true });
    expect(dialogSendButton).toHaveFocus();
  });

  it("submits from the expanded composer on Shift+Enter and closes the dialog", async () => {
    context.mockSuccessfulCreateMessage();

    const user = userEvent.setup();
    context.renderApp();
    await context.waitForReady();

    await user.type(screen.getByLabelText("Message"), "Expanded shortcut send");
    await user.click(screen.getByRole("button", { name: "Expand message editor" }));
    await user.type(screen.getByLabelText("Expanded message"), "{Shift>}{Enter}{/Shift}");

    await waitFor(() => {
      expect(context.mockedCreateMessage).toHaveBeenCalledWith({
        sessionId: "11111111-1111-1111-1111-111111111111",
        text: "Expanded shortcut send",
        attachments: [],
      });
    });

    expect(screen.queryByRole("dialog", { name: "Expanded message editor" })).not.toBeInTheDocument();
  });

  it("closes the expanded composer on Escape and restores the inline caret to the end", async () => {
    const user = userEvent.setup();
    context.renderApp();
    await context.waitForReady();

    await user.type(screen.getByLabelText("Message"), "abc");
    await user.click(screen.getByRole("button", { name: "Expand message editor" }));

    const expandedComposer = screen.getByLabelText("Expanded message");
    expect(expandedComposer).toHaveProperty("selectionStart", 3);

    await user.keyboard("{Escape}");

    const inlineComposer = screen.getByLabelText("Message");
    expect(screen.queryByRole("dialog", { name: "Expanded message editor" })).not.toBeInTheDocument();
    expect(inlineComposer).toHaveFocus();
    expect(inlineComposer).toHaveProperty("selectionStart", 3);
    expect(inlineComposer).toHaveProperty("selectionEnd", 3);
  });

  it("keeps plain Enter available for multiline messages", async () => {
    context.renderApp();
    await context.waitForReady();

    const composer = screen.getByLabelText("Message");
    await userEvent.type(composer, "First line{Enter}Second line");

    expect(context.mockedCreateMessage).not.toHaveBeenCalled();
    expect(composer).toHaveValue("First line\nSecond line");
  });

  it("keeps the attachment tooltip dismissed after clicking the button", async () => {
    const user = userEvent.setup();
    context.renderApp();
    await context.waitForReady();

    const attachmentButton = screen.getByRole("button", { name: "Add attachments" });

    await user.hover(attachmentButton);
    expect(screen.getByRole("tooltip")).toHaveTextContent("Add attachments");

    await user.click(attachmentButton);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.focus(attachmentButton);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent(window, new Event("focus"));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    await user.unhover(attachmentButton);
    await user.hover(attachmentButton);
    expect(screen.getByRole("tooltip")).toHaveTextContent("Add attachments");
  });

  it("shows an attachment limit toast without shifting the composer and dismisses it after 3 seconds", async () => {
    const { container } = context.renderApp();
    await context.waitForReady();

    vi.useFakeTimers();
    try {
      const fileInput = container.querySelector('input[type="file"]');
      expect(fileInput).toBeTruthy();

      const files = Array.from({ length: 6 }, (_, index) => {
        return new File(["x"], `image-${index + 1}.png`, { type: "image/png" });
      });

      fireEvent.change(fileInput as HTMLInputElement, { target: { files } });

      expect(screen.getByText("You can attach at most 5 files at once.")).toBeInTheDocument();
      expect(screen.queryByText("image-1.png")).not.toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(screen.queryByText("You can attach at most 5 files at once.")).not.toBeInTheDocument();
    } finally {
      vi.runOnlyPendingTimers();
      vi.useRealTimers();
    }
  });
}
