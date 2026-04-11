import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ComponentProps } from "react";

import { NewMenu } from "./NewMenu";

function renderNewMenu(props: Partial<ComponentProps<typeof NewMenu>> = {}) {
  const defaults = {
    isImageGenerationEnabled: false,
    isImageGenerationAvailable: false,
    onNewChat: vi.fn(),
    onNewImage: vi.fn(),
  };
  return render(<NewMenu {...defaults} {...props} />);
}

describe("NewMenu", () => {
  it("renders a trigger button with correct ARIA attributes when closed", () => {
    renderNewMenu();
    const trigger = screen.getByRole("button", { name: "New" });
    expect(trigger).toHaveAttribute("aria-haspopup", "menu");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("opens the menu and focuses the first item on click", async () => {
    renderNewMenu();
    await userEvent.click(screen.getByRole("button", { name: "New" }));
    expect(screen.getByRole("button", { name: "New" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("menuitem", { name: "New chat" })).toHaveFocus();
  });

  it("calls onNewChat and closes menu when New chat item is clicked, returning focus to trigger", async () => {
    const onNewChat = vi.fn();
    renderNewMenu({ onNewChat });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New chat" }));

    const trigger = screen.getByRole("button", { name: "New" });
    expect(onNewChat).toHaveBeenCalledTimes(1);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("closes the menu via Enter on the trigger when focus has returned to the trigger", async () => {
    renderNewMenu();
    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    // Return focus to trigger (e.g. via Shift+Tab) while menu stays open.
    trigger.focus();
    await userEvent.keyboard("{Enter}");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("closes on Escape and returns focus to the trigger", async () => {
    renderNewMenu();
    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    await userEvent.keyboard("{Escape}");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("restores focus to the trigger when the menu is closed by an outside pointerdown while focus is on a menuitem", async () => {
    renderNewMenu();
    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    expect(screen.getByRole("menuitem", { name: "New chat" })).toHaveFocus();

    // Simulate an outside pointerdown on a non-focusable element (e.g. body).
    fireEvent.pointerDown(document.body);

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("navigates items with ArrowDown and ArrowUp", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const items = screen.getAllByRole("menuitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveFocus();

    await userEvent.keyboard("{ArrowDown}");
    expect(items[1]).toHaveFocus();

    await userEvent.keyboard("{ArrowUp}");
    expect(items[0]).toHaveFocus();
  });

  it("opens the menu via ArrowUp on the trigger and focuses the last enabled item", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true });

    const trigger = screen.getByRole("button", { name: "New" });
    trigger.focus();
    await userEvent.keyboard("{ArrowUp}");

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const items = screen.getAllByRole("menuitem");
    expect(items[items.length - 1]).toHaveFocus();
  });

  it("focuses the first item via ArrowDown on the trigger when the menu is already open", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const items = screen.getAllByRole("menuitem");

    // Return focus to trigger while menu stays open, then press ArrowDown.
    screen.getByRole("button", { name: "New" }).focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(items[0]).toHaveFocus();
  });

  it("focuses the last item via ArrowUp on the trigger when the menu is already open", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const items = screen.getAllByRole("menuitem");

    screen.getByRole("button", { name: "New" }).focus();
    await userEvent.keyboard("{ArrowUp}");
    expect(items[items.length - 1]).toHaveFocus();
  });

  it("applies roving tabindex: focused item has tabIndex=0, others and disabled items have tabIndex=-1", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: false });

    await userEvent.click(screen.getByRole("button", { name: "New" }));

    const newChatItem = screen.getByRole("menuitem", { name: "New chat" });
    const newImageItem = screen.getByRole("menuitem", { name: "New image" });

    // New chat is the focused (active) item; New image is disabled — always tabIndex=-1.
    expect(newChatItem).toHaveAttribute("tabindex", "0");
    expect(newImageItem).toHaveAttribute("tabindex", "-1");
  });

  it("wraps focus at the start and end of the menu", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const items = screen.getAllByRole("menuitem");

    await userEvent.keyboard("{ArrowUp}");
    expect(items[items.length - 1]).toHaveFocus();

    await userEvent.keyboard("{ArrowDown}");
    expect(items[0]).toHaveFocus();
  });

  it("does not render New image item when isImageGenerationEnabled is false", async () => {
    renderNewMenu({ isImageGenerationEnabled: false });
    await userEvent.click(screen.getByRole("button", { name: "New" }));
    expect(screen.queryByRole("menuitem", { name: "New image" })).not.toBeInTheDocument();
  });

  it("renders New image item as enabled when isImageGenerationEnabled and isImageGenerationAvailable are true", async () => {
    const onNewImage = vi.fn();
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: true, onNewImage });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const item = screen.getByRole("menuitem", { name: "New image" });
    expect(item).not.toHaveAttribute("aria-disabled");

    await userEvent.click(item);
    expect(onNewImage).toHaveBeenCalledTimes(1);
  });

  it("renders New image item as disabled when isImageGenerationEnabled is true but isImageGenerationAvailable is false", async () => {
    const onNewImage = vi.fn();
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: false, onNewImage });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const item = screen.getByRole("menuitem", { name: "New image" });
    expect(item).toHaveAttribute("aria-disabled", "true");

    await userEvent.click(item);
    expect(onNewImage).not.toHaveBeenCalled();
  });

  it("skips disabled items in arrow-key navigation", async () => {
    renderNewMenu({ isImageGenerationEnabled: true, isImageGenerationAvailable: false });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    const newChatItem = screen.getByRole("menuitem", { name: "New chat" });
    expect(newChatItem).toHaveFocus();

    // ArrowDown should wrap because the only enabled item is New chat
    await userEvent.keyboard("{ArrowDown}");
    expect(newChatItem).toHaveFocus();
  });
});
