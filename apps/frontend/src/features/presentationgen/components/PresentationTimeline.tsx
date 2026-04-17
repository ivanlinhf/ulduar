import { useEffect, useRef } from "react";

import type { PresentationTurn } from "../types";
import { PresentationTurnCard } from "./PresentationTurnCard";

type PresentationTimelineProps = {
  turns: PresentationTurn[];
};

export function PresentationTimeline({ turns }: PresentationTimelineProps) {
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
          <p>No presentations yet.</p>
          <span>Enter a prompt below to generate your first deck.</span>
        </div>
      ) : (
        turns.map((turn) => <PresentationTurnCard key={turn.id} turn={turn} />)
      )}
      <div ref={endRef} />
    </div>
  );
}
