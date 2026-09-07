export type NotificationDraft = { enabled: boolean; endpoint: string; clearEndpoint: boolean; cooldown: string };

export function notificationSettingsUpdate(draft: NotificationDraft) {
  const minutes = Number(draft.cooldown);
  if (!Number.isInteger(minutes) || minutes < 1 || minutes > 1440) {
    throw new Error("冷却时间须为 1–1440 分钟的整数");
  }
  return {
    enabled: draft.clearEndpoint ? false : draft.enabled,
    cooldownMinutes: minutes,
    endpoint: draft.clearEndpoint ? "" : draft.endpoint.trim() || undefined,
  };
}
