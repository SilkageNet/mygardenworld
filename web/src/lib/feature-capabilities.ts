import { PlanStatus, type FeatureCapability } from "@/gen/mygardenworld/v1/query_service_pb";

export type SettingStatusKind = "sync_only" | "adapter_missing" | "blocked";

export type SettingStatus = {
  kind: SettingStatusKind;
  label: string;
  detail: string;
};

export function settingStatusForCapability(
  capabilities: FeatureCapability[],
  featureID: string,
): SettingStatus | undefined {
  const capability = capabilities.find((candidate) => candidate.id === featureID);
  if (!capability) {
    return {
      kind: "blocked",
      label: "未知",
      detail: "后端未声明该能力，界面不会把它视为可执行。",
    };
  }

  const detail = capability.blockedReasons.join("；");
  switch (capability.status) {
    case PlanStatus.SYNC_ONLY:
      return {
        kind: "sync_only",
        label: "同步",
        detail: detail || "当前只做状态或需求展示，不会自动执行。",
      };
    case PlanStatus.ADAPTER_MISSING:
      return {
        kind: "adapter_missing",
        label: "阻塞",
        detail: detail || "执行协议、状态或成本门槛尚不完整，暂不自动执行。",
      };
    case PlanStatus.BLOCKED:
      return {
        kind: "blocked",
        label: "阻塞",
        detail: detail || "该能力当前被后端安全门禁阻塞。",
      };
    default:
      return undefined;
  }
}
