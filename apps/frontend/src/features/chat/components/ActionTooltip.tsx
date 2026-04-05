import {
  cloneElement,
  isValidElement,
  useId,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";

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
  const [isHovered, setIsHovered] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const isOpen = isHovered || isFocused;
  const tooltipId = useId();
  const wrapperClassNames = ["action-tooltip-anchor", `action-tooltip-${align}`, `action-tooltip-${side}`, wrapperClassName]
    .filter(Boolean)
    .join(" ");
  const tooltipClassNames = ["action-tooltip-panel", tooltipClassName].filter(Boolean).join(" ");
  const child =
    isValidElement(children) && typeof children.type !== "symbol"
      ? cloneElement(children as ReactElement<{ "aria-describedby"?: string }>, {
          "aria-describedby": [
            (children.props as { "aria-describedby"?: string })["aria-describedby"],
            isOpen ? tooltipId : undefined,
          ]
            .filter(Boolean)
            .join(" ") || undefined,
        })
      : children;

  return (
    <div
      className={wrapperClassNames}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onFocusCapture={() => setIsFocused(true)}
      onBlurCapture={(event) => {
        if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
          return;
        }

        setIsFocused(false);
      }}
    >
      {child}
      <div
        aria-hidden={!isOpen}
        className={tooltipClassNames}
        data-open={isOpen ? "true" : "false"}
        id={tooltipId}
        role="tooltip"
      >
        {content}
      </div>
    </div>
  );
}
