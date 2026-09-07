import { describe, expect, it } from "vitest";
import { create, toJson } from "@bufbuild/protobuf";
import { SaveNotificationSettingsRequestSchema } from "@/gen/mygardenworld/v1/notification_pb";
import { notificationSettingsUpdate } from "./settings-model";

describe("personal notification settings", () => {
  const draft = { enabled: true, endpoint: "", clearEndpoint: false, cooldown: "30" };

  it("retains saved credentials without sending a user or game account id", () => {
    const message = create(SaveNotificationSettingsRequestSchema, notificationSettingsUpdate(draft));
    expect(toJson(SaveNotificationSettingsRequestSchema, message)).toEqual({ enabled: true, cooldownMinutes: 30 });
  });

  it("distinguishes explicit credential removal from leaving the field blank", () => {
    const message = create(SaveNotificationSettingsRequestSchema, notificationSettingsUpdate({ ...draft, clearEndpoint: true }));
    expect(message.enabled).toBe(false);
    expect(toJson(SaveNotificationSettingsRequestSchema, message)).toEqual({ endpoint: "", cooldownMinutes: 30 });
  });

  it("replaces the endpoint only with explicitly entered content", () => {
    expect(notificationSettingsUpdate({ ...draft, endpoint: " https://example.com/hook " }).endpoint).toBe("https://example.com/hook");
    expect(notificationSettingsUpdate({ ...draft, endpoint: "  " }).endpoint).toBeUndefined();
  });

  it.each(["", "0", "-1", "1.5", "1441", "NaN"])("rejects invalid cooldown %s", (cooldown) => {
    expect(() => notificationSettingsUpdate({ ...draft, cooldown })).toThrow("1–1440");
  });
});
