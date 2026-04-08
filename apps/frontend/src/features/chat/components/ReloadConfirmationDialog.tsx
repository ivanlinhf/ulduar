import { useCallback, useEffect, useRef, type KeyboardEvent } from "react";

import { reloadLosesSessionMessage } from "../../../lib/frontendUpdate";
import { getFocusableElements } from "../utils";

type ReloadConfirmationDialogProps = {
  isOpen: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ReloadConfirmationDialog({ isOpen, onCancel, onConfirm }: ReloadConfirmationDialogProps) {
  const dialogRef = useRef<HTMLElement | null>(null);
  const cancelButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const focusTimeoutId = window.setTimeout(() => {
      cancelButtonRef.current?.focus();
    }, 0);

    return () => {
      window.clearTimeout(focusTimeoutId);
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [isOpen]);

  const handleDialogKeyDown = useCallback(
    (event: KeyboardEvent<HTMLElement>) => {
      if (event.key === "Escape" && !event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey) {
        event.preventDefault();
        onCancel();
        return;
      }

      if (event.key !== "Tab" || event.altKey || event.ctrlKey || event.metaKey) {
        return;
      }

      const focusableElements = getFocusableElements(dialogRef.current);
      if (focusableElements.length === 0) {
        event.preventDefault();
        return;
      }

      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];
      const activeElement = document.activeElement;

      if (!event.shiftKey && activeElement === lastElement) {
        event.preventDefault();
        firstElement.focus();
      }

      if (event.shiftKey && activeElement === firstElement) {
        event.preventDefault();
        lastElement.focus();
      }
    },
    [onCancel],
  );

  if (!isOpen) {
    return null;
  }

  return (
    <div className="confirmation-dialog-backdrop">
      <section
        className="confirmation-dialog"
        ref={dialogRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="reload-confirmation-title"
        aria-describedby="reload-confirmation-description"
        onKeyDown={handleDialogKeyDown}
      >
        <div className="confirmation-dialog-copy">
          <p className="confirmation-dialog-eyebrow">Update available</p>
          <h2 className="confirmation-dialog-title" id="reload-confirmation-title">
            Reload to update?
          </h2>
          <p className="confirmation-dialog-description" id="reload-confirmation-description">
            {reloadLosesSessionMessage}
          </p>
        </div>

        <div className="confirmation-dialog-actions">
          <button ref={cancelButtonRef} className="secondary-button" onClick={onCancel} type="button">
            Cancel
          </button>
          <button className="primary-button" onClick={onConfirm} type="button">
            Reload
          </button>
        </div>
      </section>
    </div>
  );
}
