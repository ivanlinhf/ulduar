import { useEffect, useRef, useState } from "react";

import { ActionTooltip } from "../../chat/components/ActionTooltip";
import { IconDownload, IconSpinner } from "../../chat/components/icons";
import type { ImageTurn, ImageTurnOutputImage } from "../types";

type ImageTurnCardProps = {
  turn: ImageTurn;
};

export function ImageTurnCard({ turn }: ImageTurnCardProps) {
  // Track which previewUrls have already been revoked by onLoad/onError so the
  // unmount cleanup below doesn't double-revoke them.
  const revokedRef = useRef(new Set<string>());

  useEffect(() => {
    const revoked = revokedRef.current;
    const urls = turn.referenceImages.map((r) => r.previewUrl);
    return () => {
      for (const url of urls) {
        if (!revoked.has(url)) {
          URL.revokeObjectURL(url);
        }
      }
    };
    // referenceImages are fixed at turn creation; capture once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function revokeOnce(url: string) {
    if (!revokedRef.current.has(url)) {
      revokedRef.current.add(url);
      URL.revokeObjectURL(url);
    }
  }

  return (
    <article className="image-turn-card">
      <div className="image-turn-input">
        {turn.referenceImages.length > 0 ? (
          <div className="image-turn-references">
            {turn.referenceImages.map(({ id, previewUrl, name }) => (
              <img
                key={id}
                className="image-turn-ref-thumb"
                src={previewUrl}
                alt={name}
                title={name}
                onLoad={() => revokeOnce(previewUrl)}
                onError={() => revokeOnce(previewUrl)}
              />
            ))}
          </div>
        ) : null}

        <div className="image-turn-prompt-row">
          <p className="image-turn-prompt">{turn.prompt}</p>
          <span className={`status-badge status-${turn.status}`}>{turn.status}</span>
        </div>
      </div>

      {(turn.status === "pending" || turn.status === "running") ? (
        <div className="image-turn-progress" role="status" aria-label="Generating image">
          <IconSpinner />
          <span>{turn.status === "pending" ? "Queued…" : "Generating…"}</span>
        </div>
      ) : null}

      {turn.status === "completed" && turn.outputImages.length > 0 ? (
        <div className="image-turn-outputs">
          {turn.outputImages.map((img) => (
            <ImageTurnOutputCard key={img.assetId} image={img} />
          ))}
        </div>
      ) : null}

      {turn.status === "failed" && turn.errorMessage ? (
        <p className="image-turn-error">{turn.errorMessage}</p>
      ) : null}
    </article>
  );
}

type ImageTurnOutputCardProps = {
  image: ImageTurnOutputImage;
};

function ImageTurnOutputCard({ image }: ImageTurnOutputCardProps) {
  const [downloadState, setDownloadState] = useState<"idle" | "downloading" | "failed">("idle");
  const resolutionLabel = formatResolutionLabel(image);

  async function handleDownload() {
    if (downloadState === "downloading") {
      return;
    }

    setDownloadState("downloading");

    try {
      const response = await fetch(image.contentUrl);
      if (!response.ok) {
        throw new Error(`download failed with status ${response.status}`);
      }

      const blob = await response.blob();
      const downloadUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = downloadUrl;
      link.download = image.filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => {
        URL.revokeObjectURL(downloadUrl);
      }, 1000);
      setDownloadState("idle");
    } catch {
      setDownloadState("failed");
    }
  }

  return (
    <figure className="image-turn-output-card">
      <div className="image-turn-output-frame">
        <img
          className="image-turn-output"
          src={image.contentUrl}
          alt={image.filename}
          loading="lazy"
          decoding="async"
        />
      </div>
      <div
        className="image-turn-output-toolbar"
        role="toolbar"
        aria-label={`Generated image actions for ${image.filename}`}
      >
        <span className="image-turn-output-meta">{resolutionLabel}</span>
        <div className="image-turn-output-actions">
          {downloadState === "failed" ? (
            <span className="image-turn-output-feedback" role="status">
              Download failed
            </span>
          ) : null}
          <ActionTooltip
            tooltipClassName="message-action-tooltip"
            content={
              <span className="action-tooltip-label message-action-tooltip-label">
                {downloadState === "downloading" ? "Downloading original" : "Download original"}
              </span>
            }
          >
            <button
              type="button"
              className="message-toolbar-button"
              aria-label={
                downloadState === "downloading"
                  ? `Downloading original generated image ${image.filename}`
                  : `Download original generated image ${image.filename}`
              }
              onClick={() => {
                void handleDownload();
              }}
              disabled={downloadState === "downloading"}
            >
              {downloadState === "downloading" ? <IconSpinner /> : <IconDownload />}
            </button>
          </ActionTooltip>
        </div>
      </div>
    </figure>
  );
}

function formatResolutionLabel(image: ImageTurnOutputImage): string {
  if (image.width && image.height) {
    return `Original ${image.width} x ${image.height}`;
  }

  return image.filename;
}
