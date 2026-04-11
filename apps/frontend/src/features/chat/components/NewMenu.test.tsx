import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { NewMenu } from "./NewMenu";

function renderNewMenu(props: Partial<React.ComponentProps<typeof NewMenu>> = {}) {
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

  it("calls onNewChat and closes menu when New chat item is clicked", async () => {
    const onNewChat = vi.fn();
    renderNewMenu({ onNewChat });

    await userEvent.click(screen.getByRole("button", { name: "New" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "New chat" }));

    expect(onNewChat).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "New" })).toHaveAttribute("aria-expanded", "false");
  });

  it("closes on Escape and returns focus to the trigger", async () => {
    renderNewMenu();
    const trigger = screen.getByRole("button", { name: "New" });
    await userEvent.click(trigger);
    await userEvent.keyboard("{Escape}");
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
