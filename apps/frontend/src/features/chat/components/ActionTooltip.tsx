import {
  cloneElement,
  isValidElement,
  useId,
  useState,
  type FocusEvent,
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
  const [isOpen, setIsOpen] = useState(false);
  const tooltipId = useId();
  const wrapperClassNames = ["action-tooltip-anchor", `action-tooltip-${align}`, `action-tooltip-${side}`, wrapperClassName]
    .filter(Boolean)
    .join(" ");
  const tooltipClassNames = ["action-tooltip-panel", tooltipClassName].filter(Boolean).join(" ");
  const child =
    isValidElement(children) && typeof children.type !== "symbol"
      ? cloneElement(children as ReactElement<{ "aria-describedby"?: string }>, {
          "aria-describedby": isOpen ? tooltipId : undefined,
        })
      : children;

  function handleBlur(event: FocusEvent<HTMLDivElement>) {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
      return;
    }

    setIsOpen(false);
  }

  return (
    <div
      className={wrapperClassNames}
      onMouseEnter={() => setIsOpen(true)}
      onMouseLeave={() => setIsOpen(false)}
      onFocusCapture={() => setIsOpen(true)}
      onBlurCapture={handleBlur}
    >
      {child}
      {isOpen ? (
        <div className={tooltipClassNames} id={tooltipId} role="tooltip">
          {content}
        </div>
      ) : null}
    </div>
  );
}
