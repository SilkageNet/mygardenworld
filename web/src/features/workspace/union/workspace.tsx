import type { ReactNode } from "react";
import { Building2 } from "lucide-react";
import FmlRaceMonitorPanel from "./race-panel";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";
import { EmptyState } from "@/features/workspace/shared/workspace-ui";
import { FmlLandMonitorPanel } from "./status-panels";
import { VideoActionItems } from "@/features/workspace/shared/video-action-status";

export default function UnionWorkspace(props: WorkspaceProps) {
  const union = props.views.union;
  let statusContent: ReactNode;
  if (!union?.membershipObserved) {
    statusContent = (
      <Card className="cloud-surface">
        <CardHeader><CardTitle>公会</CardTitle></CardHeader>
        <CardContent><EmptyState title="公会状态待同步" detail="确认会员状态前，服务端不会规划或执行任何公会操作。" /></CardContent>
      </Card>
    );
  } else if (!union.inUnion) {
    statusContent = (
      <Card className="cloud-surface">
        <CardHeader><CardTitle>公会</CardTitle></CardHeader>
        <CardContent><EmptyState title="当前账号未加入公会" detail="公会土地、建设和竞赛模块均保持停用，加入公会并同步后才会运行。" /></CardContent>
      </Card>
    );
  } else {
    statusContent = (
      <div className="space-y-3 sm:space-y-4">
        <Card className="cloud-surface">
          <CardHeader><CardTitle className="flex items-center gap-2"><Building2 className="size-4" />公会 #{union.unionId}{union.memberPositionObserved && union.memberPositionLabel ? ` · ${union.memberPositionLabel}` : ""}</CardTitle></CardHeader>
          {union.videoBuild && (
            <CardContent className="pt-0">
              <VideoActionItems actions={[union.videoBuild]} />
            </CardContent>
          )}
        </Card>
        <FmlLandMonitorPanel
          lands={union.lands}
          plantableFlowers={props.views.garden?.plantableFlowers ?? []}
          observed={union.landsObserved}
          automationEnabled={props.policy?.automationEnabled ?? false}
        />
        <FmlRaceMonitorPanel
          accountId={union.accountId}
          race={union.race}
          showTakenTask={props.policy?.union?.race?.enabled ?? true}
          showPersonalScoreRank={props.policy?.union?.race?.showPersonalScoreRank ?? false}
          canDeleteTasks={union.raceDeleteAllowed}
        />
      </div>
    );
  }
  return <DomainWorkspace section="union" props={props} statusContent={statusContent} />;
}
