import { describe, expect, it } from "vitest";
import { create, toJson } from "@bufbuild/protobuf";
import { NotificationProvider, SaveNotificationSettingsRequestSchema } from "@/gen/mygardenworld/v1/notification_pb";
import { notificationSettingsUpdate } from "./settings-model";

describe("personal notification settings", () => {
  const draft = { enabled: true, endpoint: "", clearEndpoint: false, cooldown: "30", provider: NotificationProvider.CUSTOM, savedProvider: NotificationProvider.CUSTOM, signingSecret: "", clearSigningSecret: false };

  it("retains saved credentials without sending a user or game account id", () => {
    const message = create(SaveNotificationSettingsRequestSchema, notificationSettingsUpdate(draft));
    expect(toJson(SaveNotificationSettingsRequestSchema, message)).toEqual({ enabled: true, cooldownMinutes: 30, provider: "NOTIFICATION_PROVIDER_CUSTOM", signingSecret: "" });
  });

  it("distinguishes explicit credential removal from leaving the field blank", () => {
    const message = create(SaveNotificationSettingsRequestSchema, notificationSettingsUpdate({ ...draft, clearEndpoint: true }));
    expect(message.enabled).toBe(false);
    expect(toJson(SaveNotificationSettingsRequestSchema, message)).toEqual({ endpoint: "", cooldownMinutes: 30, provider: "NOTIFICATION_PROVIDER_CUSTOM", signingSecret: "" });
  });

  it("replaces the endpoint only with explicitly entered content", () => {
    expect(notificationSettingsUpdate({ ...draft, endpoint: " https://example.com/hook " }).endpoint).toBe("https://example.com/hook");
    expect(notificationSettingsUpdate({ ...draft, endpoint: "  " }).endpoint).toBeUndefined();
  });

  it.each(["", "0", "-1", "1.5", "1441", "NaN"])("rejects invalid cooldown %s", (cooldown) => {
    expect(() => notificationSettingsUpdate({ ...draft, cooldown })).toThrow("1–1440");
  });

  it("requires an explicit endpoint when changing providers", () => {
    expect(() => notificationSettingsUpdate({ ...draft, provider: NotificationProvider.FEISHU })).toThrow("新的接收地址");
    expect(notificationSettingsUpdate({ ...draft, provider: NotificationProvider.FEISHU, clearEndpoint: true }).enabled).toBe(false);
  });

  it.each([NotificationProvider.DINGTALK, NotificationProvider.FEISHU])("retains, replaces or clears signing credentials for %s", (provider) => {
    const signed = { ...draft, provider, savedProvider: provider };
    expect(notificationSettingsUpdate(signed).signingSecret).toBeUndefined();
    expect(notificationSettingsUpdate({ ...signed, signingSecret: " signature " }).signingSecret).toBe("signature");
    expect(notificationSettingsUpdate({ ...signed, clearSigningSecret: true }).signingSecret).toBe("");
    expect(notificationSettingsUpdate({ ...signed, clearEndpoint: true, signingSecret: "signature" }).signingSecret).toBe("");
  });

  it("never sends a signing secret to custom or WeCom", () => {
    expect(notificationSettingsUpdate({ ...draft, signingSecret: "secret" }).signingSecret).toBe("");
    expect(notificationSettingsUpdate({ ...draft, provider: NotificationProvider.WECOM, savedProvider: NotificationProvider.WECOM, signingSecret: "secret" }).signingSecret).toBe("");
  });

  it("rejects unknown providers", () => {
    expect(() => notificationSettingsUpdate({ ...draft, provider: NotificationProvider.UNSPECIFIED })).toThrow("支持的通知渠道");
  });
});
