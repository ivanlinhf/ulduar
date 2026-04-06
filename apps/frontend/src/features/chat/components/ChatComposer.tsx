import { useEffect, useState, type ChangeEvent, type KeyboardEvent, type RefObject, type SubmitEvent } from "react";

import { attachmentInputAccept } from "../constants";
import type { SubmissionState } from "../types";
import { ActionTooltip } from "./ActionTooltip";
import { IconAttachment, IconClose, IconExpand, IconSend, IconSpinner } from "./icons";

type ChatComposerProps = {
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
  selectedFiles: File[];
  submissionState: SubmissionState;
  submitButtonLabel: string;
};

export function ChatComposer({
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
  selectedFiles,
  submissionState,
  submitButtonLabel,
}: ChatComposerProps) {
  const [previewUrls, setPreviewUrls] = useState<Map<number, string>>(new Map());

  useEffect(() => {
    const urls = new Map<number, string>();
    selectedFiles.forEach((file, index) => {
      if (file.type.startsWith("image/")) {
        urls.set(index, URL.createObjectURL(file));
      }
    });
    setPreviewUrls(urls);

    return () => {
      urls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [selectedFiles]);

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
        <div className="composer-toolbar-start">
          <ActionTooltip
            side="above"
            dismissOnPress
            openOnFocus={false}
            content={<span className="action-tooltip-label">Add attachments</span>}
          >
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

          {selectedFiles.map((file, index) => {
            const previewUrl = previewUrls.get(index);
            return previewUrl ? (
              <div key={index} className="attachment-chip">
                <img
                  className="attachment-chip-thumb"
                  src={previewUrl}
                  alt={file.name}
                  title={file.name}
                />
                <button
                  className="attachment-remove-button"
                  aria-label={`Remove ${file.name}`}
                  onClick={() => onRemoveAttachment(file.name)}
                  type="button"
                  disabled={busy}
                >
                  <IconClose />
                </button>
              </div>
            ) : (
              <div key={index} className="attachment-chip attachment-chip-file">
                <span className="attachment-chip-name" title={file.name}>
                  {file.name}
                </span>
                <button
                  className="attachment-remove-button"
                  aria-label={`Remove ${file.name}`}
                  onClick={() => onRemoveAttachment(file.name)}
                  type="button"
                  disabled={busy}
                >
                  <IconClose />
                </button>
              </div>
            );
          })}
        </div>

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

      {screenError ? <p className="screen-error">{screenError}</p> : null}
    </form>
  );
}
