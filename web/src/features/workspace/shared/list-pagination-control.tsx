import type { ReactNode } from "react";
import { ChevronLeft, ChevronRight, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";

export function ListPaginationControl({
  ariaLabel,
  summary,
  pageIndex,
  totalPages,
  previousDisabled,
  nextDisabled,
  nextLoading = false,
  onPrevious,
  onNext,
}: {
  ariaLabel: string;
  summary: ReactNode;
  pageIndex: number;
  totalPages?: number;
  previousDisabled: boolean;
  nextDisabled: boolean;
  nextLoading?: boolean;
  onPrevious: () => void;
  onNext: () => void;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/58 bg-secondary/20 px-2.5 py-1.5 text-xs text-muted-foreground">
      <div className="min-w-0">{summary}</div>
      <nav className="flex items-center gap-1" aria-label={ariaLabel}>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-7 px-1.5"
          onClick={onPrevious}
          disabled={previousDisabled}
          aria-label="上一页"
        >
          <ChevronLeft className="size-3.5" />
          <span className="hidden min-[420px]:inline">上一页</span>
        </Button>
        <span className="min-w-14 text-center tabular-nums text-foreground/80">
          {totalPages === undefined ? `第 ${pageIndex + 1} 页` : `${pageIndex + 1} / ${totalPages}`}
        </span>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-7 px-1.5"
          onClick={onNext}
          disabled={nextDisabled}
          aria-label="下一页"
        >
          {nextLoading ? <Loader2 className="size-3.5 animate-spin" /> : <span className="hidden min-[420px]:inline">下一页</span>}
          <ChevronRight className="size-3.5" />
        </Button>
      </nav>
    </div>
  );
}
