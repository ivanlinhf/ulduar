import { useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent, type RefObject, type SubmitEvent } from "react";

import { ActionTooltip } from "../../chat/components/ActionTooltip";
import { IconAttachment, IconClose, IconSend, IconSpinner } from "../../chat/components/icons";
import { compactMediaType } from "../utils";
import type { PresentationSubmissionState, SelectedPresentationAttachment } from "../types";

type PresentationComposerProps = {
  attachmentToast: string;
  attachments: SelectedPresentationAttachment[];
  busy: boolean;
  canSubmit: boolean;
  composerRef: RefObject<HTMLTextAreaElement | null>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  generateButtonLabel: string;
  inputAccept: string;
  onFileSelection: (event: ChangeEvent<HTMLInputElement>) => void;
  onOpenFilePicker: () => void;
  onPromptChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onPromptKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onRemoveAttachment: (id: string) => void;
  onSubmit: (event: SubmitEvent<HTMLFormElement>) => Promise<void>;
  prompt: string;
  screenError: string;
  submissionState: PresentationSubmissionState;
};

export function PresentationComposer({
  attachmentToast,
  attachments,
  busy,
  canSubmit,
  composerRef,
  fileInputRef,
  generateButtonLabel,
  inputAccept,
  onFileSelection,
  onOpenFilePicker,
  onPromptChange,
  onPromptKeyDown,
  onRemoveAttachment,
  onSubmit,
  prompt,
  screenError,
  submissionState,
}: PresentationComposerProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());

  useEffect(() => {
    const map = previewUrlMapRef.current;
    const nextImageIds = new Set(
      attachments.filter((attachment) => attachment.file.type.startsWith("image/")).map((attachment) => attachment.id),
    );

    for (const [id, url] of [...map]) {
      if (!nextImageIds.has(id)) {
        URL.revokeObjectURL(url);
        map.delete(id);
      }
    }

    for (const { id, file } of attachments) {
      if (file.type.startsWith("image/") && !map.has(id)) {
        map.set(id, URL.createObjectURL(file));
      }
    }

    setPreviewUrls(new Map(map));
  }, [attachments]);

  useEffect(() => {
    const map = previewUrlMapRef.current;
    return () => {
      for (const url of map.values()) {
        URL.revokeObjectURL(url);
      }
      map.clear();
    };
  }, []);

  return (
    <form className="composer image-composer presentation-composer" onSubmit={onSubmit}>
      <div className="composer-input-shell composer-input-shell-inline">
        <span className="image-composer-mode-badge">Presentation</span>
        <textarea
          ref={composerRef}
          className="composer-input"
          aria-label="Presentation prompt"
          value={prompt}
          onChange={onPromptChange}
          onKeyDown={onPromptKeyDown}
          placeholder="Describe the presentation you want. Add optional image or PDF references when helpful. Shift + Enter to generate."
          rows={5}
          disabled={busy}
        />
      </div>

      <div className="image-composer-controls presentation-composer-controls">
        <div className="composer-footer-start">
          <ActionTooltip
            align="left"
            side="above"
            content={<span className="action-tooltip-label">Add attachments</span>}
          >
            <button
              type="button"
              className="attachment-button"
              aria-label="Add attachments"
              onClick={onOpenFilePicker}
              disabled={busy}
            >
              <IconAttachment />
            </button>
          </ActionTooltip>

          <div className="composer-attachment-list">
            {attachments.map(({ id, file }) => {
              const previewUrl = previewUrls.get(id);

              return previewUrl ? (
                <div key={id} className="attachment-chip">
                  <img
                    className="attachment-chip-thumb"
                    src={previewUrl}
                    alt={file.name}
                    title={file.name}
                  />
                  <button
                    className="attachment-remove-button"
                    aria-label={`Remove ${file.name}`}
                    onClick={() => onRemoveAttachment(id)}
                    type="button"
                    disabled={busy}
                  >
                    <IconClose />
                  </button>
                </div>
              ) : (
                <div key={id} className="attachment-chip attachment-chip-file">
                  <span className="attachment-chip-name" title={`${file.name} · ${compactMediaType(file.type)}`}>
                    {file.name} · {compactMediaType(file.type)}
                  </span>
                  <button
                    className="attachment-remove-button"
                    aria-label={`Remove ${file.name}`}
                    onClick={() => onRemoveAttachment(id)}
                    type="button"
                    disabled={busy}
                  >
                    <IconClose />
                  </button>
                </div>
              );
            })}
          </div>
        </div>

        <div className="composer-footer-end">
          <ActionTooltip
            align="right"
            side="above"
            dismissOnPress
            content={<span className="action-tooltip-label">{generateButtonLabel}</span>}
          >
            <button
              aria-label={generateButtonLabel}
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

      <input
        ref={fileInputRef}
        className="visually-hidden-file-input"
        type="file"
        accept={inputAccept}
        multiple
        onChange={onFileSelection}
        disabled={busy}
        tabIndex={-1}
      />

      {attachmentToast ? (
        <div aria-live="polite" aria-atomic="true">
          <div className="toast toast-warning">{attachmentToast}</div>
        </div>
      ) : null}
    </form>
  );
}
