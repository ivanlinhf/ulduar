import { useEffect, useRef } from "react";

import { IconSpinner } from "../../chat/components/icons";
import type { ImageTurn } from "../types";

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
            <img
              key={img.assetId}
              className="image-turn-output"
              src={img.contentUrl}
              alt={img.filename}
              loading="lazy"
              decoding="async"
            />
          ))}
        </div>
      ) : null}

      {turn.status === "failed" && turn.errorMessage ? (
        <p className="image-turn-error">{turn.errorMessage}</p>
      ) : null}
    </article>
  );
}
