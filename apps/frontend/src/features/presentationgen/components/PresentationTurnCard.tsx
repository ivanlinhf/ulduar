import { useState } from "react";

import { downloadPresentationGenerationAsset } from "../../../lib/api";
import { ActionTooltip } from "../../chat/components/ActionTooltip";
import { IconDownload, IconSpinner } from "../../chat/components/icons";
import { compactMediaType, formatPresentationOutputMediaType } from "../utils";
import type { PresentationTurn, PresentationTurnOutputAsset } from "../types";

// Keep the blob URL alive briefly so the browser can start the download before revoking it.
const downloadURLCleanupDelayMs = 1000;

type PresentationTurnCardProps = {
  turn: PresentationTurn;
};

export function PresentationTurnCard({ turn }: PresentationTurnCardProps) {
  return (
    <article className="image-turn-card presentation-turn-card">
      <div className="image-turn-input">
        {turn.inputAttachments.length > 0 ? (
          <ul className="presentation-turn-attachments" aria-label="Presentation references">
            {turn.inputAttachments.map((attachment) => (
              <li key={attachment.id} className="attachment-chip attachment-chip-file">
                <span
                  className="attachment-chip-name"
                  title={`${attachment.filename} · ${compactMediaType(attachment.mediaType)}`}
                >
                  {attachment.filename} · {compactMediaType(attachment.mediaType)}
                </span>
              </li>
            ))}
          </ul>
        ) : null}

        <div className="image-turn-prompt-row">
          <p className="image-turn-prompt">{turn.prompt}</p>
          <span className={`status-badge status-${turn.status}`}>{turn.status}</span>
        </div>
      </div>

      {(turn.status === "pending" || turn.status === "running") ? (
        <div className="image-turn-progress" role="status" aria-label="Generating presentation">
          <IconSpinner />
          <span>{turn.status === "pending" ? "Queued…" : "Generating…"}</span>
        </div>
      ) : null}

      {turn.status === "completed" && turn.outputAsset ? (
        <PresentationTurnOutputCard asset={turn.outputAsset} />
      ) : null}

      {turn.status === "failed" && turn.errorMessage ? (
        <p className="image-turn-error">{turn.errorMessage}</p>
      ) : null}
    </article>
  );
}

type PresentationTurnOutputCardProps = {
  asset: PresentationTurnOutputAsset;
};

function PresentationTurnOutputCard({ asset }: PresentationTurnOutputCardProps) {
  const [downloadState, setDownloadState] = useState<"idle" | "downloading" | "failed">("idle");

  async function handleDownload() {
    if (downloadState === "downloading") {
      return;
    }

    setDownloadState("downloading");

    try {
      const blob = await downloadPresentationGenerationAsset(
        asset.sessionId,
        asset.generationId,
        asset.assetId,
      );
      const downloadUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = downloadUrl;
      link.download = asset.filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => {
        URL.revokeObjectURL(downloadUrl);
      }, downloadURLCleanupDelayMs);
      setDownloadState("idle");
    } catch {
      setDownloadState("failed");
    }
  }

  return (
    <div className="presentation-turn-output-card">
      <div className="presentation-turn-output-file">
        <strong>{asset.filename}</strong>
        <span>{formatPresentationOutputMediaType(asset.mediaType)}</span>
      </div>
      <div
        className="image-turn-output-toolbar"
        role="toolbar"
        aria-label={`Presentation output actions for ${asset.filename}`}
      >
        <span className="image-turn-output-meta">Ready to download</span>
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
                {downloadState === "downloading" ? "Downloading PPTX" : "Download PPTX"}
              </span>
            }
          >
            <button
              type="button"
              className="message-toolbar-button"
              aria-label={
                downloadState === "downloading"
                  ? `Downloading generated presentation ${asset.filename}`
                  : `Download generated presentation ${asset.filename}`
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
    </div>
  );
}
