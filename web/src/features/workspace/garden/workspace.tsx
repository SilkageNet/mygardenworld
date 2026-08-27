import { LandMonitorPanel } from "./status-panels";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function GardenWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      section="garden"
      props={props}
      statusContent={(
        <LandMonitorPanel
          lands={props.views.garden?.lands ?? []}
          waterDrops={props.views.basic?.waterDrops ?? 0}
          waterDropsTotal={props.views.basic?.waterDropsTotal ?? 0}
          minWaterDrops={props.policy?.plant?.planting?.minWaterDrops ?? 0}
        />
      )}
    />
  );
}
