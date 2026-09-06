import { create, equals } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { exportPolicyJSON, importPolicyJSON } from "./policy-json";

const policy = create(PolicySchema, {
  schemaVersion: 3, automationEnabled: false,
  basic: { displacedSessionReloginEnabled: true },
  plant: { friendSteal: { excludeUids: [BigInt("9007199254740993")] } },
  order: {}, union: { race: { upgradeTask: true, maxSpendDiamond: BigInt(73), deleteIntervalSeconds: 180 } }, activity: {},
});

describe("configuration JSON", () => {
  it("round trips all modules and large IDs without precision loss", () => {
    const text = exportPolicyJSON(policy);
    expect(text).toContain('"9007199254740993"');
    expect(importPolicyJSON(text, policy).union?.race?.deleteIntervalSeconds).toBe(180);
    expect(text).not.toContain("$typeName");
    expect(equals(PolicySchema, importPolicyJSON(text, policy), policy)).toBe(true);
  });
  it("does not start a paused account when importing running settings", () => {
    const text = exportPolicyJSON({ ...policy, automationEnabled: true });
    expect(importPolicyJSON(text, policy).automationEnabled).toBe(false);
  });
  it.each(["not json", "null", "[]", "{}", '{"schema_version":2}', exportPolicyJSON(policy).replace('"basic":', '"unknown":')])("rejects malformed, incomplete or incompatible configuration: %s", (text) => {
    expect(() => importPolicyJSON(text, policy)).toThrow();
  });
  it("rejects unknown fields instead of silently dropping settings", () => {
    const text = JSON.stringify({ ...JSON.parse(exportPolicyJSON(policy)), obsoleteSetting: true });
    expect(() => importPolicyJSON(text, policy)).toThrow();
  });
});
