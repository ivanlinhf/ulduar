import type { ChangeEvent, KeyboardEvent, RefObject, SubmitEvent } from "react";

import { attachmentInputAccept } from "../constants";
import { compactMediaType, formatBytes } from "../utils";
import type { ChatAttachment, SubmissionState } from "../types";
import { ActionTooltip } from "./ActionTooltip";
import { IconAttachment, IconExpand, IconSend, IconSpinner } from "./icons";

type ChatComposerProps = {
  attachmentSummary: ChatAttachment[];
  busy: boolean;
  canSubmit: boolean;
  composerText: string;
  fileInputRef: RefObject<HTMLInputElement | null>;
  inlineComposerRef: RefObject<HTMLTextAreaElement | null>;
  onFileSelection: (event: ChangeEvent<HTMLInputElement>) => void;
  onOpenExpandedComposer: () => void;
  onOpenFilePicker: () => void;
  onRemoveAttachment: (filename: string) => void;
  onSubmit: (event: SubmitEvent<HTMLFormElement>) => Promise<void>;
  onTextChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onTextareaKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  screenError: string;
  submissionState: SubmissionState;
  submitButtonLabel: string;
};

export function ChatComposer({
  attachmentSummary,
  busy,
  canSubmit,
  composerText,
  fileInputRef,
  inlineComposerRef,
  onFileSelection,
  onOpenExpandedComposer,
  onOpenFilePicker,
  onRemoveAttachment,
  onSubmit,
  onTextChange,
  onTextareaKeyDown,
  screenError,
  submissionState,
  submitButtonLabel,
}: ChatComposerProps) {
  return (
    <form className="composer" onSubmit={onSubmit}>
      <div className="composer-input-shell">
        <textarea
          id="prompt"
          ref={inlineComposerRef}
          className="composer-input"
          aria-label="Message"
          value={composerText}
          onChange={onTextChange}
          onKeyDown={onTextareaKeyDown}
          placeholder="Ask about a screenshot, summarize a PDF, or start a plain text chat."
          rows={5}
          disabled={busy}
        />
        <ActionTooltip
          align="right"
          side="below"
          wrapperClassName="composer-expand-tooltip"
          content={<span className="action-tooltip-label">Expand editor</span>}
        >
          <button
            className="composer-expand-button"
            type="button"
            onClick={onOpenExpandedComposer}
            aria-label="Expand message editor"
            disabled={busy}
          >
            <IconExpand />
          </button>
        </ActionTooltip>
      </div>

      <div className="composer-toolbar">
        <ActionTooltip side="above" content={<span className="action-tooltip-label">Add attachments</span>}>
          <button
            aria-label="Add attachments"
            className="attachment-button icon-only-button"
            disabled={busy}
            onClick={onOpenFilePicker}
            type="button"
          >
            <IconAttachment />
          </button>
        </ActionTooltip>
        <input
          ref={fileInputRef}
          className="visually-hidden-file-input"
          type="file"
          accept={attachmentInputAccept}
          multiple
          onChange={onFileSelection}
          disabled={busy}
          tabIndex={-1}
        />

        <div className="composer-submit">
          <span className="composer-hint">Shift + Enter to send</span>
          <ActionTooltip
            align="right"
            side="above"
            content={<span className="action-tooltip-label">{submitButtonLabel}</span>}
          >
            <button
              aria-label={submitButtonLabel}
              className="primary-button icon-only-button send-button"
              disabled={!canSubmit}
              type="submit"
            >
              {submissionState === "idle" ? <IconSend /> : <IconSpinner />}
            </button>
          </ActionTooltip>
        </div>
      </div>

      {attachmentSummary.length > 0 ? (
        <ul className="composer-attachments">
          {attachmentSummary.map((attachment) => (
            <li key={attachment.filename}>
              <span>{attachment.filename}</span>
              <span>{compactMediaType(attachment.mediaType)}</span>
              <span>{formatBytes(attachment.sizeBytes)}</span>
              <button onClick={() => onRemoveAttachment(attachment.filename)} type="button">
                Remove
              </button>
            </li>
          ))}
        </ul>
      ) : null}

      {screenError ? <p className="screen-error">{screenError}</p> : null}
    </form>
  );
}
