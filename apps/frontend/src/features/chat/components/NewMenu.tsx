import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

import { IconNewChat } from "./icons";

type MenuItemKey = "new-chat" | "new-image";

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
  const newChatRef = useRef<HTMLButtonElement>(null);
  const newImageRef = useRef<HTMLButtonElement>(null);
  const menuId = useId();
  // Tracks which item holds tabIndex=0 in the roving tabindex scheme.
  const [activeKey, setActiveKey] = useState<MenuItemKey | null>(null);
  const focusLastOnOpenRef = useRef(false);
  const canCreateImage = isImageGenerationEnabled && isImageGenerationAvailable && typeof onNewImage === "function";

  // Returns the ordered list of enabled item keys (disabled items excluded).
  const getEnabledKeys = useCallback((): MenuItemKey[] => {
    const keys: MenuItemKey[] = ["new-chat"];
    if (canCreateImage) {
      keys.push("new-image");
    }
    return keys;
  }, [canCreateImage]);

  function focusItem(key: MenuItemKey) {
    setActiveKey(key);
    if (key === "new-chat") newChatRef.current?.focus();
    else if (key === "new-image") newImageRef.current?.focus();
  }

  function open() {
    setIsOpen(true);
  }

  function close() {
    setIsOpen(false);
    setActiveKey(null);
  }

  function handleTriggerClick() {
    if (isOpen) {
      close();
    } else {
      open();
    }
  }

  function handleTriggerKeyDown(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (isOpen) {
        close();
      } else {
        open();
      }
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      if (isOpen) {
        // Menu already open: move focus to the first enabled item.
        focusItem(getEnabledKeys()[0]);
      } else {
        open();
      }
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      if (isOpen) {
        // Menu already open: move focus to the last enabled item.
        const keys = getEnabledKeys();
        focusItem(keys[keys.length - 1]);
      } else {
        focusLastOnOpenRef.current = true;
        open();
      }
    }
  }

  function handleMenuKeyDown(event: KeyboardEvent) {
    const keys = getEnabledKeys();
    const index = activeKey !== null ? keys.indexOf(activeKey) : 0;

    if (event.key === "Escape") {
      event.preventDefault();
      close();
      triggerRef.current?.focus();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      focusItem(keys[(index + 1) % keys.length]);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      focusItem(keys[(index - 1 + keys.length) % keys.length]);
    } else if (event.key === "Home") {
      event.preventDefault();
      focusItem(keys[0]);
    } else if (event.key === "End") {
      event.preventDefault();
      focusItem(keys[keys.length - 1]);
    } else if (event.key === "Tab") {
      close();
    }
  }

  // Focus first (or last) enabled menu item when the menu opens.
  useEffect(() => {
    if (!isOpen) return;
    const keys = getEnabledKeys();
    const keyToActivate = focusLastOnOpenRef.current ? keys[keys.length - 1] : keys[0];
    focusLastOnOpenRef.current = false;
    focusItem(keyToActivate);
  }, [isOpen, getEnabledKeys]);

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
        // If focus is currently inside the menu panel, return it to the trigger
        // before closing so it is never left on an aria-hidden element.
        if (menuRef.current?.contains(document.activeElement)) {
          triggerRef.current?.focus();
        }
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
        aria-label="New"
        aria-expanded={isOpen}
        aria-haspopup="menu"
        className="secondary-button icon-only-button new-menu-trigger"
        type="button"
        onClick={handleTriggerClick}
        onKeyDown={handleTriggerKeyDown}
      >
        <IconNewChat />
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
          ref={newChatRef}
          role="menuitem"
          type="button"
          className="new-menu-item"
          tabIndex={activeKey === "new-chat" ? 0 : -1}
          onClick={() => {
            triggerRef.current?.focus();
            onNewChat();
            close();
          }}
        >
          New Chat
        </button>

        {isImageGenerationEnabled && (
          <button
            ref={newImageRef}
            role="menuitem"
            type="button"
            className="new-menu-item"
            tabIndex={canCreateImage && activeKey === "new-image" ? 0 : -1}
            disabled={!canCreateImage}
            aria-disabled={!canCreateImage ? "true" : undefined}
            onClick={() => {
              triggerRef.current?.focus();
              onNewImage?.();
              close();
            }}
          >
            New Image
          </button>
        )}
      </div>
    </div>
  );
}
