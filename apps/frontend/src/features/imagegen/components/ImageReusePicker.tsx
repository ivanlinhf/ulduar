import { useEffect, useRef, useState } from "react";

import type { ReusableImageSource } from "../types";

type ImageReusePickerProps = {
  busy: boolean;
  onReuseImage: (source: ReusableImageSource) => Promise<void>;
  reusingImageIds: string[];
  reusableImages: ReusableImageSource[];
};

export function ImageReusePicker({
  busy,
  onReuseImage,
  reusingImageIds,
  reusableImages,
}: ImageReusePickerProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());
  const uploadedImages = reusableImages.filter((image) => image.kind === "upload");
  const generatedImages = reusableImages.filter((image) => image.kind === "generated");

  useEffect(() => {
    const map = previewUrlMapRef.current;
    const uploadEntries = reusableImages.filter(
      (image): image is ReusableImageSource & { file: File } => image.kind === "upload" && !!image.file,
    );
    const nextIds = new Set(uploadEntries.map((image) => image.id));

    for (const [id, url] of [...map]) {
      if (!nextIds.has(id)) {
        URL.revokeObjectURL(url);
        map.delete(id);
      }
    }

    for (const image of uploadEntries) {
      if (!map.has(image.id)) {
        map.set(image.id, URL.createObjectURL(image.file));
      }
    }

    setPreviewUrls(new Map(map));
  }, [reusableImages]);

  useEffect(() => {
    const map = previewUrlMapRef.current;
    return () => {
      for (const url of map.values()) {
        URL.revokeObjectURL(url);
      }
      map.clear();
    };
  }, []);

  if (reusableImages.length === 0) {
    return null;
  }

  return (
    <section className="image-reuse-panel" aria-label="Reuse images from this session">
      <div className="image-reuse-header">
        <p className="image-reuse-title">Reuse from this session</p>
        <span className="image-reuse-description">
          Select earlier uploads or generated outputs to attach them to this draft.
        </span>
      </div>

      {uploadedImages.length > 0 ? (
        <ImageReuseGroup
          busy={busy}
          buttonLabelPrefix="Attach previous upload"
          onReuseImage={onReuseImage}
          previewUrls={previewUrls}
          reusingImageIds={reusingImageIds}
          sources={uploadedImages}
          title="Previous uploads"
        />
      ) : null}

      {generatedImages.length > 0 ? (
        <ImageReuseGroup
          busy={busy}
          buttonLabelPrefix="Attach generated image"
          onReuseImage={onReuseImage}
          previewUrls={previewUrls}
          reusingImageIds={reusingImageIds}
          sources={generatedImages}
          title="Generated outputs"
        />
      ) : null}
    </section>
  );
}

type ImageReuseGroupProps = {
  busy: boolean;
  buttonLabelPrefix: string;
  onReuseImage: (source: ReusableImageSource) => Promise<void>;
  previewUrls: Map<string, string>;
  reusingImageIds: string[];
  sources: ReusableImageSource[];
  title: string;
};

function ImageReuseGroup({
  busy,
  buttonLabelPrefix,
  onReuseImage,
  previewUrls,
  reusingImageIds,
  sources,
  title,
}: ImageReuseGroupProps) {
  return (
    <div className="image-reuse-group">
      <p className="image-reuse-group-title">{title}</p>
      <div className="image-reuse-list">
        {sources.map((source) => {
          const previewUrl = source.kind === "upload" ? previewUrls.get(source.id) : source.contentUrl;
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
              {previewUrl ? (
                <img
                  alt=""
                  aria-hidden="true"
                  className="image-reuse-item-thumb"
                  src={previewUrl}
                />
              ) : null}
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
