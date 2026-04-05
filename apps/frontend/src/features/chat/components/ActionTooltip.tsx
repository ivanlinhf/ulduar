import type { ReactNode } from "react";

type ActionTooltipProps = {
  align?: "left" | "right";
  children: ReactNode;
  content: ReactNode;
  side?: "above" | "below";
  tooltipClassName?: string;
  wrapperClassName?: string;
};

export function ActionTooltip({
  align = "left",
  children,
  content,
  side = "below",
  tooltipClassName = "",
  wrapperClassName = "",
}: ActionTooltipProps) {
  const wrapperClassNames = ["action-tooltip-anchor", `action-tooltip-${align}`, `action-tooltip-${side}`, wrapperClassName]
    .filter(Boolean)
    .join(" ");
  const tooltipClassNames = ["action-tooltip-panel", tooltipClassName].filter(Boolean).join(" ");

  return (
    <div className={wrapperClassNames}>
      {children}
      <div className={tooltipClassNames}>{content}</div>
    </div>
  );
}
