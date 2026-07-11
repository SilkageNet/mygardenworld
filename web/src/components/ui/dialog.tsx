"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

const DialogContext = React.createContext<{ onOpenChange: (open: boolean) => void; titleId: string } | null>(null);

function Dialog({ open, onOpenChange, children }: DialogProps) {
  const overlayRef = React.useRef<HTMLDivElement>(null);
  const onOpenChangeRef = React.useRef(onOpenChange);
  const titleId = React.useId();

  React.useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  React.useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    const previousRootOverflow = document.documentElement.style.overflow;
    const previousScrollbarGutter = document.documentElement.style.scrollbarGutter;

    document.body.style.overflow = "hidden";
    document.documentElement.style.overflow = "hidden";
    document.documentElement.style.scrollbarGutter = "auto";

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onOpenChangeRef.current(false);
    }

    window.addEventListener("keydown", handleKeyDown);
    const focusFrame = window.requestAnimationFrame(() => {
      const focusTarget = overlayRef.current?.querySelector<HTMLElement>("[data-dialog-autofocus]");
      focusTarget?.focus({ preventScroll: true });
    });

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.cancelAnimationFrame(focusFrame);
      document.body.style.overflow = previousOverflow;
      document.documentElement.style.overflow = previousRootOverflow;
      document.documentElement.style.scrollbarGutter = previousScrollbarGutter;
      previouslyFocused?.focus({ preventScroll: true });
    };
  }, [open]);

  if (!open || typeof document === "undefined") return null;
  const useStableMobilePortal =
    document.documentElement.dataset.theme === "dark" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(max-width: 639px)").matches;
  const portalTop = useStableMobilePortal ? window.scrollY : 0;
  return createPortal(
    <DialogContext.Provider value={{ onOpenChange, titleId }}>
      <div
        ref={overlayRef}
        data-slot="dialog-root"
        className={useStableMobilePortal ? "contents" : "fixed inset-0 z-50 overscroll-none"}
      >
        {!useStableMobilePortal && (
          <div
            data-slot="dialog-overlay"
            className="absolute inset-0 z-0 bg-sky-950/52 dark:bg-black/80"
            onClick={() => onOpenChange(false)}
          />
        )}
        <div
          data-slot="dialog-positioner"
          className={useStableMobilePortal ? "absolute z-50 flex h-auto justify-center" : "absolute inset-0 z-10 flex items-end justify-center pt-[max(0.5rem,env(safe-area-inset-top))] pr-[max(0.75rem,env(safe-area-inset-right))] pb-[max(0.5rem,env(safe-area-inset-bottom))] pl-[max(0.75rem,env(safe-area-inset-left))] sm:items-center sm:p-4"}
          style={useStableMobilePortal ? {
            top: `calc(${portalTop}px + 5dvh)`,
            right: "max(0.75rem, env(safe-area-inset-right))",
            left: "max(0.75rem, env(safe-area-inset-left))",
          } : undefined}
        >
          {children}
        </div>
      </div>
    </DialogContext.Provider>,
    document.body,
  );
}

function DialogContent({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  const dialog = React.useContext(DialogContext);
  return (
    <div
      role="dialog"
      data-slot="dialog-content"
      aria-modal="true"
      aria-labelledby={dialog?.titleId}
      className={cn(
        "toy-shadow relative z-10 max-h-[90dvh] w-full max-w-md overflow-y-auto overscroll-contain rounded-xl border border-white/70 bg-card p-4 shadow-2xl ring-1 ring-border/60 outline-none sm:max-h-[calc(100dvh-2rem)] sm:rounded-lg sm:p-6 dark:border-white/10",
        className
      )}
      onClick={(e) => e.stopPropagation()}
      {...props}
    >
      <Button
        type="button"
        variant="ghost"
        size="icon"
        data-dialog-autofocus
        className="absolute right-2 top-2 z-10 size-10 text-muted-foreground hover:text-foreground sm:right-3 sm:top-3 sm:size-8 max-sm:dark:transition-none"
        onClick={() => dialog?.onOpenChange(false)}
        aria-label="关闭弹窗"
      >
        <X className="size-4" />
      </Button>
      {children}
    </div>
  );
}

function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("mb-4 space-y-1.5 pr-10", className)} {...props} />;
}

function DialogTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  const dialog = React.useContext(DialogContext);
  return <h2 id={dialog?.titleId} className={cn("text-lg font-semibold", className)} {...props} />;
}

function DialogDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-sm text-muted-foreground", className)} {...props} />;
}

function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("mt-6 flex flex-col-reverse gap-2 min-[380px]:flex-row min-[380px]:justify-end", className)} {...props} />;
}

export { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter };
