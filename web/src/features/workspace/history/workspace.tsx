import { History, Loader2 } from "lucide-react";
import type { WorkspaceHistoryItem } from "@/gen/mygardenworld/v1/workspace_pb";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { WorkspaceProps } from "@/features/workspace/shared/domain-workspace";
import { EmptyState } from "@/features/workspace/shared/workspace-ui";
import { cn } from "@/lib/utils";
import { BusinessStatisticsPanel, RuntimeStatisticsPanel } from "./status-panels";

export default function HistoryWorkspace({
  views,
  status,
  items,
  hasMore,
  loading,
  onLoadMore,
}: Pick<WorkspaceProps, "views" | "status"> & {
  items: WorkspaceHistoryItem[];
  hasMore: boolean;
  loading: boolean;
  onLoadMore: () => void;
}) {
  const history = views.history;
  return (
    <div className="space-y-3 sm:space-y-4">
      <RuntimeStatisticsPanel runtimeStatistics={history?.runtimeStatistics ?? status?.runtimeStatistics} />
      <BusinessStatisticsPanel statistics={history?.businessStatistics} />
      <Card className="cloud-surface">
        <CardHeader><CardTitle className="flex items-center gap-2"><History className="size-4" />最近操作</CardTitle></CardHeader>
        <CardContent>
          {items.length ? (
            <div className="space-y-2">
              {items.map((item) => (
                <div key={item.id.toString()} className="flex flex-col gap-1 rounded-md border border-border/55 bg-white/40 px-3 py-2 dark:bg-white/5 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{item.label}</div>
                    <div className="break-words text-xs text-muted-foreground">{item.message}</div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2 text-xs">
                    {item.ts && <span className="text-muted-foreground">{new Date(Number(item.ts.seconds) * 1000).toLocaleString("zh-CN")}</span>}
                    <span className={cn("font-medium", item.outcome === "failed" ? "text-destructive" : "text-emerald-600 dark:text-emerald-400")}>{item.outcome === "failed" ? "失败" : "完成"}</span>
                  </div>
                </div>
              ))}
              {hasMore && (
                <div className="flex justify-center pt-2">
                  <Button type="button" variant="outline" size="sm" onClick={onLoadMore} disabled={loading}>
                    {loading && <Loader2 className="size-4 animate-spin" />}
                    {loading ? "加载中" : "加载更多"}
                  </Button>
                </div>
              )}
            </div>
          ) : <EmptyState title="暂无操作历史" detail="自动化执行后会在这里形成结构化记录。" />}
        </CardContent>
      </Card>
    </div>
  );
}
