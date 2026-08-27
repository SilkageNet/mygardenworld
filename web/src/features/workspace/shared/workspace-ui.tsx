"use client";

import { useState, type ReactNode } from "react";
import { ChevronDown, Sparkles } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

export function CollapsibleCard({
  title,
  actions,
  children,
  className,
  contentClassName,
  defaultOpen = true,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <Card className={cn("cloud-surface bg-card/88", !open && "gap-0", className)}>
      <CardHeader className="border-b border-border/42 px-3 pb-3 sm:px-4">
        <div className="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
          <button
            type="button"
            className="flex min-w-0 items-center gap-2 text-left text-foreground transition-colors hover:text-primary active:scale-[0.99]"
            aria-expanded={open}
            onClick={() => setOpen((value) => !value)}
          >
            <ChevronDown className={cn("size-4 shrink-0 transition-transform", !open && "-rotate-90")} />
            <CardTitle className="truncate">{title}</CardTitle>
          </button>
          {actions && <div className="flex min-w-0 flex-wrap justify-end gap-1.5">{actions}</div>}
        </div>
      </CardHeader>
      {open && <CardContent className={cn("px-3 sm:px-4", contentClassName)}>{children}</CardContent>}
    </Card>
  );
}

export function OverviewStat({
  icon,
  label,
  value,
  detail,
  wrap = false,
  compact = false,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  wrap?: boolean;
  compact?: boolean;
}) {
  return (
    <div className="flex min-h-[72px] min-w-0 items-center gap-2 rounded-md border border-border/55 bg-white/52 px-2.5 py-2 shadow-sm transition-colors hover:bg-white/68 dark:bg-white/6 dark:hover:bg-white/9 sm:min-h-[76px] sm:gap-3 sm:px-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 shadow-sm dark:bg-white/8 dark:text-sky-300 sm:size-9 [&_svg]:size-4">{icon}</div>
      <div className="min-w-0 flex-1">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div
          className={cn(
            "font-semibold tabular-nums",
            compact ? "text-sm sm:text-base" : "text-base sm:text-lg",
            wrap ? "whitespace-normal break-all" : "truncate",
          )}
        >
          {value}
        </div>
        {detail && (
          <div className={cn("text-xs text-muted-foreground", wrap ? "whitespace-normal break-all" : "truncate")}>{detail}</div>
        )}
      </div>
    </div>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

export function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}
