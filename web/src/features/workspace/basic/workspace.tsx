import { OperationPanel, PearlHirePanel, StatusOverviewPanel } from "./status-panels";
import { RedeemAttemptsPanel } from "./redeem-attempts-panel";
import type { RedeemAttemptFeed } from "./redeem-attempts-model";
import type { AccountRedeemAttemptFilter } from "@/gen/mygardenworld/v1/workspace_pb";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function BasicWorkspace(props: WorkspaceProps & {
  redeemFeed: RedeemAttemptFeed;
  onRedeemFilterChange: (filter: AccountRedeemAttemptFilter) => void;
  onLoadMoreRedeemAttempts: () => void;
}) {
  return (
    <DomainWorkspace
      section="basic"
      props={props}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <StatusOverviewPanel basic={props.views.basic} warehouse={props.views.warehouse} status={props.status} />
          <RedeemAttemptsPanel
            feed={props.redeemFeed}
            onFilterChange={props.onRedeemFilterChange}
            onLoadMore={props.onLoadMoreRedeemAttempts}
          />
          <PearlHirePanel pearlHire={props.views.basic?.pearlHire} />
          <OperationPanel operations={props.views.basic?.plannedOperations ?? []} />
        </div>
      )}
    />
  );
}
