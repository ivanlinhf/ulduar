import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import * as browser from "../../../lib/browser";
import * as frontendUpdate from "../../../lib/frontendUpdate";
import type { AppTestContext } from "./testContext";

export function registerUpdateSuite(context: AppTestContext) {
  it("checks for frontend updates on load, when the tab becomes visible, when the browser comes online, and on an interval", async () => {
    vi.useFakeTimers();
    let visibilityState: DocumentVisibilityState = "visible";
    const originalVisibilityDescriptor = Object.getOwnPropertyDescriptor(document, "visibilityState");

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => visibilityState,
    });

    try {
      context.renderApp();
      await act(async () => {
        await Promise.resolve();
      });

      expect(context.mockedFetch).toHaveBeenCalledTimes(1);

      visibilityState = "hidden";
      fireEvent(document, new Event("visibilitychange"));
      expect(context.mockedFetch).toHaveBeenCalledTimes(1);

      visibilityState = "visible";
      fireEvent(document, new Event("visibilitychange"));
      await act(async () => {
        await Promise.resolve();
      });
      expect(context.mockedFetch).toHaveBeenCalledTimes(2);

      fireEvent(window, new Event("online"));
      await act(async () => {
        await Promise.resolve();
      });
      expect(context.mockedFetch).toHaveBeenCalledTimes(3);

      await act(async () => {
        vi.advanceTimersByTime(frontendUpdate.versionCheckIntervalMs);
        await Promise.resolve();
      });

      expect(context.mockedFetch).toHaveBeenCalledTimes(4);
    } finally {
      if (originalVisibilityDescriptor) {
        Object.defineProperty(document, "visibilityState", originalVisibilityDescriptor);
      }
      vi.useRealTimers();
    }
  });

  it("shows a reload notification and reloads immediately when there are no user turns", async () => {
    context.mockFrontendVersion("new-frontend-version");

    const reloadSpy = vi.spyOn(browser, "reloadWindow").mockImplementation(() => undefined);
    const confirmSpy = vi.spyOn(window, "confirm");

    try {
      context.renderApp();
      await context.waitForReady();

      expect(await screen.findByText("A newer version of Ulduar is available.")).toBeInTheDocument();
      expect(screen.getByText("Reload when you're ready to use the latest version.")).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Reload" }));

      expect(confirmSpy).not.toHaveBeenCalled();
      expect(reloadSpy).toHaveBeenCalledTimes(1);
    } finally {
      reloadSpy.mockRestore();
      confirmSpy.mockRestore();
    }
  });

  it("warns before reloading when the current session already has user turns", async () => {
    context.mockSuccessfulCreateMessage();

    const reloadSpy = vi.spyOn(browser, "reloadWindow").mockImplementation(() => undefined);
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true);

    try {
      context.renderApp();
      await context.waitForReady();

      await userEvent.type(screen.getByLabelText("Message"), "Keep this chat");
      await userEvent.click(screen.getByRole("button", { name: "Send" }));

      await waitFor(() => {
        expect(context.mockedCreateMessage).toHaveBeenCalledTimes(1);
      });

      context.mockFrontendVersion("new-frontend-version");
      fireEvent(window, new Event("online"));

      expect(await screen.findByText("Reloading will start a new chat and lose this session.")).toBeInTheDocument();

      const reloadButton = screen.getByRole("button", { name: "Reload" });
      await userEvent.click(reloadButton);
      expect(confirmSpy).toHaveBeenCalledWith("Reloading will start a new chat and lose this session. Reload now?");
      expect(reloadSpy).not.toHaveBeenCalled();

      await userEvent.click(reloadButton);
      expect(reloadSpy).toHaveBeenCalledTimes(1);
    } finally {
      reloadSpy.mockRestore();
      confirmSpy.mockRestore();
    }
  });
}
