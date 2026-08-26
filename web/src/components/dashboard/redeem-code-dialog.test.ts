import { describe, expect, it } from "vitest";

import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  availableRedeemChannels,
  initialRedeemChannel,
  redeemTargets,
} from "@/components/dashboard/redeem-code-dialog";

function account(id: bigint, channel: Channel): Account {
  return {
    $typeName: "mygardenworld.v1.Account",
    id,
    name: `account-${id.toString()}`,
    channel,
    username: `user-${id.toString()}`,
    aid: BigInt(0),
    gsIdx: 0,
    wsUrl: "",
    connected: false,
  };
}

describe("redeem code targets", () => {
  const accounts = [
    account(BigInt(1), Channel.IOS),
    account(BigInt(2), Channel.ALIPAY),
    account(BigInt(3), Channel.IOS),
  ];

  it("lists only channels that have accounts in stable product order", () => {
    expect(availableRedeemChannels(accounts)).toEqual([Channel.IOS, Channel.ALIPAY]);
    expect(availableRedeemChannels([account(BigInt(4), Channel.ALIPAY)])).toEqual([Channel.ALIPAY]);
  });

  it("selects every account from exactly one channel", () => {
    expect(redeemTargets(accounts, Channel.IOS).map((item) => item.id)).toEqual([BigInt(1), BigInt(3)]);
    expect(redeemTargets(accounts, Channel.ALIPAY).map((item) => item.id)).toEqual([BigInt(2)]);
  });

  it("prefers the current account channel and falls back to an available channel", () => {
    expect(initialRedeemChannel(accounts, Channel.ALIPAY)).toBe(Channel.ALIPAY);
    expect(initialRedeemChannel([account(BigInt(5), Channel.IOS)], Channel.ALIPAY)).toBe(Channel.IOS);
    expect(initialRedeemChannel([])).toBe(Channel.UNSPECIFIED);
  });
});
