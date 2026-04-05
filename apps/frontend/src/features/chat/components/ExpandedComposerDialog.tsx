import type { ChangeEvent, KeyboardEvent, MouseEvent, RefObject } from "react";

import type { SubmissionState } from "../types";
import { ActionTooltip } from "./ActionTooltip";
import { IconSend, IconSpinner } from "./icons";

type ExpandedComposerDialogProps = {
  busy: boolean;
  canSubmit: boolean;
  composerText: string;
  dialogRef: RefObject<HTMLElement | null>;
  expandedComposerRef: RefObject<HTMLTextAreaElement | null>;
  isOpen: boolean;
  onBackdropMouseDown: (event: MouseEvent<HTMLDivElement>) => void;
  onDialogKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
  onSendClick: () => Promise<void>;
  onTextChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onTextareaKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  submissionState: SubmissionState;
  submitButtonLabel: string;
};

export function ExpandedComposerDialog({
  busy,
  canSubmit,
  composerText,
  dialogRef,
  expandedComposerRef,
  isOpen,
  onBackdropMouseDown,
  onDialogKeyDown,
  onSendClick,
  onTextChange,
  onTextareaKeyDown,
  submissionState,
  submitButtonLabel,
}: ExpandedComposerDialogProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <div className="composer-dialog-backdrop" onMouseDown={onBackdropMouseDown}>
      <section
        className="composer-dialog"
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Expanded message editor"
        onKeyDown={onDialogKeyDown}
      >
        <textarea
          ref={expandedComposerRef}
          className="composer-dialog-input"
          aria-label="Expanded message"
          value={composerText}
          onChange={onTextChange}
          onKeyDown={onTextareaKeyDown}
          placeholder="Ask about a screenshot, summarize a PDF, or start a plain text chat."
          disabled={busy}
        />

        <div className="composer-dialog-footer">
          <div className="composer-dialog-actions">
            <span className="composer-hint">Shift + Enter to send</span>
            <ActionTooltip
              align="right"
              side="above"
              content={<span className="action-tooltip-label">{submitButtonLabel}</span>}
            >
              <button
                aria-label={submitButtonLabel}
                className="primary-button icon-only-button send-button"
                type="button"
                onClick={() => void onSendClick()}
                disabled={!canSubmit}
              >
                {submissionState === "idle" ? <IconSend /> : <IconSpinner />}
              </button>
            </ActionTooltip>
          </div>
        </div>
      </section>
    </div>
  );
}
