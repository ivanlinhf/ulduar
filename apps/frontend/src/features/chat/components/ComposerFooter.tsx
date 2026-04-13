import { useEffect, useRef, useState, type MouseEventHandler } from "react";

import type { SelectedAttachment, SubmissionState } from "../types";
import { ActionTooltip } from "./ActionTooltip";
import { IconAttachment, IconClose, IconSend, IconSpinner } from "./icons";

type ComposerFooterProps = {
  busy: boolean;
  canSubmit: boolean;
  onOpenFilePicker: () => void;
  onRemoveAttachment: (id: string) => void;
  onSendClick?: MouseEventHandler<HTMLButtonElement>;
  selectedFiles: SelectedAttachment[];
  sendButtonType: "button" | "submit";
  submissionState: SubmissionState;
  submitButtonLabel: string;
};

export function ComposerFooter({
  busy,
  canSubmit,
  onOpenFilePicker,
  onRemoveAttachment,
  onSendClick,
  selectedFiles,
  sendButtonType,
  submissionState,
  submitButtonLabel,
}: ComposerFooterProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());

  // Incrementally update blob URLs: create only for new ids, revoke only for removed ids.
  useEffect(() => {
    const map = previewUrlMapRef.current;
    const nextIds = new Set(selectedFiles.map((attachment) => attachment.id));

    for (const [id, url] of [...map]) {
      if (!nextIds.has(id)) {
        URL.revokeObjectURL(url);
        map.delete(id);
      }
    }

    for (const { id, file } of selectedFiles) {
      if (!map.has(id) && file.type.startsWith("image/")) {
        map.set(id, URL.createObjectURL(file));
      }
    }

    setPreviewUrls(new Map(map));
  }, [selectedFiles]);

  // Revoke all remaining blob URLs on unmount.
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
    <div className="composer-footer">
      <div className="composer-footer-start">
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

        <div className="composer-attachment-list">
          {selectedFiles.map(({ id, file }) => {
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
                <span className="attachment-chip-name" title={file.name}>
                  {file.name}
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
          content={<span className="action-tooltip-label">{submitButtonLabel}</span>}
        >
          <button
            aria-label={submitButtonLabel}
            className="primary-button icon-only-button send-button"
            disabled={!canSubmit}
            onClick={onSendClick}
            type={sendButtonType}
          >
            {submissionState === "idle" ? <IconSend /> : <IconSpinner />}
          </button>
        </ActionTooltip>
      </div>
    </div>
  );
}
