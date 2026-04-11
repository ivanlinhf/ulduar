import { useCallback, useEffect, useId, useRef, useState } from "react";

import { IconNewChat } from "./icons";

type NewMenuProps = {
  isImageGenerationEnabled: boolean;
  isImageGenerationAvailable?: boolean;
  onNewChat: () => void;
  onNewImage?: () => void;
};

export function NewMenu({
  isImageGenerationEnabled,
  isImageGenerationAvailable = true,
  onNewChat,
  onNewImage,
}: NewMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  function open() {
    setIsOpen(true);
  }

  function close() {
    setIsOpen(false);
  }

  const getMenuItems = useCallback((): HTMLElement[] => {
    if (!menuRef.current) return [];
    return Array.from(
      menuRef.current.querySelectorAll<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])'),
    );
  }, []);

  function handleTriggerClick() {
    if (isOpen) {
      close();
    } else {
      open();
    }
  }

  function handleTriggerKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowDown" || event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  }

  function handleMenuKeyDown(event: React.KeyboardEvent) {
    const items = getMenuItems();
    const focused = document.activeElement as HTMLElement;
    const index = items.indexOf(focused);

    if (event.key === "Escape") {
      event.preventDefault();
      close();
      triggerRef.current?.focus();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      items[(index + 1) % items.length]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      items[(index - 1 + items.length) % items.length]?.focus();
    } else if (event.key === "Home") {
      event.preventDefault();
      items[0]?.focus();
    } else if (event.key === "End") {
      event.preventDefault();
      items[items.length - 1]?.focus();
    } else if (event.key === "Tab") {
      close();
    }
  }

  // Focus first enabled menu item when the menu opens
  useEffect(() => {
    if (isOpen) {
      const items = getMenuItems();
      items[0]?.focus();
    }
  }, [isOpen, getMenuItems]);

  // Close when focus leaves the entire widget
  useEffect(() => {
    if (!isOpen) return;

    function handleFocusOut(event: FocusEvent) {
      const next = event.relatedTarget as Node | null;
      if (
        next &&
        (triggerRef.current?.contains(next) || menuRef.current?.contains(next))
      ) {
        return;
      }
      close();
    }

    const trigger = triggerRef.current;
    const menu = menuRef.current;
    trigger?.addEventListener("focusout", handleFocusOut);
    menu?.addEventListener("focusout", handleFocusOut);
    return () => {
      trigger?.removeEventListener("focusout", handleFocusOut);
      menu?.removeEventListener("focusout", handleFocusOut);
    };
  }, [isOpen]);

  // Close on pointer-down outside the widget
  useEffect(() => {
    if (!isOpen) return;

    function handlePointerDown(event: PointerEvent) {
      const target = event.target as Node;
      if (!triggerRef.current?.contains(target) && !menuRef.current?.contains(target)) {
        close();
      }
    }

    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [isOpen]);

  return (
    <div className="new-menu-anchor">
      <button
        ref={triggerRef}
        aria-controls={menuId}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        className="secondary-button new-menu-trigger"
        type="button"
        onClick={handleTriggerClick}
        onKeyDown={handleTriggerKeyDown}
      >
        <IconNewChat />
        <span className="new-menu-trigger-label">New</span>
      </button>

      <div
        ref={menuRef}
        id={menuId}
        role="menu"
        aria-label="New"
        aria-hidden={!isOpen}
        className="new-menu-panel"
        data-open={isOpen ? "true" : "false"}
        onKeyDown={handleMenuKeyDown}
      >
        <button
          role="menuitem"
          type="button"
          className="new-menu-item"
          tabIndex={isOpen ? 0 : -1}
          onClick={() => {
            onNewChat();
            close();
          }}
        >
          New chat
        </button>

        {isImageGenerationEnabled && (
          <button
            role="menuitem"
            type="button"
            className="new-menu-item"
            tabIndex={isOpen ? 0 : -1}
            aria-disabled={!isImageGenerationAvailable ? "true" : undefined}
            onClick={() => {
              if (!isImageGenerationAvailable) return;
              onNewImage?.();
              close();
            }}
          >
            New image
          </button>
        )}
      </div>
    </div>
  );
}
