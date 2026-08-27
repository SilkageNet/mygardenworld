import { WarehouseMonitorPanel } from "./status-panels";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function WarehouseWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      props={props}
      statusContent={<WarehouseMonitorPanel ledger={props.views.warehouse?.inventoryLedger} />}
    />
  );
}
