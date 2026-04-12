import { useEffect, useRef, useState } from "react";

import { IconSpinner } from "../../chat/components/icons";
import type { ImageTurn } from "../types";

type ImageTurnCardProps = {
  turn: ImageTurn;
};

export function ImageTurnCard({ turn }: ImageTurnCardProps) {
  const previewUrlMapRef = useRef<Map<string, string>>(new Map());
  const [previewUrls, setPreviewUrls] = useState<Map<string, string>>(new Map());

  useEffect(() => {
    const map = previewUrlMapRef.current;
    let added = false;
    for (const { id, file } of turn.referenceImages) {
      if (!map.has(id)) {
        map.set(id, URL.createObjectURL(file));
        added = true;
      }
    }
    if (added) {
      setPreviewUrls(new Map(map));
    }
  }, [turn.referenceImages]);

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
    <article className="image-turn-card">
      <div className="image-turn-input">
        {turn.referenceImages.length > 0 ? (
          <div className="image-turn-references">
            {turn.referenceImages.map(({ id, file }) => {
              const url = previewUrls.get(id);
              return url ? (
                <img
                  key={id}
                  className="image-turn-ref-thumb"
                  src={url}
                  alt={file.name}
                  title={file.name}
                />
              ) : null;
            })}
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
