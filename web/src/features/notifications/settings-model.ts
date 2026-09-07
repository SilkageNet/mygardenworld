import { NotificationProvider } from "@/gen/mygardenworld/v1/notification_pb";

export const notificationProviders = [
  { value: NotificationProvider.CUSTOM, label: "自定义 Webhook", placeholder: "https://example.com/webhook" },
  { value: NotificationProvider.WECOM, label: "企业微信群机器人", placeholder: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…" },
  { value: NotificationProvider.DINGTALK, label: "钉钉群机器人", placeholder: "https://oapi.dingtalk.com/robot/send?access_token=…" },
  { value: NotificationProvider.FEISHU, label: "飞书群机器人", placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/…" },
];

export function supportsSigning(provider: NotificationProvider) {
  return provider === NotificationProvider.DINGTALK || provider === NotificationProvider.FEISHU;
}

export type NotificationDraft = {
  enabled: boolean; endpoint: string; clearEndpoint: boolean; cooldown: string;
  provider: NotificationProvider; savedProvider: NotificationProvider;
  signingSecret: string; clearSigningSecret: boolean;
};

export function notificationSettingsUpdate(draft: NotificationDraft) {
  const minutes = Number(draft.cooldown);
  if (!Number.isInteger(minutes) || minutes < 1 || minutes > 1440) {
    throw new Error("冷却时间须为 1–1440 分钟的整数");
  }
  if (!notificationProviders.some((item) => item.value === draft.provider)) {
    throw new Error("请选择支持的通知渠道");
  }
  if (draft.provider !== draft.savedProvider && !draft.endpoint.trim() && !draft.clearEndpoint) {
    throw new Error("切换渠道时请填写新的接收地址");
  }
  return {
    provider: draft.provider,
    enabled: draft.clearEndpoint ? false : draft.enabled,
    cooldownMinutes: minutes,
    endpoint: draft.clearEndpoint ? "" : draft.endpoint.trim() || undefined,
    signingSecret: draft.clearEndpoint || draft.clearSigningSecret || !supportsSigning(draft.provider) ? "" : draft.signingSecret.trim() || undefined,
  };
}
