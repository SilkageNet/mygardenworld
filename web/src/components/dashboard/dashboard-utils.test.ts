import { describe, expect, it } from "vitest";

import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { AccountStatus, Event } from "@/lib/api/query-models";
import {
  accountAreaLabel,
  accountNickname,
  channelLabel,
  collapseRaceSyncLogEvents,
  isTransientConnectionMessage,
  landDisplayNumber,
} from "@/components/dashboard/dashboard-utils";

function account(overrides: Partial<Account> = {}): Account {
  return {
    $typeName: "mygardenworld.v1.Account",
    id: BigInt(1),
    name: "海棠 · 第3区",
    channel: Channel.IOS,
    username: "game",
    aid: BigInt(0),
    gsIdx: 0,
    wsUrl: "",
    connected: false,
    ...overrides,
  };
}

function event(overrides: Partial<Event>): Event {
  return {
    $typeName: "mygardenworld.v1.Event",
    id: BigInt(1),
    accountId: BigInt(1),
    accountName: "main",
    kind: "operation_ack",
    message: "",
    payloadJson: "",
    category: "race",
    domain: "union.race.sync",
    action: "sync",
    label: "同步竞赛任务",
    level: "info",
    ...overrides,
  };
}

describe("dashboard account labels", () => {
  it("separates nickname, area and channel", () => {
    const value = account();
    expect(accountNickname(value)).toBe("海棠");
    expect(accountAreaLabel(value)).toBe("第3区");
    expect(channelLabel(value.channel)).toBe("iOS");
    expect(channelLabel(Channel.ALIPAY)).toBe("支付宝");
  });

  it("prefers the observed game-server index", () => {
    expect(accountAreaLabel(account(), { gsIdx: 8 } as AccountStatus)).toBe("第8区");
  });
});

describe("dashboard event and status helpers", () => {
  it("collapses repeated race sync completions but keeps other events", () => {
    const other = event({ id: BigInt(3), kind: "session", domain: "account.session", label: "连接" });
    const newest = event({ id: BigInt(2) });
    const older = event({ id: BigInt(1) });
    const planned = event({ id: BigInt(0), kind: "operation_planned" });
    expect(collapseRaceSyncLogEvents([other, newest, older, planned])).toEqual([other, newest]);
  });

  it("recognizes retryable connection messages without hiding auth failures", () => {
    expect(isTransientConnectionMessage("failed to fetch")).toBe(true);
    expect(isTransientConnectionMessage("后端服务暂时不可用")).toBe(true);
    expect(isTransientConnectionMessage("invalid password")).toBe(false);
  });

  it("formats observed land identifiers", () => {
    expect(landDisplayNumber(1007)).toBe(7);
    expect(landDisplayNumber(42)).toBe(42);
  });
});
