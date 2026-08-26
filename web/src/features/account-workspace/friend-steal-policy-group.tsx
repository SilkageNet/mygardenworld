import { Users } from "lucide-react";
import { SelectionMode, type FriendStealPolicy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AssetsView, FeatureCapability, GardenView } from "@/lib/api/query-models";
import { CatalogFlowerMultiSelectRow } from "@/components/dashboard/flower-picker-controls";
import {
  FriendTouchFriendList,
  NumberRow,
  PolicyGroup,
  QualityRow,
  SegmentedRow,
  ToggleRow,
} from "@/components/dashboard/policy-controls";
import { settingStatusForCapability } from "@/lib/feature-capabilities";

const FRIEND_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部可摘" },
  { value: SelectionMode.SPECIFIC, label: "指定次数" },
];

const FLOWER_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.QUALITY, label: "品质" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

export default function FriendStealPolicyGroup({
  policy,
  garden,
  assets,
  capabilities,
  onChange,
  onCountChange,
  onExcludedChange,
}: {
  policy?: FriendStealPolicy;
  garden: GardenView | null;
  assets: AssetsView | null;
  capabilities: FeatureCapability[];
  onChange: (patch: Partial<FriendStealPolicy>) => void;
  onCountChange: (uid: bigint, count: number) => void;
  onExcludedChange: (uid: bigint, excluded: boolean) => void;
}) {
  const flowerMode = policy?.mode || SelectionMode.ALL;
  const friendMode = policy?.friendMode || SelectionMode.ALL;

  return (
    <PolicyGroup title="好友摸花" icon={<Users />}>
      <p className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
        仅自动摸取服务端明确标记为可摸的成熟鲜花；花灵摸取尚缺少状态与成功回包实测，因此不会发送{" "}
        <code>stealElves=1</code>。
      </p>
      <div className="grid gap-2 sm:grid-cols-2">
        <ToggleRow
          label="自动摸花"
          checked={policy?.enabled ?? false}
          onChange={(enabled) => onChange({ enabled })}
          status={settingStatusForCapability(capabilities, "plant.friend_steal")}
        />
        <SegmentedRow
          label="好友范围"
          value={friendMode}
          options={FRIEND_MODE_OPTIONS}
          onChange={(value) => onChange({ friendMode: value })}
        />
        <SegmentedRow
          label="鲜花范围"
          value={flowerMode}
          options={FLOWER_MODE_OPTIONS}
          onChange={(value) => onChange({ mode: value })}
        />
        {flowerMode === SelectionMode.QUALITY && (
          <QualityRow
            label="指定品质"
            value={policy?.qualities ?? []}
            onChange={(qualities) => onChange({ qualities })}
          />
        )}
        {flowerMode === SelectionMode.SPECIFIC && (
          <CatalogFlowerMultiSelectRow
            label="指定鲜花"
            value={policy?.flowerIds ?? []}
            inventory={assets?.inventory ?? {}}
            synced={Boolean(assets)}
            onChange={(flowerIds) => onChange({ flowerIds })}
          />
        )}
        {flowerMode === SelectionMode.EXCLUDE && (
          <CatalogFlowerMultiSelectRow
            label="排除鲜花"
            value={policy?.excludeFlowerIds ?? []}
            inventory={assets?.inventory ?? {}}
            synced={Boolean(assets)}
            onChange={(excludeFlowerIds) => onChange({ excludeFlowerIds })}
          />
        )}
        <ToggleRow
          label="友情币兑换次数"
          checked={policy?.autoBuyTimes ?? false}
          onChange={(autoBuyTimes) => onChange({ autoBuyTimes })}
          status={settingStatusForCapability(capabilities, "plant.friend_steal_buy")}
        />
        <NumberRow
          label="每好友兑换上限"
          value={policy?.maxBuyPerFriend || 0}
          min={0}
          max={10}
          onChange={(maxBuyPerFriend) => onChange({ maxBuyPerFriend })}
          description="每次消耗 1 友情币；0 使用静态目录 $pickMax（当前为 10）"
        />
      </div>
      <FriendTouchFriendList
        friends={garden?.friendTouchFriends ?? []}
        observed={garden?.friendTouchFriendsObserved ?? false}
        mode={friendMode}
        counts={policy?.friendCounts ?? {}}
        excluded={new Set((policy?.excludeUids ?? []).map((uid) => uid.toString()))}
        autoBuy={policy?.autoBuyTimes ?? false}
        maxBuyPerFriend={policy?.maxBuyPerFriend || 10}
        onCountChange={onCountChange}
        onExcludedChange={onExcludedChange}
      />
    </PolicyGroup>
  );
}
