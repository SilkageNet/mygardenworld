import { describe, expect, it } from "vitest";

import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  availableRedeemChannels,
  initialRedeemChannel,
  redeemTargets,
} from "@/components/dashboard/redeem-code-dialog";

function account(id: string, channel: Channel): Account {
  return {
    $typeName: "mygardenworld.v1.Account",
    id,
    name: `account-${id}`,
    channel,
    username: `user-${id}`,
    aid: BigInt(0),
    gsIdx: 0,
    wsUrl: "",
    connected: false,
  };
}

describe("redeem code targets", () => {
  const accounts = [
    account("ios-1", Channel.IOS),
    account("alipay-1", Channel.ALIPAY),
    account("ios-2", Channel.IOS),
  ];

  it("lists only channels that have accounts in stable product order", () => {
    expect(availableRedeemChannels(accounts)).toEqual([Channel.IOS, Channel.ALIPAY]);
    expect(availableRedeemChannels([account("alipay", Channel.ALIPAY)])).toEqual([Channel.ALIPAY]);
  });

  it("selects every account from exactly one channel", () => {
    expect(redeemTargets(accounts, Channel.IOS).map((item) => item.id)).toEqual(["ios-1", "ios-2"]);
    expect(redeemTargets(accounts, Channel.ALIPAY).map((item) => item.id)).toEqual(["alipay-1"]);
  });

  it("prefers the current account channel and falls back to an available channel", () => {
    expect(initialRedeemChannel(accounts, Channel.ALIPAY)).toBe(Channel.ALIPAY);
    expect(initialRedeemChannel([account("ios", Channel.IOS)], Channel.ALIPAY)).toBe(Channel.IOS);
    expect(initialRedeemChannel([])).toBe(Channel.UNSPECIFIED);
  });
});
