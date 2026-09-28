import { forwardRef, type ButtonHTMLAttributes } from "react";
import { X } from "lucide-react";
import { Tooltip } from "./Tooltip";

interface ModalCloseButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  label: string;
  showTooltip?: boolean;
  showTooltipOnFocus?: boolean;
}

export const ModalCloseButton = forwardRef<HTMLButtonElement, ModalCloseButtonProps>(function ModalCloseButton({
  label,
  showTooltip = true,
  showTooltipOnFocus = true,
  className = "",
  type = "button",
  ...props
}, ref) {
  const ariaLabel = props["aria-label"] ?? label;
  const classes = `modal-close-button${className ? ` ${className}` : ""}`;

  return (
    <Tooltip label={label} disabled={!showTooltip} showOnFocus={showTooltipOnFocus}>
      <button {...props} ref={ref} type={type} className={classes} aria-label={ariaLabel}>
        <X size={15} strokeWidth={1.9} />
      </button>
    </Tooltip>
  );
});
