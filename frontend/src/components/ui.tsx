import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type MouseEvent as ReactMouseEvent,
} from "react";
import { createPortal } from "react-dom";
import {
  AlertTriangle,
  CheckCircle2,
  Info,
  Loader2,
  X,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import { cn } from "../lib/format";

// ------------------------------ Modal ------------------------------

export function Modal({
  open,
  onClose,
  title,
  subtitle,
  icon: Icon,
  size = "md",
  footer,
  children,
  closeOnBackdrop = true,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  icon?: LucideIcon;
  size?: "sm" | "md" | "lg" | "xl";
  footer?: ReactNode;
  children: ReactNode;
  closeOnBackdrop?: boolean;
}) {
  const modalRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const prev = document.activeElement as HTMLElement | null;
    const timer = window.setTimeout(() => {
      const target =
        modalRef.current?.querySelector<HTMLElement>("[data-autofocus]") ??
        modalRef.current?.querySelector<HTMLElement>(
          "input:not([type=hidden]), textarea, select, button",
        );
      target?.focus();
    }, 30);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key !== "Tab" || !modalRef.current) return;
      const focusables = Array.from(
        modalRef.current.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ),
      ).filter((el) => el.offsetParent !== null);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener("keydown", onKey);
      prev?.focus?.();
    };
  }, [open, onClose]);

  if (!open) return null;
  return createPortal(
    <div
      className="modal-backdrop"
      onMouseDown={(e) => {
        if (closeOnBackdrop && e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={modalRef}
        className={cn("modal", `modal-${size}`)}
        role="dialog"
        aria-modal="true"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="modal-head">
          {Icon && (
            <div className="ai-icon" style={{ width: 30, height: 30 }}>
              <Icon size={15} />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <div className="modal-title">{title}</div>
            {subtitle && <div className="modal-desc">{subtitle}</div>}
          </div>
          <button className="btn btn-icon btn-sm no-drag" onClick={onClose} title="关闭">
            <X size={14} />
          </button>
        </div>
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-foot">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

// ------------------------------ Dropdown ------------------------------

const MenuCloseContext = createContext<() => void>(() => {});

export function Dropdown({
  trigger,
  children,
  align = "right",
  className,
}: {
  trigger: (props: { onClick: (e: ReactMouseEvent) => void; open: boolean }) => ReactNode;
  children: ReactNode;
  align?: "left" | "right";
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className={cn("relative no-drag", className)}>
      {trigger({
        onClick: (e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        },
        open,
      })}
      {open && (
        <div
          className="menu"
          style={{
            top: "calc(100% + 7px)",
            ...(align === "right" ? { right: 0 } : { left: 0 }),
          }}
        >
          <MenuCloseContext.Provider value={() => setOpen(false)}>
            {children}
          </MenuCloseContext.Provider>
        </div>
      )}
    </div>
  );
}

export function MenuAction({
  icon: Icon,
  children,
  onClick,
  danger,
  disabled,
  shortcut,
  hint,
}: {
  icon?: LucideIcon;
  children: ReactNode;
  onClick?: () => void;
  danger?: boolean;
  disabled?: boolean;
  shortcut?: string;
  hint?: string;
}) {
  const close = useContext(MenuCloseContext);
  return (
    <button
      className={cn("menu-item", danger && "danger")}
      disabled={disabled}
      style={disabled ? { opacity: 0.45, cursor: "default" } : undefined}
      onClick={() => {
        if (disabled) return;
        onClick?.();
        close();
      }}
      title={hint}
    >
      {Icon && <Icon size={13} className="opacity-70" />}
      <span className="flex-1">{children}</span>
      {shortcut && <span className="kbd">{shortcut}</span>}
    </button>
  );
}

export function MenuDivider() {
  return <div className="menu-sep" />;
}

export function MenuLabel({ children }: { children: ReactNode }) {
  return <div className="menu-label">{children}</div>;
}

// ------------------------------ Toast ------------------------------

export type ToastKind = "info" | "success" | "error";
export interface ToastItem {
  id: number;
  kind: ToastKind;
  message: string;
}

export function ToastStack({
  items,
  onClose,
}: {
  items: ToastItem[];
  onClose: (id: number) => void;
}) {
  return createPortal(
    <div className="toast-stack">
      {items.map((t) => {
        const Icon = t.kind === "success" ? CheckCircle2 : t.kind === "error" ? XCircle : Info;
        return (
          <div key={t.id} className={cn("toast", t.kind)}>
            <Icon
              size={14}
              className={cn(
                "mt-0.5 flex-none",
                t.kind === "success" ? "text-ai-fp" : t.kind === "error" ? "text-sev-error" : "text-accent",
              )}
            />
            <div className="flex-1 whitespace-pre-wrap">{t.message}</div>
            <button
              className="btn btn-icon btn-sm flex-none"
              style={{ width: 20, height: 20 }}
              onClick={() => onClose(t.id)}
              title="关闭"
            >
              <X size={11} />
            </button>
          </div>
        );
      })}
    </div>,
    document.body,
  );
}

// ------------------------------ 其它小组件 ------------------------------

export function Toggle({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label?: ReactNode;
  disabled?: boolean;
}) {
  return (
    <label className="toggle" style={disabled ? { opacity: 0.5, pointerEvents: "none" } : undefined}>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="toggle-track" />
      {label && <span>{label}</span>}
    </label>
  );
}

export function Spinner({ size = 14, className }: { size?: number; className?: string }) {
  return <Loader2 size={size} className={cn("spin flex-none", className)} />;
}

export function Tooltip({
  children,
  tip,
  className,
}: {
  children: ReactNode;
  tip: string;
  className?: string;
}) {
  return (
    <span className={cn("tooltip inline-flex", className)} data-tip={tip}>
      {children}
    </span>
  );
}

export function ProgressBar({
  value,
  indeterminate,
}: {
  value: number;
  indeterminate?: boolean;
}) {
  return (
    <div className="progress-track">
      <div
        className={cn("progress-fill", indeterminate && "progress-indeterminate")}
        style={indeterminate ? undefined : { width: `${Math.max(0, Math.min(100, value * 100))}%` }}
      />
    </div>
  );
}

export function EmptyPane({
  icon: Icon,
  title,
  desc,
  children,
}: {
  icon: LucideIcon;
  title: string;
  desc?: string;
  children?: ReactNode;
}) {
  return (
    <div className="empty-pane">
      <div className="empty-icon">
        <Icon size={20} />
      </div>
      <div className="empty-title">{title}</div>
      {desc && <div className="empty-desc">{desc}</div>}
      {children}
    </div>
  );
}

export function WarningBox({
  children,
  error,
  icon: Icon = AlertTriangle,
}: {
  children: ReactNode;
  error?: boolean;
  icon?: LucideIcon;
}) {
  return (
    <div className={cn("warning-box", error && "error-box")}>
      <Icon size={14} className="mt-0.5 flex-none" />
      <div className="flex-1 min-w-0">{children}</div>
    </div>
  );
}
