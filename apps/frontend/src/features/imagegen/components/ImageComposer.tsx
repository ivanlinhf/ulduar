import { useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent, type MouseEventHandler, type SubmitEvent } from "react";

import type { ImageGenerationCapabilitiesResponse } from "../../../lib/api";
import { ActionTooltip } from "../../chat/components/ActionTooltip";
import { IconAttachment, IconClose, IconSend, IconSpinner } from "../../chat/components/icons";
import { imagePromptPlaceholder } from "../constants";
import type { ImageSubmissionState, SelectedReferenceImage } from "../types";

type ImageComposerProps = {
  busy: boolean;
  canSubmit: boolean;
  capabilities: ImageGenerationCapabilitiesResponse;
  composerRef: React.RefObject<HTMLTextAreaElement | null>;
  onOpenFilePicker: () => void;
  onPromptChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onPromptKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onRemoveReferenceImage: (id: string) => void;
  onResolutionChange: (event: ChangeEvent<HTMLSelectElement>) => void;
  onSubmit: (event: SubmitEvent<HTMLFormElement>) => Promise<void>;
  prompt: string;
  referenceImages: SelectedReferenceImage[];
  resolution: string;
  screenError: string;
  submissionState: ImageSubmissionState;
  generateButtonLabel: string;
};

export function ImageComposer({
  busy,
  canSubmit,
  capabilities,
  composerRef,
  onOpenFilePicker,
  onPromptChange,
  onPromptKeyDown,
  onRemoveReferenceImage,
  onResolutionChange,
  onSubmit,
  prompt,
  referenceImages,
  resolution,
  screenError,
  submissionState,
  generateButtonLabel,
}: ImageComposerProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());

  // Incrementally update blob URLs for reference image thumbnails.
  useEffect(() => {
    const map = previewUrlMapRef.current;
    const nextIds = new Set(referenceImages.map((r) => r.id));

    for (const [id, url] of [...map]) {
      if (!nextIds.has(id)) {
        URL.revokeObjectURL(url);
        map.delete(id);
      }
    }

    for (const { id, file } of referenceImages) {
      if (!map.has(id)) {
        map.set(id, URL.createObjectURL(file));
      }
    }

    setPreviewUrls(new Map(map));
  }, [referenceImages]);

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

  const handleAttachClick: MouseEventHandler<HTMLButtonElement> = () => {
    onOpenFilePicker();
  };

  return (
    <form className="composer image-composer" onSubmit={onSubmit}>
      <div className="composer-input-shell composer-input-shell-inline">
        <textarea
          ref={composerRef}
          className="composer-input"
          aria-label="Image prompt"
          value={prompt}
          onChange={onPromptChange}
          onKeyDown={onPromptKeyDown}
          placeholder={imagePromptPlaceholder}
          rows={5}
          disabled={busy}
        />
      </div>

      <div className="image-composer-controls">
        <div className="image-composer-controls-start">
          <label htmlFor="image-resolution" className="image-resolution-label">
            Size
          </label>
          <select
            id="image-resolution"
            className="image-resolution-select"
            value={resolution}
            onChange={onResolutionChange}
            disabled={busy}
          >
            {capabilities.resolutions.map((res) => (
              <option key={res.key} value={res.key}>
                {res.width} × {res.height}
              </option>
            ))}
          </select>
        </div>

        <div className="composer-footer-start">
          <ActionTooltip
            side="above"
            dismissOnPress
            openOnFocus={false}
            content={<span className="action-tooltip-label">Add reference images</span>}
          >
            <button
              aria-label="Add reference images"
              className="attachment-button icon-only-button"
              disabled={busy}
              onClick={handleAttachClick}
              type="button"
            >
              <IconAttachment />
            </button>
          </ActionTooltip>

          <div className="composer-attachment-list">
            {referenceImages.map(({ id, file }) => {
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
                    onClick={() => onRemoveReferenceImage(id)}
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
                    onClick={() => onRemoveReferenceImage(id)}
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
    </form>
  );
}
