import { OperationPanel, StatusOverviewPanel } from "./status-panels";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function BasicWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      section="basic"
      props={props}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <StatusOverviewPanel basic={props.views.basic} warehouse={props.views.warehouse} status={props.status} />
          <OperationPanel operations={props.views.basic?.plannedOperations ?? []} />
        </div>
      )}
    />
  );
}
