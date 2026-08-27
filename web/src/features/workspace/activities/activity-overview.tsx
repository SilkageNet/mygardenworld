import { BookOpenText, CircleCheckBig, Coins, ListTodo, ShieldCheck } from "lucide-react";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { itemName } from "@/lib/game/catalog";
import { EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";

export type ActivityProfile = {
  name: string;
  currencyItemId: number;
  mechanic: string;
  rewardPath: string;
  description: string;
  automation: string;
};

export const CYCLIC_NOTE_PROFILE: ActivityProfile = {
  name: "花笺集芳",
  currencyItemId: 1107,
  mechanic: "品质任务",
  rewardPath: "进度奖励 · 花笺商店",
  description: "完成不同品质的活动任务可获得集芳笺；累计集芳笺解锁进度奖励，也可在花笺商店兑换活动奖励。",
  automation: "自动同步活动，领取已完成任务与进度奖励；不执行钻石刷新或立即完成。",
};

export const CYCLIC_STORY_PROFILE: ActivityProfile = {
  name: "莳花纪闻",
  currencyItemId: 1108,
  mechanic: "鲜花订单",
  rewardPath: "进度奖励 · 莳花商店",
  description: "提交指定鲜花订单可获得花史残页；累计花史残页解锁进度奖励，也可在莳花商店兑换活动奖励。",
  automation: "自动同步活动，在库存充足时提交订单并领取进度奖励；不付费刷新或跳过冷却。",
};

type ActivityAvailability = {
  found?: boolean;
  observed?: boolean;
  phase?: number;
};

export function activityIsVisible<T extends ActivityAvailability>(activity?: T): activity is T & { found: true; phase: 1 | 2 | 3 } {
  return activity?.found === true && (activity.phase === 1 || activity.phase === 2 || activity.phase === 3);
}

export function ActivitySupportOverview({ activities }: { activities: (ActivityAvailability | undefined)[] }) {
  const synced = activities.filter((activity) => activity?.observed).length;
  const open = activities.filter(activityIsVisible).length;
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border/60 bg-card/82 px-3 py-3 shadow-sm sm:flex-row sm:items-center sm:justify-between sm:px-4">
      <div className="flex min-w-0 items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 dark:text-sky-300">
          <CircleCheckBig className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="text-sm font-semibold">活动监控</div>
          <div className="mt-0.5 text-xs leading-5 text-muted-foreground">完整支持花笺集芳与莳花纪闻；未开放时仍展示玩法与自动化边界。</div>
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap gap-1.5 pl-12 sm:pl-0">
        <Badge variant="outline">支持 2</Badge>
        <Badge variant="outline">已同步 {synced}/2</Badge>
        <Badge variant={open > 0 ? "secondary" : "outline"}>开放中 {open}/2</Badge>
      </div>
    </div>
  );
}

export function InactiveActivityOverview({
  activity,
  profile,
  currencyItemId,
}: {
  activity?: ActivityAvailability;
  profile: ActivityProfile;
  currencyItemId?: number;
}) {
  const observed = activity?.observed === true;
  const itemId = currencyItemId && currencyItemId > 0 ? currencyItemId : profile.currencyItemId;
  return (
    <div className="space-y-3">
      <EmptyState
        title={observed ? "当前没有开放批次" : "活动状态尚未同步"}
        detail={observed
          ? "游戏活动状态已同步，目前没有处于预告、进行中或领奖期的批次。"
          : "连接游戏并完成活动状态同步后，这里会自动显示当前批次。"}
      />
      <div className="grid gap-2 sm:grid-cols-3">
        <OverviewStat icon={<ListTodo />} label="核心玩法" value={profile.mechanic} detail="完成活动目标" />
        <OverviewStat icon={<Coins />} label="活动货币" value={itemName(itemId)} detail={`道具 #${itemId}`} />
        <OverviewStat icon={<BookOpenText />} label="奖励去向" value={profile.rewardPath} detail="活动结束后货币清空" compact wrap />
      </div>
      <div className="grid gap-2 lg:grid-cols-2">
        <ActivityInfo icon={<BookOpenText />} title="活动机制" detail={profile.description} />
        <ActivityInfo icon={<ShieldCheck />} title="自动化范围" detail={profile.automation} />
      </div>
    </div>
  );
}

function ActivityInfo({ icon, title, detail }: { icon: ReactNode; title: string; detail: string }) {
  return (
    <div className="flex min-w-0 gap-3 rounded-md border border-border/55 bg-muted/18 px-3 py-3">
      <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 dark:text-sky-300 [&_svg]:size-4">{icon}</div>
      <div className="min-w-0">
        <div className="text-sm font-semibold">{title}</div>
        <div className="mt-1 text-xs leading-5 text-muted-foreground">{detail}</div>
      </div>
    </div>
  );
}
