import { TaskOrderMonitorPanel } from "./status-panels";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function OrdersWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      section="orders"
      props={props}
      statusContent={(
        <TaskOrderMonitorPanel
          tasks={props.views.orders?.pendingTasks ?? []}
          statistics={props.views.orders?.orderStatistics}
          videoOrders={props.views.orders?.residentVideoOrders ?? []}
          policy={props.policy}
        />
      )}
    />
  );
}
