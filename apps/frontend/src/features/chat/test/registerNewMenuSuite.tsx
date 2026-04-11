import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";

import type { AppTestContext } from "./testContext";

export function registerNewMenuSuite(context: AppTestContext) {
  it("renders the New trigger button with menu closed by default", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-haspopup", "menu");
    expect(trigger).toHaveAttribute("aria-expanded", "false");

    // Menu is closed: items are hidden from the accessibility tree via aria-hidden
    expect(screen.queryByRole("menuitem", { name: "New chat" })).toBeNull();
  });

  it("opens the menu and shows New chat item", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("menuitem", { name: "New chat" })).toBeInTheDocument();
  });

  it("closes the menu on second trigger click", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("menuitem", { name: "New chat" })).toBeNull();
  });

  it("closes the menu on Escape and restores focus to the trigger", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await userEvent.keyboard("{Escape}");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("closes the menu on outside pointer-down", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await userEvent.click(document.body);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("starts a new chat when New chat is selected and closes the menu", async () => {
    context.renderApp();
    await context.waitForReady();

    const initialCallCount = context.mockedCreateSession.mock.calls.length;

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New chat" }));

    expect(screen.getByRole("button", { name: "New" })).toHaveAttribute("aria-expanded", "false");
    expect(context.mockedCreateSession.mock.calls.length).toBeGreaterThan(initialCallCount);
  });

  it("focuses the first menu item when opened via keyboard", async () => {
    context.renderApp();
    await context.waitForReady();

    const trigger = screen.getByRole("button", { name: "New" });
    trigger.focus();
    await userEvent.keyboard("{Enter}");

    expect(screen.getByRole("menuitem", { name: "New chat" })).toHaveFocus();
  });

  it("does not show New image item when image generation is disabled by default", async () => {
    context.renderApp();
    await context.waitForReady();

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    // VITE_IMAGE_GENERATION_ENABLED is not set in test env, so New image item is absent
    expect(screen.queryByRole("menuitem", { name: "New image" })).not.toBeInTheDocument();
  });
}
