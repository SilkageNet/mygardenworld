"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Activity, BookOpenText, Building2, Settings2 } from "lucide-react";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AccountStatus, Event, FeatureCapability } from "@/gen/mygardenworld/v1/query_service_pb";
import PolicyPanel, { type PolicySection } from "@/components/dashboard/policy-panel";
import {
  BusinessStatisticsPanel,
  EmptyState,
  EventPanel,
  FmlLandMonitorPanel,
  LandMonitorPanel,
  OperationPanel,
  RuntimeStatisticsPanel,
  StatusOverviewPanel,
  TaskOrderMonitorPanel,
  WarehouseMonitorPanel,
} from "@/components/dashboard/monitor-panels";
import {
  CyclicNoteMonitorPanel,
  CyclicStoryMonitorPanel,
  DessertMonitorPanel,
  FmlRaceMonitorPanel,
} from "@/components/dashboard/activity-monitor-panels";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { AccountViews } from "./model";

type WorkspaceMode = "status" | "settings" | "records";

type WorkspaceProps = {
  views: AccountViews;
  status?: AccountStatus;
  events: Event[];
  policy: Policy | null;
  capabilities: FeatureCapability[];
  policyLoading: boolean;
  savingPolicy: boolean;
  policyMessage: string;
  onPolicyChange: (policy: Policy | null) => void;
  onPolicySave: () => void;
};

const WORKSPACE_MODES: { id: WorkspaceMode; label: string; icon: ReactNode }[] = [
  { id: "status", label: "状态", icon: <Activity /> },
  { id: "settings", label: "设置", icon: <Settings2 /> },
  { id: "records", label: "记录", icon: <BookOpenText /> },
];

function WorkspaceModeBar({ activeMode, onChange }: { activeMode: WorkspaceMode; onChange: (mode: WorkspaceMode) => void }) {
  return (
    <div className="dark-scrollbar flex gap-1 overflow-x-auto rounded-md border border-border/60 bg-white/45 p-1 dark:bg-white/5">
      {WORKSPACE_MODES.map((mode) => (
        <button
          key={mode.id}
          type="button"
          className={cn(
            "flex h-9 min-w-24 shrink-0 items-center justify-center gap-2 rounded px-3 text-sm font-semibold transition-colors",
            activeMode === mode.id
              ? "bg-white text-foreground shadow-sm dark:bg-muted"
              : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
          )}
          onClick={() => onChange(mode.id)}
        >
          <span className="[&_svg]:size-4">{mode.icon}</span>
          {mode.label}
        </button>
      ))}
    </div>
  );
}

function DomainWorkspace({
  section,
  props,
  statusContent,
  recordEvents,
}: {
  section: PolicySection;
  props: WorkspaceProps;
  statusContent: ReactNode;
  recordEvents: Event[];
}) {
  const [mode, setMode] = useState<WorkspaceMode>("status");
  return (
    <div className="space-y-3 sm:space-y-4">
      <WorkspaceModeBar activeMode={mode} onChange={setMode} />
      {mode === "status" && statusContent}
      {mode === "settings" && (
        <PolicyPanel
          policy={props.policy}
          section={section}
          overview={props.views.overview}
          garden={props.views.garden}
          orders={props.views.orders}
          assets={props.views.assets}
          capabilities={props.capabilities}
          loading={props.policyLoading}
          saving={props.savingPolicy}
          message={props.policyMessage}
          onPolicyChange={props.onPolicyChange}
          onSave={props.onPolicySave}
        />
      )}
      {mode === "records" && <EventPanel events={recordEvents} />}
    </div>
  );
}

function useDomainEvents(events: Event[], matches: (event: Event) => boolean) {
  return useMemo(() => events.filter(matches), [events, matches]);
}

const overviewEvent = (event: Event) => ["account", "basic", "system"].includes(event.category);
const gardenEvent = (event: Event) => ["plant", "water"].includes(event.category);
const ordersEvent = (event: Event) => event.category === "order";
const unionEvent = (event: Event) => ["union", "race"].includes(event.category);
const activityEvent = (event: Event) => event.category === "activity";
const assetEvent = (event: Event) =>
  event.kind === "inventory_changed" ||
  event.kind === "resource_changed" ||
  ["shop", "pearl", "benefit", "mail", "sign", "zoo"].some((domain) => event.domain.includes(domain));

export function OverviewWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, overviewEvent);
  return (
    <DomainWorkspace
      section="routine"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <StatusOverviewPanel overview={props.views.overview} assets={props.views.assets} status={props.status} />
          <RuntimeStatisticsPanel runtimeStatistics={props.views.overview?.runtimeStatistics ?? props.status?.runtimeStatistics} />
          <OperationPanel operations={props.views.overview?.plannedOperations ?? []} />
          <TaskOrderMonitorPanel tasks={props.views.overview?.pendingTasks ?? []} statistics={props.views.orders?.orderStatistics} />
        </div>
      )}
    />
  );
}

export function GardenWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, gardenEvent);
  return (
    <DomainWorkspace
      section="garden"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <LandMonitorPanel
            lands={props.views.garden?.lands ?? []}
            waterDrops={props.views.overview?.waterDrops ?? 0}
            waterDropsTotal={props.views.overview?.waterDropsTotal ?? 0}
            minWaterDrops={props.policy?.plant?.planting?.minWaterDrops ?? 0}
          />
        </div>
      )}
    />
  );
}

export function OrdersWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, ordersEvent);
  return (
    <DomainWorkspace
      section="orders"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <TaskOrderMonitorPanel tasks={props.views.orders?.pendingTasks ?? []} statistics={props.views.orders?.orderStatistics} />
          <BusinessStatisticsPanel statistics={props.views.orders?.businessStatistics} />
        </div>
      )}
    />
  );
}

export function UnionWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, unionEvent);
  const union = props.views.union;
  if (!union?.membershipObserved) {
    return (
      <Card className="cloud-surface">
        <CardHeader><CardTitle>公会</CardTitle></CardHeader>
        <CardContent><EmptyState title="公会状态待同步" detail="确认公会成员状态前，不会运行或展示公会自动化。" /></CardContent>
      </Card>
    );
  }
  if (!union.inUnion) {
    return (
      <Card className="cloud-surface">
        <CardHeader><CardTitle>公会</CardTitle></CardHeader>
        <CardContent><EmptyState title="当前账号未加入公会" detail="公会土地、建设和竞赛逻辑已停用；加入公会并完成同步后才会开放。" /></CardContent>
      </Card>
    );
  }
  return (
    <DomainWorkspace
      section="union"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <Card className="cloud-surface">
            <CardHeader><CardTitle className="flex items-center gap-2"><Building2 className="size-4" />公会 #{union.unionId}</CardTitle></CardHeader>
          </Card>
          <FmlLandMonitorPanel
            lands={union.lands}
            plantableFlowers={props.views.garden?.plantableFlowers ?? []}
            observed={union.landsObserved}
            automationEnabled={props.policy?.automationEnabled ?? false}
          />
          <FmlRaceMonitorPanel
            race={union.race}
            showTakenTask={props.policy?.union?.race?.enabled ?? true}
            showPersonalScoreRank={props.policy?.union?.race?.showPersonalScoreRank ?? false}
          />
        </div>
      )}
    />
  );
}

export function ActivitiesWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, activityEvent);
  return (
    <DomainWorkspace
      section="activities"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <CyclicNoteMonitorPanel activity={props.views.activities?.cyclicNote} />
          <CyclicStoryMonitorPanel activity={props.views.activities?.cyclicStory} />
          <DessertMonitorPanel activity={props.views.activities?.dessert} />
        </div>
      )}
    />
  );
}

export function AssetsWorkspace(props: WorkspaceProps) {
  const records = useDomainEvents(props.events, assetEvent);
  return (
    <DomainWorkspace
      section="assets"
      props={props}
      recordEvents={records}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <WarehouseMonitorPanel ledger={props.views.assets?.inventoryLedger} />
        </div>
      )}
    />
  );
}
