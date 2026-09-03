import { RedeemValidation } from "@/gen/mygardenworld/v1/redeem_pb";

export const EXPIRY_PRESETS = [
  { id: "5m", label: "5分钟", seconds: 5 * 60 },
  { id: "10m", label: "10分钟", seconds: 10 * 60 },
  { id: "30m", label: "30分钟", seconds: 30 * 60 },
  { id: "1h", label: "1小时", seconds: 60 * 60 },
  { id: "6h", label: "6小时", seconds: 6 * 60 * 60 },
  { id: "1d", label: "1天", seconds: 24 * 60 * 60 },
] as const;

export const REDEEM_PAGE_SIZES = [5, 10] as const;
export const DEFAULT_REDEEM_PAGE_SIZE = 5;

export type ExpiryMode = (typeof EXPIRY_PRESETS)[number]["id"] | "permanent" | "custom";
export type CustomUnit = "minutes" | "hours" | "days";

export function redeemPageCount(total: number, pageSize: number): number {
  if (!Number.isFinite(total) || total <= 0 || !Number.isFinite(pageSize) || pageSize <= 0) return 1;
  return Math.max(1, Math.ceil(total / pageSize));
}

const UNIT_SECONDS: Record<CustomUnit, number> = {
  minutes: 60,
  hours: 60 * 60,
  days: 24 * 60 * 60,
};

export function resolveRedeemExpiry(
  mode: ExpiryMode,
  customAmount: number,
  customUnit: CustomUnit,
  now = new Date(),
): Date | null {
  if (mode === "permanent") return null;
  const preset = EXPIRY_PRESETS.find((item) => item.id === mode);
  const seconds = preset?.seconds ?? customAmount * UNIT_SECONDS[customUnit];
  if (!Number.isFinite(seconds) || seconds <= 0) throw new Error("请输入有效的过期时间");
  return new Date(now.getTime() + seconds * 1000);
}

export function redeemValidationLabel(validation: RedeemValidation, expired: boolean, communityVerified = false): string {
  if (expired || validation === RedeemValidation.EXPIRED) return "已过期";
  switch (validation) {
    case RedeemValidation.SUCCESS:
      return "本节点已验证";
    case RedeemValidation.ALREADY_REDEEMED:
      return "已被兑换";
    case RedeemValidation.INVALID:
      return "错误码";
    case RedeemValidation.RETRYABLE:
      return "等待重试";
    case RedeemValidation.UNKNOWN:
      return "等待确认";
    default:
      return communityVerified ? "社区已验证" : "待验证";
  }
}
