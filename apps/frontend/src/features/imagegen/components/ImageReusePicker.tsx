import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

import { IconAttachment, IconClose } from "../../chat/components/icons";
import type { ReusableImageSource } from "../types";

type ImageReusePickerProps = {
  busy: boolean;
  onOpenFilePicker: () => void;
  onReuseImage: (source: ReusableImageSource) => Promise<void>;
  reusingImageIds: string[];
  reusableImages: ReusableImageSource[];
};

export function ImageReusePicker({
  busy,
  onOpenFilePicker,
  onReuseImage,
  reusingImageIds,
  reusableImages,
}: ImageReusePickerProps) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const fileItemRef = useRef<HTMLButtonElement>(null);
  const sessionItemRef = useRef<HTMLButtonElement>(null);
  const menuId = useId();
  const panelId = useId();
  const generatedImages = reusableImages;

  useEffect(() => {
    if (!isOpen && !isMenuOpen) {
      return;
    }

    function handlePointerDown(event: PointerEvent) {
      const target = event.target as Node;
      if (
        triggerRef.current?.contains(target) ||
        menuRef.current?.contains(target) ||
        panelRef.current?.contains(target)
      ) {
        return;
      }

      setIsMenuOpen(false);
      setIsOpen(false);
    }

    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [isMenuOpen, isOpen]);

  useEffect(() => {
    if (reusableImages.length === 0) {
      setIsOpen(false);
    }
  }, [reusableImages.length]);

  useEffect(() => {
    if (!isMenuOpen) {
      return;
    }

    fileItemRef.current?.focus();
  }, [generatedImages.length, isMenuOpen]);

  const closeAll = useCallback(() => {
    setIsMenuOpen(false);
    setIsOpen(false);
  }, []);

  function openMenu() {
    setIsOpen(false);
    setIsMenuOpen(true);
  }

  function closeMenu() {
    setIsMenuOpen(false);
  }

  function handlePanelKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      closeAll();
      triggerRef.current?.focus();
    }
  }

  function handleMenuKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      closeAll();
      triggerRef.current?.focus();
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();

      if (document.activeElement === fileItemRef.current && generatedImages.length > 0) {
        sessionItemRef.current?.focus();
        return;
      }

      if (document.activeElement === sessionItemRef.current) {
        fileItemRef.current?.focus();
        return;
      }

      fileItemRef.current?.focus();
    }
  }

  return (
    <div className="image-reuse-anchor">
      <button
        ref={triggerRef}
        aria-controls={isOpen ? panelId : menuId}
        aria-expanded={isMenuOpen || isOpen}
        aria-haspopup={isOpen ? "dialog" : "menu"}
        aria-label="Add reference images"
        className="attachment-button icon-only-button"
        disabled={busy}
        type="button"
        onClick={() => {
          if (isOpen || isMenuOpen) {
            closeAll();
            return;
          }

          openMenu();
        }}
      >
        <IconAttachment />
      </button>

      <div
        ref={menuRef}
        id={menuId}
        role="menu"
        aria-label="Reference image sources"
        aria-hidden={!isMenuOpen}
        className="image-reference-menu"
        data-open={isMenuOpen ? "true" : "false"}
        onKeyDown={handleMenuKeyDown}
      >
        <button
          ref={fileItemRef}
          role="menuitem"
          type="button"
          className="image-reference-menu-item"
          onClick={() => {
            closeMenu();
            onOpenFilePicker();
            triggerRef.current?.focus();
          }}
        >
          From File
        </button>

        <button
          ref={sessionItemRef}
          role="menuitem"
          type="button"
          className="image-reference-menu-item"
          disabled={generatedImages.length === 0}
          aria-disabled={generatedImages.length === 0 ? "true" : undefined}
          onClick={() => {
            if (generatedImages.length === 0) {
              return;
            }

            setIsMenuOpen(false);
            setIsOpen(true);
          }}
        >
          From Session
        </button>
      </div>

      <div
        ref={panelRef}
        id={panelId}
        role="dialog"
        aria-label="Reference images from this session"
        aria-hidden={!isOpen}
        className="image-reuse-panel"
        data-open={isOpen ? "true" : "false"}
        onKeyDown={handlePanelKeyDown}
      >
        <div className="image-reuse-header">
          <div className="image-reuse-header-copy">
            <p className="image-reuse-title">From session</p>
            <span className="image-reuse-description">
              Select a generated image from this session to attach it as a reference for the next result.
            </span>
          </div>

          <button
            aria-label="Close session image picker"
            className="image-reuse-close"
            type="button"
            onClick={() => {
              setIsOpen(false);
              triggerRef.current?.focus();
            }}
          >
            <IconClose />
          </button>
        </div>

        {generatedImages.length > 0 ? (
          <ImageReuseGroup
            busy={busy}
            buttonLabelPrefix="Attach generated image"
            onReuseImage={onReuseImage}
            reusingImageIds={reusingImageIds}
            sources={generatedImages}
            title="Generated images"
          />
        ) : (
          <p className="image-reuse-empty">No generated images in this session yet.</p>
        )}
      </div>
    </div>
  );
}

type ImageReuseGroupProps = {
  busy: boolean;
  buttonLabelPrefix: string;
  onReuseImage: (source: ReusableImageSource) => Promise<void>;
  reusingImageIds: string[];
  sources: ReusableImageSource[];
  title: string;
};

function ImageReuseGroup({
  busy,
  buttonLabelPrefix,
  onReuseImage,
  reusingImageIds,
  sources,
  title,
}: ImageReuseGroupProps) {
  return (
    <div className="image-reuse-group">
      <p className="image-reuse-group-title">{title}</p>
      <div className="image-reuse-list">
        {sources.map((source) => {
          const isReusing = reusingImageIds.includes(source.id);

          return (
            <button
              key={source.id}
              aria-label={`${buttonLabelPrefix} ${source.name}`}
              className="image-reuse-item"
              disabled={busy || isReusing}
              onClick={() => void onReuseImage(source)}
              type="button"
            >
              <img
                alt=""
                aria-hidden="true"
                className="image-reuse-item-thumb"
                src={source.contentUrl}
              />
              <span className="image-reuse-item-name" title={source.name}>
                {source.name}
              </span>
              <span className="image-reuse-item-action">{isReusing ? "Adding…" : "Attach"}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
