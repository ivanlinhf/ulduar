import {
  cloneElement,
  isValidElement,
  useEffect,
  useId,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";

type ActionTooltipProps = {
  align?: "left" | "right";
  children: ReactNode;
  content: ReactNode;
  dismissOnPress?: boolean;
  openOnFocus?: boolean;
  side?: "above" | "below";
  tooltipClassName?: string;
  wrapperClassName?: string;
};

export function ActionTooltip({
  align = "left",
  children,
  content,
  dismissOnPress = false,
  openOnFocus = true,
  side = "below",
  tooltipClassName = "",
  wrapperClassName = "",
}: ActionTooltipProps) {
  const [isHovered, setIsHovered] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const [isDismissed, setIsDismissed] = useState(false);
  const isOpen = !isDismissed && (isHovered || isFocused);
  const tooltipId = useId();

  useEffect(() => {
    if (!dismissOnPress || !isDismissed) {
      return;
    }

    function handleWindowFocus() {
      setIsHovered(false);
      setIsFocused(false);
      setIsDismissed(false);
    }

    window.addEventListener("focus", handleWindowFocus);
    return () => window.removeEventListener("focus", handleWindowFocus);
  }, [dismissOnPress, isDismissed]);

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
      onClickCapture={() => {
        if (dismissOnPress) {
          setIsDismissed(true);
        }
      }}
      onMouseEnter={() => {
        setIsHovered(true);
      }}
      onMouseLeave={() => {
        setIsHovered(false);
        setIsDismissed(false);
      }}
      onPointerDownCapture={() => {
        if (dismissOnPress) {
          setIsDismissed(true);
        }
      }}
      onFocusCapture={(event) => {
        if (openOnFocus && event.target instanceof Element && event.target.matches(":focus-visible")) {
          setIsFocused(true);
        }
      }}
      onBlurCapture={(event) => {
        if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
          return;
        }

        setIsFocused(false);
        setIsDismissed(false);
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
