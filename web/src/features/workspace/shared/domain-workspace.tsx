"use client";

import { useState, type ReactNode } from "react";
import { Activity, Settings2 } from "lucide-react";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AccountStatus, FeatureCapability } from "@/lib/api/workspace-models";
import PolicyPanel, { type PolicySection } from "./policy-panel";
import { cn } from "@/lib/utils";
import type { AccountViews } from "@/features/workspace/model";

type WorkspaceMode = "status" | "settings";

export type WorkspaceProps = {
  views: AccountViews;
  status?: AccountStatus;
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
];

export default function DomainWorkspace({ section, props, statusContent }: {
  section: PolicySection;
  props: WorkspaceProps;
  statusContent: ReactNode;
}) {
  const [mode, setMode] = useState<WorkspaceMode>("status");
  const settingsAvailable =
    section !== "union" || props.views.union?.inUnion === true;
  const activeMode: WorkspaceMode = settingsAvailable ? mode : "status";
  const availableModes = settingsAvailable
    ? WORKSPACE_MODES
    : WORKSPACE_MODES.filter((entry) => entry.id === "status");

  return (
    <div className="space-y-3 sm:space-y-4">
      <div className="dark-scrollbar flex gap-1 overflow-x-auto rounded-md border border-border/60 bg-white/45 p-1 dark:bg-white/5">
        {availableModes.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className={cn(
              "flex h-9 min-w-24 shrink-0 items-center justify-center gap-2 rounded px-3 text-sm font-semibold transition-colors",
              activeMode === entry.id
                ? "bg-white text-foreground shadow-sm dark:bg-muted"
                : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
            )}
            onClick={() => setMode(entry.id)}
          >
            <span className="[&_svg]:size-4">{entry.icon}</span>
            {entry.label}
          </button>
        ))}
      </div>
      {activeMode === "status" && statusContent}
      {activeMode === "settings" && (
        <PolicyPanel
          policy={props.policy}
          section={section}
          basicView={props.views.basic}
          garden={props.views.garden}
          orders={props.views.orders}
          warehouse={props.views.warehouse}
          capabilities={props.capabilities}
          loading={props.policyLoading}
          saving={props.savingPolicy}
          message={props.policyMessage}
          onPolicyChange={props.onPolicyChange}
          onSave={props.onPolicySave}
        />
      )}
    </div>
  );
}
