import { fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { PolicySchema, type Policy } from "@/gen/mygardenworld/v1/policy_pb";

export function exportPolicyJSON(policy: Policy): string {
  return toJsonString(PolicySchema, policy, { prettySpaces: 2, alwaysEmitImplicit: true, useProtoFieldName: true });
}

export function importPolicyJSON(text: string, current: Policy): Policy {
  if (text.length > 1024 * 1024) throw new Error("配置 JSON 不能超过 1 MB");
  const policy = fromJsonString(PolicySchema, text, { ignoreUnknownFields: false });
  if (policy.schemaVersion !== current.schemaVersion) throw new Error("配置版本与当前版本不一致，请使用当前版本导出的 JSON");
  for (const key of ["basic", "plant", "order", "union", "activity"] as const) {
    if (!policy[key]) throw new Error(`缺少 ${key} 配置，请导入完整配置 JSON`);
  }
  // Account start/pause is a lifecycle command, not an import side effect.
  policy.automationEnabled = current.automationEnabled;
  return policy;
}
