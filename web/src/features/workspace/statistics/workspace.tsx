import type { WorkspaceProps } from "@/features/workspace/shared/domain-workspace";
import { BusinessStatisticsPanel, RuntimeStatisticsPanel } from "./status-panels";

export default function StatisticsWorkspace({
  views,
  status,
}: Pick<WorkspaceProps, "views" | "status">) {
  const statistics = views.statistics;
  return (
    <div className="space-y-3 sm:space-y-4">
      <RuntimeStatisticsPanel runtimeStatistics={statistics?.runtimeStatistics ?? status?.runtimeStatistics} />
      <BusinessStatisticsPanel statistics={statistics?.businessStatistics} />
    </div>
  );
}
