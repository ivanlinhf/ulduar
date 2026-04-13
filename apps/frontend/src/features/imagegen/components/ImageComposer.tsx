import { useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent, type RefObject, type SubmitEvent } from "react";

import type { ImageGenerationCapabilitiesResponse, ImageGenerationMode } from "../../../lib/api";
import { ActionTooltip } from "../../chat/components/ActionTooltip";
import { IconClose, IconSend, IconSpinner } from "../../chat/components/icons";
import { referenceImageInputAccept } from "../constants";
import type { ImageSubmissionState, ReusableImageSource, SelectedReferenceImage } from "../types";
import { ImageReusePicker } from "./ImageReusePicker";

type ImageComposerProps = {
  attachmentToast: string;
  busy: boolean;
  canSubmit: boolean;
  capabilities: ImageGenerationCapabilitiesResponse;
  composerRef: RefObject<HTMLTextAreaElement | null>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  onFileSelection: (event: ChangeEvent<HTMLInputElement>) => void;
  onOpenFilePicker: () => void;
  onPromptChange: (event: ChangeEvent<HTMLTextAreaElement>) => void;
  onPromptKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onRemoveReferenceImage: (id: string) => void;
  onResolutionChange: (event: ChangeEvent<HTMLSelectElement>) => void;
  onReuseImage: (source: ReusableImageSource) => Promise<void>;
  onSubmit: (event: SubmitEvent<HTMLFormElement>) => Promise<void>;
  mode: ImageGenerationMode;
  prompt: string;
  referenceImages: SelectedReferenceImage[];
  reusableImages: ReusableImageSource[];
  resolution: string;
  reusingImageIds: string[];
  screenError: string;
  submissionState: ImageSubmissionState;
  generateButtonLabel: string;
};

export function ImageComposer({
  attachmentToast,
  busy,
  canSubmit,
  capabilities,
  composerRef,
  fileInputRef,
  onFileSelection,
  onOpenFilePicker,
  onPromptChange,
  onPromptKeyDown,
  onRemoveReferenceImage,
  onResolutionChange,
  onReuseImage,
  onSubmit,
  mode,
  prompt,
  referenceImages,
  reusableImages,
  resolution,
  reusingImageIds,
  screenError,
  submissionState,
  generateButtonLabel,
}: ImageComposerProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());
  const isEditMode = mode === "image_edit";
  const modeLabel = isEditMode ? "Edit" : "Generate";
  const inputPlaceholder = isEditMode
    ? "Describe the result you want. Add reference images from file or this session to guide the next image. Shift + Enter to generate."
    : "Describe the result you want. Add reference images from file or this session when you want the next image to follow or edit them. Shift + Enter to generate.";

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

  return (
    <form className="composer image-composer" onSubmit={onSubmit}>
      <div className="composer-input-shell composer-input-shell-inline">
        <span className="image-composer-mode-badge">{modeLabel}</span>
        <textarea
          ref={composerRef}
          className="composer-input"
          aria-label="Image prompt"
          value={prompt}
          onChange={onPromptChange}
          onKeyDown={onPromptKeyDown}
          placeholder={inputPlaceholder}
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
          <ImageReusePicker
            busy={busy}
            onOpenFilePicker={onOpenFilePicker}
            onReuseImage={onReuseImage}
            reusingImageIds={reusingImageIds}
            reusableImages={reusableImages}
          />

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
            dismissOnPress
            openOnFocus={false}
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
        accept={referenceImageInputAccept}
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
