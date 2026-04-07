import type { ChangeEvent, KeyboardEvent, MouseEvent, RefObject } from "react";

import { composerPlaceholder } from "../constants";
import type { SelectedAttachment, SubmissionState } from "../types";
import { ComposerFooter } from "./ComposerFooter";

type ExpandedComposerDialogProps = {
  busy: boolean;
  canSubmit: boolean;
  composerText: string;
  dialogRef: RefObject<HTMLElement | null>;
  expandedComposerRef: RefObject<HTMLTextAreaElement | null>;
  isOpen: boolean;
  onBackdropMouseDown: (event: MouseEvent<HTMLDivElement>) => void;
  onDialogKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
  onOpenFilePicker: () => void;
  onRemoveAttachment: (id: string) => void;
  onSendClick: () => Promise<void>;
  onTextChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onTextareaKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  selectedFiles: SelectedAttachment[];
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
  onOpenFilePicker,
  onRemoveAttachment,
  onSendClick,
  onTextChange,
  onTextareaKeyDown,
  selectedFiles,
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
        <div className="composer-input-shell composer-input-shell-dialog">
          <textarea
            ref={expandedComposerRef}
            className="composer-dialog-input"
            aria-label="Expanded message"
            value={composerText}
            onChange={onTextChange}
            onKeyDown={onTextareaKeyDown}
            placeholder={composerPlaceholder}
            disabled={busy}
          />

          <ComposerFooter
            busy={busy}
            canSubmit={canSubmit}
            onOpenFilePicker={onOpenFilePicker}
            onRemoveAttachment={onRemoveAttachment}
            onSendClick={() => void onSendClick()}
            selectedFiles={selectedFiles}
            sendButtonType="button"
            submissionState={submissionState}
            submitButtonLabel={submitButtonLabel}
          />
        </div>
      </section>
    </div>
  );
}
