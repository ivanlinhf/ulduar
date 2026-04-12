import { useEffect, useRef } from "react";

import type { ImageTurn } from "../types";
import { ImageTurnCard } from "./ImageTurnCard";

type ImageTimelineProps = {
  turns: ImageTurn[];
};

export function ImageTimeline({ turns }: ImageTimelineProps) {
  const endRef = useRef<HTMLDivElement | null>(null);
  const prevLengthRef = useRef(0);

  useEffect(() => {
    if (turns.length <= prevLengthRef.current) {
      prevLengthRef.current = turns.length;
      return;
    }
    prevLengthRef.current = turns.length;

    const prefersReducedMotion =
      typeof window !== "undefined" &&
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    endRef.current?.scrollIntoView({
      behavior: prefersReducedMotion ? "auto" : "smooth",
      block: "end",
    });
  }, [turns.length]);

  return (
    <div className={turns.length === 0 ? "image-timeline image-timeline-empty" : "image-timeline"}>
      {turns.length === 0 ? (
        <div className="empty-state">
          <p>No images yet.</p>
          <span>Enter a prompt below to generate your first image.</span>
        </div>
      ) : (
        turns.map((turn) => <ImageTurnCard key={turn.id} turn={turn} />)
      )}
      <div ref={endRef} />
    </div>
  );
}
