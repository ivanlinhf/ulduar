import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";

import type { AppTestContext } from "./testContext";

export function registerSessionSuite(context: AppTestContext) {
  it("creates a session on load", async () => {
    context.renderApp();

    expect(context.mockedCreateSession).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Ready for the next turn.")).toBeInTheDocument();
    expect(screen.getByText("11111111")).toBeInTheDocument();
  });

  it("sends a message and renders streamed assistant output", async () => {
    context.mockSuccessfulCreateMessage();

    context.renderApp();
    await context.waitForReady();

    await userEvent.type(screen.getByLabelText("Message"), "Explain this");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(context.mockedCreateMessage).toHaveBeenCalledWith({
        sessionId: "11111111-1111-1111-1111-111111111111",
        text: "Explain this",
        attachments: [],
      });
    });

    const streamHandlers = context.requireStreamHandlers();
    await act(async () => {
      streamHandlers.onRunStarted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        responseId: "resp_123",
        modelName: "gpt-5",
      });
      streamHandlers.onMessageDelta?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        delta: "Assistant reply",
      });
      streamHandlers.onRunCompleted?.({
        runId: "44444444-4444-4444-4444-444444444444",
        messageId: "33333333-3333-3333-3333-333333333333",
        modelName: "gpt-5",
      });
    });

    expect(screen.getByText("Explain this")).toBeInTheDocument();
    expect(screen.getByText("Assistant reply")).toBeInTheDocument();
    expect(screen.getByText("gpt-5")).toBeInTheDocument();
  });
}
