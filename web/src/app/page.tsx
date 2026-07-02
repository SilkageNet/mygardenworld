"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import {
  AlertTriangle,
  BadgeCheck,
  Building2,
  CalendarDays,
  Coins,
  Flower2,
  Gem,
  HandCoins,
  ListChecks,
  LogIn,
  LogOut,
  Package,
  Play,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
  Square,
  Sprout,
  Trash2,
  Trophy,
  Users,
  Waves,
} from "lucide-react";

import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { AutomationService } from "@/gen/mygardenworld/v1/automation_service_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  ActivityModulePolicySchema,
  ActivityPolicySchema,
  BasicPolicySchema,
  BasicTaskPolicySchema,
  BenefitPolicySchema,
  CultivatePolicySchema,
  CustomerOrderPolicySchema,
  FeedCatPolicySchema,
  FlowerElvesPolicySchema,
  FlowerMarketPolicySchema,
  FlowerPlantPolicySchema,
  FlowerArtPolicySchema,
  FriendStealPolicySchema,
  IntListSchema,
  MarketBuyMode,
  MarketPutMode,
  OrderPolicySchema,
  PalaceOrderPolicySchema,
  PearlPolicySchema,
  PlantPolicySchema,
  PlantingMode,
  PolicySchema,
  ReputationPolicySchema,
  ResidentOrderPolicySchema,
  SelectionMode,
  SignPolicySchema,
  ShopBuyPolicySchema,
  ShopPolicySchema,
  TeamOrderPolicySchema,
  UnionBuildPolicySchema,
  UnionFlowerPolicySchema,
  UnionLandPolicySchema,
  UnionPolicySchema,
  UnionRacePolicySchema,
  VipShopPolicySchema,
} from "@/gen/mygardenworld/v1/policy_pb";
import type {
  ActivityModulePolicy,
  ActivityPolicy,
  BasicPolicy,
  BasicTaskPolicy,
  BenefitPolicy,
  CultivatePolicy,
  CustomerOrderPolicy,
  FeedCatPolicy,
  FlowerElvesPolicy,
  FlowerMarketPolicy,
  FlowerArtPolicy,
  FlowerPlantPolicy,
  FriendStealPolicy,
  OrderPolicy,
  PalaceOrderPolicy,
  PearlPolicy,
  PlantPolicy,
  Policy,
  ReputationPolicy,
  ResidentOrderPolicy,
  ShopBuyPolicy,
  ShopPolicy,
  SignPolicy,
  TeamOrderPolicy,
  UnionBuildPolicy,
  UnionFlowerPolicy,
  UnionLandPolicy,
  UnionPolicy,
  UnionRacePolicy,
  VipShopPolicy,
} from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import { QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import type {
  AccountStatus,
  DemandView,
  DomainStatus,
  Event,
  FlowerArtAvailabilityView,
  GetSnapshotResponse,
  PlannedOperation,
  VaseView,
} from "@/gen/mygardenworld/v1/query_service_pb";
import AppShell from "@/components/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { transport } from "@/lib/api/client";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

const accountClient = createClient(AccountService, transport);
const automationClient = createClient(AutomationService, transport);
const policyClient = createClient(PolicyService, transport);
const queryClient = createClient(QueryService, transport);

const EVENT_LIMIT = 120;
const STATUS_POLL_MS = 5000;
const SNAPSHOT_REFRESH_EVENT_KINDS = new Set([
  "operation_ack",
  "resource_changed",
  "inventory_changed",
  "land_changed",
  "order_finish",
  "order_customer",
  "flower_art",
  "flower_rack",
  "task_recv",
  "waterwheel",
  "free_water",
  "benefit_box",
]);

const GOAL_OPTIONS = [
  { id: "order.customer", label: "顾客订单", defaultPriority: 10 },
  { id: "order.resident", label: "居民任务", defaultPriority: 20 },
  { id: "order.flower_art", label: "花艺/花架", defaultPriority: 30 },
  { id: "basic.task.main", label: "主线任务", defaultPriority: 40 },
  { id: "basic.task.daily", label: "日常任务", defaultPriority: 50 },
  { id: "basic.task.weekly", label: "周常任务", defaultPriority: 60 },
  { id: "fallback.low_stock", label: "低库存补种", defaultPriority: 1000 },
];

type PolicyTabId = "basic" | "plant" | "order" | "union" | "activity";

const POLICY_TABS: { id: PolicyTabId; label: string; icon: ReactNode }[] = [
  { id: "basic", label: "基础", icon: <ShieldCheck /> },
  { id: "plant", label: "种植", icon: <Sprout /> },
  { id: "order", label: "订单", icon: <ListChecks /> },
  { id: "union", label: "公会", icon: <Users /> },
  { id: "activity", label: "活动", icon: <CalendarDays /> },
];

const QUALITY_OPTIONS = [1, 2, 3, 4, 5];

const PLANTING_MODE_OPTIONS = [
  { value: PlantingMode.QUALITY, label: "品质" },
  { value: PlantingMode.COUNT, label: "数量" },
  { value: PlantingMode.SPECIFIC, label: "指定" },
];

const SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.QUALITY, label: "品质" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

const MARKET_PUT_MODE_OPTIONS = [
  { value: MarketPutMode.INVENTORY, label: "库存最多" },
  { value: MarketPutMode.SPECIFIC, label: "指定花朵" },
];

const MARKET_BUY_MODE_OPTIONS = [
  { value: MarketBuyMode.ALL, label: "全部" },
  { value: MarketBuyMode.SPECIFIC, label: "指定花朵" },
  { value: MarketBuyMode.QUALITY, label: "指定品质" },
];

const UNION_RACE_TASKS = [2004, 3006, 3016, 3017, 3018, 3023, 3024, 3030, 3034, 3035, 3036, 3044, 3052];

type SettingStatusKind = "sync_only" | "adapter_missing" | "paused";

type SettingStatus = {
  kind: SettingStatusKind;
  label: string;
  detail: string;
};

const SETTING_STATUS = {
  syncOnly: { kind: "sync_only", label: "同步", detail: "策略项已登记，当前只做状态/需求展示，暂不自动执行。" },
  adapterMissing: { kind: "adapter_missing", label: "阻塞", detail: "需要补协议、状态或成本门槛，暂不自动执行。" },
  videoTokenMissing: { kind: "adapter_missing", label: "阻塞", detail: "依赖客户端 SDK 广告 token，本地 runner 不伪造视频完成。" },
  paused: { kind: "paused", label: "暂停", detail: "本阶段不继续拓展该功能，已接入能力保留运行。" },
} satisfies Record<string, SettingStatus>;

type ActivityModuleMeta = {
  id: string;
  label: string;
  status?: SettingStatus;
  boolParams?: { key: string; label: string }[];
  intParams?: { key: string; label: string; min?: number }[];
  stringParams?: { key: string; label: string }[];
};

const ACTIVITY_MODULES: ActivityModuleMeta[] = [
  { id: "cyclicNote", label: "花笺集芳", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "unlock_slot", label: "解锁任务槽" }, { key: "auto_enable_modules", label: "自动启用模块" }] },
  { id: "actCyclicStory", label: "莳花纪闻", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "refresh_enabled", label: "自动刷新" }], intParams: [{ key: "max_finish_count_per_batch", label: "每批完成数", min: 0 }] },
  { id: "fishMerge", label: "丰仓鱼干", status: SETTING_STATUS.adapterMissing, boolParams: [{ key: "show_result", label: "显示结果" }, { key: "auto_restart", label: "自动重开" }] },
  { id: "magicBubble", label: "奇妙泡泡", status: SETTING_STATUS.adapterMissing },
  { id: "zooGameElim", label: "花香满园", status: SETTING_STATUS.syncOnly },
  { id: "fishFun", label: "鱼乐无穷", status: SETTING_STATUS.adapterMissing, boolParams: [{ key: "auto_claim_energy", label: "领取体力" }, { key: "show_result", label: "显示结果" }, { key: "auto_restart", label: "自动重开" }], intParams: [{ key: "speed", label: "倍速", min: 1 }] },
  { id: "actElim", label: "花漾物语", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "auto_claim_energy", label: "领取体力" }], intParams: [{ key: "speed", label: "倍速", min: 1 }] },
  { id: "actSpool", label: "梳丝引线", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "auto_claim_energy", label: "领取体力" }], intParams: [{ key: "speed", label: "倍速", min: 1 }] },
  { id: "redPacket", label: "红包雨", status: SETTING_STATUS.syncOnly },
  { id: "recvLuck", label: "迎新接福", status: SETTING_STATUS.adapterMissing },
  { id: "yzCall", label: "为紫打 call", status: SETTING_STATUS.adapterMissing },
  { id: "moneyTree", label: "摇钱树", status: SETTING_STATUS.syncOnly },
  { id: "lanternFestival", label: "元宵灯谜", status: SETTING_STATUS.adapterMissing },
  { id: "actDuanWu", label: "龙舟竞渡", status: SETTING_STATUS.adapterMissing, boolParams: [{ key: "claim_box", label: "领取进度宝箱" }, { key: "open_box", label: "打开舟赛宝箱" }] },
  { id: "actDessert", label: "香卉甜糕", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "auto_claim_energy", label: "领取体力" }, { key: "use_items", label: "使用道具" }], intParams: [{ key: "speed", label: "倍速", min: 1 }] },
  { id: "actMerge2", label: "田园奇趣", status: SETTING_STATUS.syncOnly, boolParams: [{ key: "auto_claim_energy", label: "领取体力" }], intParams: [{ key: "speed", label: "倍速", min: 1 }] },
];

const EMPTY_ADD_FORM = {
  name: "",
  username: "",
  password: "",
  loginNow: true,
};

type AddAccountForm = typeof EMPTY_ADD_FORM;

type BlockedItem = {
  source: string;
  label: string;
  reasons: string[];
};

export default function HomePage() {
  return (
    <AppShell>
      <DashboardContent />
    </AppShell>
  );
}

function DashboardContent() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [statuses, setStatuses] = useState<Map<string, AccountStatus>>(new Map());
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [snapshot, setSnapshot] = useState<GetSnapshotResponse | null>(null);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [policyLoading, setPolicyLoading] = useState(false);
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [busyAction, setBusyAction] = useState("");
  const [error, setError] = useState("");
  const [policyMessage, setPolicyMessage] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [addForm, setAddForm] = useState<AddAccountForm>(EMPTY_ADD_FORM);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) ?? null,
    [accounts, selectedAccountId],
  );
  const selectedStatus = selectedAccountId ? statuses.get(selectedAccountId) : undefined;

  const refreshStatus = useCallback(async () => {
    const [accountRes, statusRes] = await Promise.all([
      accountClient.listAccounts({}),
      queryClient.getStatus({}),
    ]);
    const nextStatuses = new Map<string, AccountStatus>();
    for (const status of statusRes.accounts) {
      nextStatuses.set(status.accountId, status);
    }
    setAccounts(accountRes.accounts);
    setStatuses(nextStatuses);
  }, []);

  const refreshSnapshot = useCallback(async (accountId: string, showLoading = false) => {
    if (!accountId) {
      setSnapshot(null);
      return;
    }
    if (showLoading) {
      setSnapshotLoading(true);
    }
    try {
      setSnapshot(await queryClient.getSnapshot({ accountId }));
    } catch (err) {
      setSnapshot(null);
      setError(err instanceof Error ? err.message : "读取快照失败");
    } finally {
      setSnapshotLoading(false);
    }
  }, []);

  const refreshPolicy = useCallback(async (accountId: string) => {
    if (!accountId) {
      setPolicy(null);
      return;
    }
    setPolicyLoading(true);
    try {
      const res = await policyClient.getPolicy({ accountId });
      setPolicy(res.policy ?? create(PolicySchema));
      setPolicyMessage("");
    } catch (err) {
      setPolicy(null);
      setPolicyMessage(err instanceof Error ? err.message : "读取策略失败");
    } finally {
      setPolicyLoading(false);
    }
  }, []);

  const refreshWorkspace = useCallback(async () => {
    setError("");
    try {
      await refreshStatus();
    } catch (err) {
      setError(err instanceof Error ? err.message : "刷新失败");
    } finally {
      setLoading(false);
    }
  }, [refreshStatus]);

  useEffect(() => {
    void refreshWorkspace();
  }, [refreshWorkspace]);

  useEffect(() => {
    if (!selectedAccountId && accounts.length > 0) {
      setSelectedAccountId(accounts[0].id);
    }
  }, [accounts, selectedAccountId]);

  useEffect(() => {
    if (!selectedAccountId) {
      setSnapshot(null);
      setPolicy(null);
      setEvents([]);
      return;
    }
    void refreshSnapshot(selectedAccountId, true);
    void refreshPolicy(selectedAccountId);
  }, [refreshPolicy, refreshSnapshot, selectedAccountId]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void refreshStatus().catch(() => undefined);
      if (selectedAccountId) {
        void refreshSnapshot(selectedAccountId).catch(() => undefined);
      }
    }, STATUS_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refreshSnapshot, refreshStatus, selectedAccountId]);

  useEffect(() => {
    if (!selectedAccountId) return;
    const controller = new AbortController();
    let active = true;
    setEvents([]);

    async function readEvents() {
      try {
        for await (const event of queryClient.streamEvents(
          { accountId: selectedAccountId },
          { signal: controller.signal },
        )) {
          if (!active) return;
          setEvents((prev) => [event, ...prev].slice(0, EVENT_LIMIT));
          if (SNAPSHOT_REFRESH_EVENT_KINDS.has(event.kind)) {
            void refreshSnapshot(selectedAccountId).catch(() => undefined);
          }
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(err instanceof Error ? err.message : "事件流中断");
        }
      }
    }

    void readEvents();
    return () => {
      active = false;
      controller.abort();
    };
  }, [refreshSnapshot, selectedAccountId]);

  async function runAccountAction(action: "login" | "logout" | "start" | "stop" | "reload") {
    if (!selectedAccount) return;
    setBusyAction(action);
    setError("");
    try {
      if (action === "login") await accountClient.loginAccount({ id: selectedAccount.id });
      if (action === "logout") await accountClient.logoutAccount({ id: selectedAccount.id });
      if (action === "start") await automationClient.start({ accountId: selectedAccount.id });
      if (action === "stop") await automationClient.stop({ accountId: selectedAccount.id });
      if (action === "reload") await automationClient.reload({ accountId: selectedAccount.id });
      await refreshStatus();
      await refreshSnapshot(selectedAccount.id);
      await refreshPolicy(selectedAccount.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    } finally {
      setBusyAction("");
    }
  }

  async function createAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!addForm.name.trim() || !addForm.username.trim() || !addForm.password) return;
    setBusyAction("create");
    setError("");
    try {
      const res = await accountClient.createAccount({
        name: addForm.name.trim(),
        username: addForm.username.trim(),
        password: addForm.password,
        channel: Channel.IOS,
        loginNow: addForm.loginNow,
      });
      setAddOpen(false);
      setAddForm(EMPTY_ADD_FORM);
      await refreshStatus();
      if (res.account?.id) {
        setSelectedAccountId(res.account.id);
      }
      if (res.loginError) {
        setError(res.loginError);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "新增账号失败");
    } finally {
      setBusyAction("");
    }
  }

  async function deleteSelectedAccount() {
    if (!selectedAccount) return;
    const confirmed = window.confirm(`确认删除账号「${selectedAccount.name}」？此操作会移除本地账号、会话和策略。`);
    if (!confirmed) return;
    setBusyAction("delete");
    setError("");
    try {
      await accountClient.deleteAccount({ id: selectedAccount.id });
      const nextAccounts = accounts.filter((account) => account.id !== selectedAccount.id);
      setSelectedAccountId(nextAccounts[0]?.id ?? "");
      setSnapshot(null);
      setPolicy(null);
      await refreshStatus();
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除账号失败");
    } finally {
      setBusyAction("");
    }
  }

  async function savePolicy() {
    if (!selectedAccount || !policy) return;
    setSavingPolicy(true);
    setPolicyMessage("");
    try {
      const res = await policyClient.setPolicy({ accountId: selectedAccount.id, policy });
      setPolicy(res.policy ?? policy);
      setPolicyMessage("已保存");
      await refreshStatus();
      await refreshSnapshot(selectedAccount.id);
    } catch (err) {
      setPolicyMessage(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSavingPolicy(false);
    }
  }

  const blockedItems = useMemo(() => collectBlocked(snapshot, selectedStatus), [snapshot, selectedStatus]);

  return (
    <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
      <aside className="min-h-0 xl:overflow-hidden">
        <Card className="h-full min-h-[480px]">
          <CardHeader className="border-b border-border/70 pb-3">
            <div className="flex items-center justify-between gap-2">
              <CardTitle>账号</CardTitle>
              <div className="flex items-center gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => void refreshWorkspace()}
                  aria-label="刷新"
                  disabled={loading}
                >
                  <RefreshCw className={cn("size-4", loading && "animate-spin")} />
                </Button>
                <Button type="button" variant="outline" size="icon-sm" onClick={() => setAddOpen(true)} aria-label="新增账号">
                  <Plus className="size-4" />
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
            {accounts.length === 0 ? (
              <EmptyState title="暂无账号" detail="本地库未配置游戏账号。" />
            ) : (
              <div className="dark-scrollbar flex-1 space-y-2 overflow-y-auto pr-1">
                {accounts.map((account) => {
                  const status = statuses.get(account.id);
                  const selected = account.id === selectedAccountId;
                  return (
                    <button
                      key={account.id}
                      type="button"
                      className={cn(
                        "w-full rounded-md border p-3 text-left transition-colors",
                        selected
                          ? "border-primary/60 bg-primary/10"
                          : "border-border/70 bg-muted/20 hover:bg-muted/45",
                      )}
                      onClick={() => setSelectedAccountId(account.id)}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="truncate text-sm font-medium">{account.name}</div>
                          <div className="truncate text-xs text-muted-foreground">{account.username}</div>
                        </div>
                        <HealthBadge status={status} account={account} />
                      </div>
                      <div className="mt-3 grid grid-cols-3 gap-2 text-xs text-muted-foreground">
                        <MetricMini label="土地" value={status?.knownLands ?? 0} />
                        <MetricMini label="库存" value={status?.flowerStockTotal ?? 0} />
                        <MetricMini label="自动" value={status?.automationEnabled ? "开" : "关"} />
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </aside>

      <section className="dark-scrollbar min-h-0 overflow-y-auto pr-1">
        <div className="space-y-4 pb-2">
          {error && (
            <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <HeaderPanel
            account={selectedAccount}
            status={selectedStatus}
            snapshot={snapshot}
            snapshotLoading={snapshotLoading}
            busyAction={busyAction}
            onRefresh={() => selectedAccount && void refreshSnapshot(selectedAccount.id, true)}
            onAction={runAccountAction}
            onDelete={() => void deleteSelectedAccount()}
          />

          <div className="grid gap-4 2xl:grid-cols-[minmax(0,1.1fr)_minmax(360px,0.9fr)]">
            <div className="space-y-4">
              <SnapshotSummary snapshot={snapshot} status={selectedStatus} />
              <DemandPanel demands={snapshot?.demands ?? []} />
              <OperationPanel operations={snapshot?.plannedOperations ?? []} />
              <EventPanel events={events} />
            </div>
            <div className="space-y-4">
              <BlockedPanel items={blockedItems} />
              <FlowerArtPanel
                vases={snapshot?.vases ?? []}
                availability={snapshot?.flowerArtAvailability ?? []}
              />
              <PolicyPanel
                policy={policy}
                loading={policyLoading}
                saving={savingPolicy}
                message={policyMessage}
                onPolicyChange={setPolicy}
                onSave={() => void savePolicy()}
              />
            </div>
          </div>
        </div>
      </section>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增账号</DialogTitle>
            <DialogDescription>本地保存凭据，当前协议通道固定为 iOS。</DialogDescription>
          </DialogHeader>
          <form className="space-y-4" onSubmit={createAccount}>
            <Field label="名称">
              <Input
                value={addForm.name}
                onChange={(event) => setAddForm((prev) => ({ ...prev, name: event.target.value }))}
                autoComplete="off"
              />
            </Field>
            <Field label="账号">
              <Input
                value={addForm.username}
                onChange={(event) => setAddForm((prev) => ({ ...prev, username: event.target.value }))}
                autoComplete="username"
              />
            </Field>
            <Field label="密码">
              <Input
                type="password"
                value={addForm.password}
                onChange={(event) => setAddForm((prev) => ({ ...prev, password: event.target.value }))}
                autoComplete="current-password"
              />
            </Field>
            <ToggleRow
              label="立即登录"
              checked={addForm.loginNow}
              onChange={(checked) => setAddForm((prev) => ({ ...prev, loginNow: checked }))}
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
                取消
              </Button>
              <Button type="submit" disabled={busyAction === "create"}>
                <Plus className="size-4" />
                新增
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function HeaderPanel({
  account,
  status,
  snapshot,
  snapshotLoading,
  busyAction,
  onRefresh,
  onAction,
  onDelete,
}: {
  account: Account | null;
  status?: AccountStatus;
  snapshot: GetSnapshotResponse | null;
  snapshotLoading: boolean;
  busyAction: string;
  onRefresh: () => void;
  onAction: (action: "login" | "logout" | "start" | "stop" | "reload") => Promise<void>;
  onDelete: () => void;
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="truncate text-xl font-semibold">{account?.name ?? "未选择账号"}</h1>
            {account && <HealthBadge account={account} status={status} />}
            {status?.automationEnabled && <Badge variant="secondary">自动化已启用</Badge>}
          </div>
          <div className="mt-1 flex flex-wrap gap-3 text-sm text-muted-foreground">
            <span>角色：{snapshot?.roleName || "-"}</span>
            <span>等级：{snapshot?.level || 0}</span>
            <span>最近事件：{formatTimestamp(status?.lastEventAt)}</span>
            <span>健康：{status?.health || "-"}</span>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" variant="outline" onClick={onRefresh} disabled={!account || snapshotLoading}>
            <RefreshCw className={cn("size-4", snapshotLoading && "animate-spin")} />
            刷新
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => void onAction("reload")}
            disabled={!account || busyAction === "reload"}
          >
            <RefreshCw className="size-4" />
            重载
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => void onAction("login")}
            disabled={!account || busyAction === "login"}
          >
            <LogIn className="size-4" />
            登录
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => void onAction("logout")}
            disabled={!account || busyAction === "logout"}
          >
            <LogOut className="size-4" />
            断开
          </Button>
          <Button
            type="button"
            onClick={() => void onAction("start")}
            disabled={!account || busyAction === "start"}
          >
            <Play className="size-4" />
            启动
          </Button>
          <Button
            type="button"
            variant="secondary"
            onClick={() => void onAction("stop")}
            disabled={!account || busyAction === "stop"}
          >
            <Square className="size-4" />
            停止
          </Button>
          <Button type="button" variant="destructive" onClick={onDelete} disabled={!account || busyAction === "delete"}>
            <Trash2 className="size-4" />
            删除
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function SnapshotSummary({ snapshot, status }: { snapshot: GetSnapshotResponse | null; status?: AccountStatus }) {
  const recommendationStats = useMemo(() => {
    const stats = new Map<string, number>();
    for (const land of snapshot?.lands ?? []) {
      stats.set(land.recommendation || "unknown", (stats.get(land.recommendation || "unknown") ?? 0) + 1);
    }
    return [...stats.entries()].sort((a, b) => b[1] - a[1]);
  }, [snapshot]);

  const inventoryTop = useMemo(() => {
    if (!snapshot) return [];
    return Object.entries(snapshot.inventory)
      .map(([id, count]) => ({ id: Number(id), count }))
      .filter((item) => item.count > 0)
      .sort((a, b) => b.count - a.count)
      .slice(0, 10);
  }, [snapshot]);

  return (
    <div className="grid gap-4 lg:grid-cols-4">
      <MetricCard icon={<Coins />} label="金币" value={snapshot?.gold ?? 0} />
      <MetricCard icon={<Gem />} label="钻石" value={(snapshot?.diamondsFree ?? 0) + (snapshot?.diamondsPaid ?? 0)} />
      <MetricCard icon={<Waves />} label="水滴" value={`${snapshot?.waterDrops ?? 0}/${snapshot?.waterDropsTotal ?? 0}`} />
      <MetricCard icon={<Sprout />} label="土地" value={status?.knownLands ?? snapshot?.lands.length ?? 0} />
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>土地状态</CardTitle>
        </CardHeader>
        <CardContent>
          {recommendationStats.length === 0 ? (
            <EmptyState title="暂无土地快照" />
          ) : (
            <div className="flex flex-wrap gap-2">
              {recommendationStats.map(([key, count]) => (
                <Badge key={key} variant="outline">
                  {recommendationLabel(key)} · {count}
                </Badge>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>库存 Top</CardTitle>
        </CardHeader>
        <CardContent>
          {inventoryTop.length === 0 ? (
            <EmptyState title="暂无库存数据" />
          ) : (
            <div className="grid gap-2 sm:grid-cols-2">
              {inventoryTop.map((item) => (
                <div key={item.id} className="flex items-center justify-between rounded-md border border-border/70 px-3 py-2">
                  <span className="truncate text-sm">{itemName(item.id)}</span>
                  <span className="font-mono text-sm">{item.count}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DemandPanel({ demands }: { demands: DemandView[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>需求缺口</CardTitle>
          <Badge variant="secondary">{demands.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {demands.length === 0 ? (
          <EmptyState title="当前无需求" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>来源</TableHead>
                <TableHead>物品</TableHead>
                <TableHead>需求</TableHead>
                <TableHead>库存</TableHead>
                <TableHead>分配</TableHead>
                <TableHead>可用</TableHead>
                <TableHead>缺口</TableHead>
                <TableHead>优先级</TableHead>
                <TableHead>状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {demands.slice(0, 16).map((demand) => (
                <TableRow key={demand.id}>
                  <TableCell>
                    <div className="font-medium">{goalLabel(demand.goalId)}</div>
                    <div className="text-xs text-muted-foreground">{demand.label || demand.entityId}</div>
                  </TableCell>
                  <TableCell>{demand.itemName || itemName(demand.itemId)}</TableCell>
                  <TableCell>{demand.required}</TableCell>
                  <TableCell>{demand.owned}</TableCell>
                  <TableCell>{demand.allocated}</TableCell>
                  <TableCell>{demand.available}</TableCell>
                  <TableCell className={cn(demand.missing > 0 && "text-destructive")}>{demand.missing}</TableCell>
                  <TableCell>{demand.priority}</TableCell>
                  <TableCell>
                    <ReasonList reasons={demand.blockedReasons} fallback="可执行" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function OperationPanel({ operations }: { operations: PlannedOperation[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>执行队列</CardTitle>
          <Badge variant="secondary">{operations.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {operations.length === 0 ? (
          <EmptyState title="当前无计划操作" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>状态</TableHead>
                <TableHead>操作</TableHead>
                <TableHead>目标</TableHead>
                <TableHead>成本</TableHead>
                <TableHead>原因</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {operations.slice(0, 18).map((operation, index) => (
                <TableRow key={operation.operationId || `${operation.rpc}-${index}`}>
                  <TableCell>
                    <OperationStatusBadge operation={operation} />
                  </TableCell>
                  <TableCell>
                    <div className="font-medium">{operation.label || `${operation.domain}.${operation.action}`}</div>
                    <div className="text-xs text-muted-foreground">
                      {goalLabel(operation.goalId)} · {operation.rpc || "local"}
                    </div>
                  </TableCell>
                  <TableCell>
                    <OperationTarget operation={operation} />
                  </TableCell>
                  <TableCell>
                    <CostView operation={operation} />
                  </TableCell>
                  <TableCell className="max-w-[280px] whitespace-normal text-muted-foreground">
                    {operation.reason || <ReasonList reasons={operation.blockedReasons} fallback="-" />}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function BlockedPanel({ items }: { items: BlockedItem[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>阻塞原因</CardTitle>
          <Badge variant={items.length ? "destructive" : "secondary"}>{items.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <EmptyState title="无阻塞" />
        ) : (
          <div className="space-y-2">
            {items.slice(0, 12).map((item, index) => (
              <div key={`${item.source}-${index}`} className="rounded-md border border-destructive/30 bg-destructive/10 p-3">
                <div className="flex items-center gap-2 text-sm font-medium text-destructive">
                  <AlertTriangle className="size-4" />
                  {item.label}
                </div>
                <div className="mt-1 text-xs text-muted-foreground">{item.source}</div>
                <div className="mt-2 flex flex-wrap gap-1">
                  {item.reasons.map((reason) => (
                    <Badge key={reason} variant="outline">
                      {reason}
                    </Badge>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function FlowerArtPanel({
  vases,
  availability,
}: {
  vases: VaseView[];
  availability: FlowerArtAvailabilityView[];
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>花艺能力</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <div className="mb-2 text-xs font-medium text-muted-foreground">已解锁花瓶</div>
          {vases.length === 0 ? (
            <EmptyState title="未观察到花瓶状态" />
          ) : (
            <div className="flex flex-wrap gap-2">
              {vases.map((vase) => (
                <Badge key={vase.vaseId} variant="outline">
                  花瓶 {vase.vaseId}
                </Badge>
              ))}
            </div>
          )}
        </div>
        <div>
          <div className="mb-2 text-xs font-medium text-muted-foreground">可制作性</div>
          {availability.length === 0 ? (
            <EmptyState title="暂无花艺配方视图" />
          ) : (
            <div className="space-y-2">
              {availability.slice(0, 8).map((art) => (
                <div key={art.artId} className="rounded-md border border-border/70 p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{art.artName || itemName(art.artId)}</div>
                      <div className="text-xs text-muted-foreground">
                        花瓶 {art.vaseId} · 等级 {art.level} · 价值 {art.saleValue}
                      </div>
                    </div>
                    <Badge variant={art.craftable ? "secondary" : "outline"}>
                      {art.craftable ? "可制作" : "受限"}
                    </Badge>
                  </div>
                  {art.blockedReasons.length > 0 && (
                    <div className="mt-2">
                      <ReasonList reasons={art.blockedReasons} fallback="-" />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function PolicyPanel({
  policy,
  loading,
  saving,
  message,
  onPolicyChange,
  onSave,
}: {
  policy: Policy | null;
  loading: boolean;
  saving: boolean;
  message: string;
  onPolicyChange: (policy: Policy | null) => void;
  onSave: () => void;
}) {
  const [activeTab, setActiveTab] = useState<PolicyTabId>("basic");
  const plant = policy?.plant;
  const flower = plant?.flower;
  const cultivate = plant?.cultivate;
  const friendSteal = plant?.friendSteal;
  const elves = plant?.elves;
  const market = plant?.market;
  const basic = policy?.basic;
  const reputation = basic?.reputation;
  const task = basic?.task;
  const benefit = basic?.benefit;
  const sign = basic?.sign;
  const pearl = basic?.pearl;
  const shop = basic?.shop;
  const cultivateShop = shop?.cultivateShop;
  const vipShop = shop?.vipShop;
  const feedCat = basic?.feedCat;
  const order = policy?.order;
  const customer = order?.customer;
  const resident = order?.resident;
  const palace = order?.palace;
  const team = order?.team;
  const flowerArt = order?.flowerArt;
  const union = policy?.union;
  const unionBuild = union?.build;
  const unionFlower = union?.flower;
  const unionRace = union?.race;
  const unionLand = union?.land;
  const activity = policy?.activity;

  const updatePolicy = (patch: Partial<Policy>) => {
    if (!policy) return;
    onPolicyChange({ ...policy, ...patch });
  };
  const updatePlant = (patch: Partial<PlantPolicy>) => {
    if (!policy) return;
    const current = policy.plant ?? create(PlantPolicySchema);
    onPolicyChange({ ...policy, plant: { ...current, ...patch } });
  };
  const updateBasic = (patch: Partial<BasicPolicy>) => {
    if (!policy) return;
    const current = policy.basic ?? create(BasicPolicySchema);
    onPolicyChange({ ...policy, basic: { ...current, ...patch } });
  };
  const updateReputation = (patch: Partial<ReputationPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.reputation ?? create(ReputationPolicySchema);
    updateBasic({ reputation: { ...current, ...patch } });
  };
  const updateBasicTask = (patch: Partial<BasicTaskPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.task ?? create(BasicTaskPolicySchema);
    updateBasic({ task: { ...current, ...patch } });
  };
  const updateBenefit = (patch: Partial<BenefitPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.benefit ?? create(BenefitPolicySchema);
    updateBasic({ benefit: { ...current, ...patch } });
  };
  const updateSign = (patch: Partial<SignPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.sign ?? create(SignPolicySchema);
    updateBasic({ sign: { ...current, ...patch } });
  };
  const updatePearl = (patch: Partial<PearlPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.pearl ?? create(PearlPolicySchema);
    updateBasic({ pearl: { ...current, ...patch } });
  };
  const updateShop = (patch: Partial<ShopPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.shop ?? create(ShopPolicySchema);
    updateBasic({ shop: { ...current, ...patch } });
  };
  const updateCultivateShop = (patch: Partial<ShopBuyPolicy>) => {
    if (!policy) return;
    const currentShop = (policy.basic ?? create(BasicPolicySchema)).shop ?? create(ShopPolicySchema);
    const current = currentShop.cultivateShop ?? create(ShopBuyPolicySchema);
    updateShop({ cultivateShop: { ...current, ...patch } });
  };
  const updateVipShop = (patch: Partial<VipShopPolicy>) => {
    if (!policy) return;
    const currentShop = (policy.basic ?? create(BasicPolicySchema)).shop ?? create(ShopPolicySchema);
    const current = currentShop.vipShop ?? create(VipShopPolicySchema);
    updateShop({ vipShop: { ...current, ...patch } });
  };
  const updateFeedCat = (patch: Partial<FeedCatPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.feedCat ?? create(FeedCatPolicySchema);
    updateBasic({ feedCat: { ...current, ...patch } });
  };
  const updateFlower = (patch: Partial<FlowerPlantPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.flower ?? create(FlowerPlantPolicySchema);
    updatePlant({ flower: { ...current, ...patch } });
  };
  const updateCultivate = (patch: Partial<CultivatePolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.cultivate ?? create(CultivatePolicySchema);
    updatePlant({ cultivate: { ...current, ...patch } });
  };
  const updateFriendSteal = (patch: Partial<FriendStealPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.friendSteal ?? create(FriendStealPolicySchema);
    updatePlant({ friendSteal: { ...current, ...patch } });
  };
  const updateElves = (patch: Partial<FlowerElvesPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.elves ?? create(FlowerElvesPolicySchema);
    updatePlant({ elves: { ...current, ...patch } });
  };
  const updateMarket = (patch: Partial<FlowerMarketPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.market ?? create(FlowerMarketPolicySchema);
    updatePlant({ market: { ...current, ...patch } });
  };
  const updateOrder = (patch: Partial<OrderPolicy>) => {
    if (!policy) return;
    const current = policy.order ?? create(OrderPolicySchema);
    onPolicyChange({ ...policy, order: { ...current, ...patch } });
  };
  const updateCustomer = (patch: Partial<CustomerOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.customer ?? create(CustomerOrderPolicySchema);
    updateOrder({ customer: { ...current, ...patch } });
  };
  const updateResident = (patch: Partial<ResidentOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.resident ?? create(ResidentOrderPolicySchema);
    updateOrder({ resident: { ...current, ...patch } });
  };
  const updatePalace = (patch: Partial<PalaceOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.palace ?? create(PalaceOrderPolicySchema);
    updateOrder({ palace: { ...current, ...patch } });
  };
  const updateTeam = (patch: Partial<TeamOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.team ?? create(TeamOrderPolicySchema);
    updateOrder({ team: { ...current, ...patch } });
  };
  const updateFlowerArt = (patch: Partial<FlowerArtPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.flowerArt ?? create(FlowerArtPolicySchema);
    updateOrder({ flowerArt: { ...current, ...patch } });
  };
  const updateUnion = (patch: Partial<UnionPolicy>) => {
    if (!policy) return;
    const current = policy.union ?? create(UnionPolicySchema);
    onPolicyChange({ ...policy, union: { ...current, ...patch } });
  };
  const updateUnionBuild = (patch: Partial<UnionBuildPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.build ?? create(UnionBuildPolicySchema);
    updateUnion({ build: { ...current, ...patch } });
  };
  const updateUnionFlower = (patch: Partial<UnionFlowerPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.flower ?? create(UnionFlowerPolicySchema);
    updateUnion({ flower: { ...current, ...patch } });
  };
  const updateUnionRace = (patch: Partial<UnionRacePolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.race ?? create(UnionRacePolicySchema);
    updateUnion({ race: { ...current, ...patch } });
  };
  const updateUnionLand = (patch: Partial<UnionLandPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.land ?? create(UnionLandPolicySchema);
    updateUnion({ land: { ...current, ...patch } });
  };
  const updateActivity = (patch: Partial<ActivityPolicy>) => {
    if (!policy) return;
    const current = policy.activity ?? create(ActivityPolicySchema);
    onPolicyChange({ ...policy, activity: { ...current, ...patch } });
  };
  const updateActivityModule = (moduleID: string, patch: Partial<ActivityModulePolicy>) => {
    if (!policy) return;
    const currentActivity = policy.activity ?? create(ActivityPolicySchema);
    const current = currentActivity.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivity({
      modules: {
        ...currentActivity.modules,
        [moduleID]: { ...current, ...patch },
      },
    });
  };
  const updateActivityBoolParam = (moduleID: string, key: string, value: boolean) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { boolParams: { ...current.boolParams, [key]: value } });
  };
  const updateActivityIntParam = (moduleID: string, key: string, value: bigint) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { intParams: { ...current.intParams, [key]: value } });
  };
  const updateActivityStringParam = (moduleID: string, key: string, value: string) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { stringParams: { ...current.stringParams, [key]: value } });
  };
  const updateActivityIntListParam = (moduleID: string, key: string, values: number[]) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, {
      intListParams: {
        ...current.intListParams,
        [key]: create(IntListSchema, { values }),
      },
    });
  };
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>策略</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState title="策略加载中" />
        </CardContent>
      </Card>
    );
  }

  if (!policy) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>策略</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState title="未加载策略" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>策略</CardTitle>
          <Button type="button" size="sm" onClick={onSave} disabled={saving}>
            <Save className="size-4" />
            保存
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {message && <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm">{message}</div>}

        <section className="space-y-3">
          <SectionTitle icon={<ShieldCheck />}>总开关</SectionTitle>
          <div className="grid gap-2 sm:grid-cols-2">
            <ToggleRow label="自动化" checked={policy.automationEnabled} onChange={(checked) => updatePolicy({ automationEnabled: checked })} />
            <NumberRow label="决策间隔" value={policy.decisionIntervalSeconds || 4} min={1} onChange={(value) => updatePolicy({ decisionIntervalSeconds: value })} />
          </div>
        </section>

        <div className="flex gap-1 overflow-x-auto rounded-md border border-border/70 bg-muted/20 p-1">
          {POLICY_TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "flex min-h-9 shrink-0 items-center gap-2 rounded px-3 text-sm font-medium transition-colors [&_svg]:size-4",
                activeTab === tab.id ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>

        {activeTab === "basic" && (
          <div className="space-y-4">
            <PolicyGroup title="基础配置" icon={<ShieldCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="礼仪分监控" checked={reputation?.enabled ?? false} onChange={(checked) => updateReputation({ enabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <NumberRow label="礼仪分阈值" value={reputation?.threshold || 80} min={0} onChange={(value) => updateReputation({ threshold: value })} />
                <ToggleRow label="道具日志" checked={basic?.itemLogEnabled ?? false} onChange={(checked) => updateBasic({ itemLogEnabled: checked })} />
                <NumberRow label="重连间隔秒" value={basic?.reconnectIntervalSeconds || 300} min={1} onChange={(value) => updateBasic({ reconnectIntervalSeconds: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="任务与剧情" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="主线任务" checked={task?.mainEnabled ?? false} onChange={(checked) => updateBasicTask({ mainEnabled: checked })} />
                <ToggleRow label="每日任务" checked={task?.dailyEnabled ?? false} onChange={(checked) => updateBasicTask({ dailyEnabled: checked })} />
                <ToggleRow label="每周任务" checked={task?.weeklyEnabled ?? false} onChange={(checked) => updateBasicTask({ weeklyEnabled: checked })} />
                <ToggleRow label="主线剧情" checked={task?.storyEnabled ?? false} onChange={(checked) => updateBasicTask({ storyEnabled: checked })} />
                <ToggleRow label="花坊悬赏" checked={task?.achievementEnabled ?? false} onChange={(checked) => updateBasicTask({ achievementEnabled: checked })} />
                <ToggleRow label="动物/地图事件" checked={basic?.randomEventEnabled ?? false} onChange={(checked) => updateBasic({ randomEventEnabled: checked })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="邮件、福利、祈愿" icon={<BadgeCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="邮件" checked={basic?.mailEnabled ?? false} onChange={(checked) => updateBasic({ mailEnabled: checked })} />
                <ToggleRow label="水车水滴" checked={basic?.waterwheelEnabled ?? false} onChange={(checked) => updateBasic({ waterwheelEnabled: checked })} />
                <ToggleRow label="限时水滴" checked={basic?.freeWaterEnabled ?? false} onChange={(checked) => updateBasic({ freeWaterEnabled: checked })} />
                <NumberRow label="水滴领取阈值" value={basic?.waterClaimThreshold || 0} min={0} onChange={(value) => updateBasic({ waterClaimThreshold: value })} />
                <ToggleRow label="双倍金币" checked={benefit?.doubleCoinEnabled ?? false} onChange={(checked) => updateBenefit({ doubleCoinEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />
                <ToggleRow label="福利宝箱" checked={benefit?.boxEnabled ?? false} onChange={(checked) => updateBenefit({ boxEnabled: checked })} />
                <ToggleRow label="分享奖励" checked={benefit?.shareRewardEnabled ?? false} onChange={(checked) => updateBenefit({ shareRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="防骗宝箱" checked={benefit?.antiScamBoxEnabled ?? false} onChange={(checked) => updateBenefit({ antiScamBoxEnabled: checked })} />
                <ToggleRow label="每日祈愿" checked={sign?.dailyEnabled ?? false} onChange={(checked) => updateSign({ dailyEnabled: checked })} />
                <ToggleRow label="自动补签" checked={sign?.patchEnabled ?? false} onChange={(checked) => updateSign({ patchEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="成长之路" checked={basic?.roadGrowRewardEnabled ?? false} onChange={(checked) => updateBasic({ roadGrowRewardEnabled: checked })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="珍珠" icon={<Gem />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="免费珍珠" checked={pearl?.freeEnabled ?? false} onChange={(checked) => updatePearl({ freeEnabled: checked })} />
                <ToggleRow label="雇佣劳工" checked={pearl?.autoHireEnabled ?? false} onChange={(checked) => updatePearl({ autoHireEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <NumberRow label="雇佣等级上限" value={pearl?.maxHireLevel || 0} min={0} onChange={(value) => updatePearl({ maxHireLevel: value })} />
                <NumberRow label="雇佣券上限" value={pearl?.maxHireTicketUsage || 0} min={0} onChange={(value) => updatePearl({ maxHireTicketUsage: value })} />
                <ToggleRow label="自动开珍珠" checked={pearl?.drawEnabled ?? false} onChange={(checked) => updatePearl({ drawEnabled: checked })} />
                <ToggleRow label="开启防身" checked={pearl?.protectEnabled ?? false} onChange={(checked) => updatePearl({ protectEnabled: checked })} />
                <ToggleRow label="买雇佣书" checked={pearl?.autoBuyHireTicket ?? false} onChange={(checked) => updatePearl({ autoBuyHireTicket: checked })} status={SETTING_STATUS.adapterMissing} />
                <BigIntNumberRow label="元宝上限" value={pearl?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updatePearl({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="商城与喂猫" icon={<ShoppingBag />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="视频礼包" checked={shop?.videoFreeGiftEnabled ?? false} onChange={(checked) => updateShop({ videoFreeGiftEnabled: checked })} />
                <ToggleRow label="材料商店" checked={cultivateShop?.autoBuy ?? false} onChange={(checked) => updateCultivateShop({ autoBuy: checked })} />
                <BigIntNumberRow label="材料金币上限" value={cultivateShop?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateCultivateShop({ maxSpendGold: value })} />
                <IntListRow label="材料商品 ID" value={cultivateShop?.itemIds ?? []} onChange={(value) => updateCultivateShop({ itemIds: value })} />
                <ToggleRow label="VIP 商店" checked={vipShop?.autoBuy ?? false} onChange={(checked) => updateVipShop({ autoBuy: checked })} status={SETTING_STATUS.adapterMissing} />
                <BigIntNumberRow label="VIP 元宝上限" value={vipShop?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendDiamond: value })} />
                <BigIntNumberRow label="VIP 花坊币上限" value={vipShop?.maxSpendFloralCoin ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendFloralCoin: value })} />
                <IntListRow label="VIP 商品 ID" value={vipShop?.itemIds ?? []} onChange={(value) => updateVipShop({ itemIds: value })} />
                <ToggleRow label="喂猫模块" checked={feedCat?.enabled ?? false} onChange={(checked) => updateFeedCat({ enabled: checked })} />
                <ToggleRow label="自动召回" checked={feedCat?.autoRecall ?? false} onChange={(checked) => updateFeedCat({ autoRecall: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="购买猫粮" checked={feedCat?.autoBuyFood ?? false} onChange={(checked) => updateFeedCat({ autoBuyFood: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="自动喂猫" checked={feedCat?.autoFeed ?? false} onChange={(checked) => updateFeedCat({ autoFeed: checked })} />
                <ToggleRow label="自动撸猫" checked={feedCat?.autoStroke ?? false} onChange={(checked) => updateFeedCat({ autoStroke: checked })} />
                <BigIntNumberRow label="猫金币上限" value={feedCat?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateFeedCat({ maxSpendGold: value })} />
                <BigIntNumberRow label="猫元宝上限" value={feedCat?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateFeedCat({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "plant" && (
          <div className="space-y-4">
            <PolicyGroup title="培育配置" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动培育" checked={cultivate?.enabled ?? false} onChange={(checked) => updateCultivate({ enabled: checked })} />
                <ToggleRow label="视频加速培育" checked={cultivate?.videoSpeedUpEnabled ?? false} onChange={(checked) => updateCultivate({ videoSpeedUpEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />
                <ToggleRow label="鲜花升级" checked={cultivate?.upgradeEnabled ?? false} onChange={(checked) => updateCultivate({ upgradeEnabled: checked })} />
                <NumberRow label="目标等级" value={cultivate?.targetLevel || 20} min={1} onChange={(value) => updateCultivate({ targetLevel: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="土地与种植" icon={<Sprout />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="解锁土地" checked={flower?.autoUnlockLand ?? false} onChange={(checked) => updateFlower({ autoUnlockLand: checked })} />
                <ToggleRow label="自动收获" checked={flower?.harvestEnabled ?? false} onChange={(checked) => updateFlower({ harvestEnabled: checked })} />
                <ToggleRow label="一键收获" checked={flower?.harvestPreferOneKey ?? false} onChange={(checked) => updateFlower({ harvestPreferOneKey: checked })} />
                <ToggleRow label="自动种植" checked={flower?.plantEnabled ?? false} onChange={(checked) => updateFlower({ plantEnabled: checked })} />
                <ToggleRow label="自动浇水" checked={flower?.waterEnabled ?? false} onChange={(checked) => updateFlower({ waterEnabled: checked })} />
                <ToggleRow label="视频加速" checked={flower?.videoSpeedUpEnabled ?? false} onChange={(checked) => updateFlower({ videoSpeedUpEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />
                <ToggleRow label="使用加速券" checked={flower?.useSpeedUpTicket ?? false} onChange={(checked) => updateFlower({ useSpeedUpTicket: checked })} />
                <NumberRow label="加速券上限" value={flower?.speedUpTicketMax || 0} min={0} onChange={(value) => updateFlower({ speedUpTicketMax: value })} />
                <NumberRow label="保留水滴" value={flower?.minWaterDrops || 0} min={0} onChange={(value) => updateFlower({ minWaterDrops: value })} />
                <NumberRow label="浇水批量" value={flower?.waterMaxBatch || 8} min={1} onChange={(value) => updateFlower({ waterMaxBatch: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="种植策略" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="任务优先" checked={flower?.taskPriorityEnabled ?? false} onChange={(checked) => updateFlower({ taskPriorityEnabled: checked })} />
                <ToggleRow label="任务日志" checked={flower?.taskLogEnabled ?? false} onChange={(checked) => updateFlower({ taskLogEnabled: checked })} />
                <SegmentedRow label="种植模式" value={flower?.plantingMode || PlantingMode.COUNT} options={PLANTING_MODE_OPTIONS} onChange={(value) => updateFlower({ plantingMode: value })} />
                <QualityRow label="选择品质" value={flower?.allowedQualities ?? []} onChange={(value) => updateFlower({ allowedQualities: value })} />
                <NumberRow label="选择数量" value={flower?.flowerKindCount || 4} min={1} onChange={(value) => updateFlower({ flowerKindCount: value })} />
                <IntListRow label="指定花朵" value={flower?.specifiedFlowerIds ?? []} onChange={(value) => updateFlower({ specifiedFlowerIds: value })} />
                <IntListRow label="排除花朵" value={flower?.blockedFlowerIds ?? []} onChange={(value) => updateFlower({ blockedFlowerIds: value })} />
                <NumberRow label="最低花朵等级" value={flower?.minFlowerLevel || 0} min={0} onChange={(value) => updateFlower({ minFlowerLevel: value })} />
                <NumberRow label="每轮种植" value={flower?.plantMaxBatch || 8} min={1} onChange={(value) => updateFlower({ plantMaxBatch: value })} />
                <NumberRow label="单花上限" value={flower?.maxPerFlowerPerCycle || 4} min={1} onChange={(value) => updateFlower({ maxPerFlowerPerCycle: value })} />
                <NumberRow label="补种水位" value={flower?.fallbackStockFloor || 0} min={0} onChange={(value) => updateFlower({ fallbackStockFloor: value })} />
              </div>
              <div className="mt-3 grid gap-2">
                {GOAL_OPTIONS.map((goal) => (
                  <NumberRow
                    key={goal.id}
                    label={`${goal.label}优先级`}
                    value={flower?.goalPriority?.[goal.id] ?? goal.defaultPriority}
                    min={1}
                    onChange={(value) => updateFlower({ goalPriority: { ...(flower?.goalPriority ?? {}), [goal.id]: value } })}
                  />
                ))}
              </div>
            </PolicyGroup>

            <PolicyGroup title="好友偷花" icon={<HandCoins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动偷花" checked={friendSteal?.enabled ?? false} onChange={(checked) => updateFriendSteal({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="偷取花灵" checked={friendSteal?.stealElves ?? false} onChange={(checked) => updateFriendSteal({ stealElves: checked })} />
                <SegmentedRow label="偷花模式" value={friendSteal?.mode || SelectionMode.ALL} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateFriendSteal({ mode: value })} />
                <QualityRow label="指定品质" value={friendSteal?.qualities ?? []} onChange={(value) => updateFriendSteal({ qualities: value })} />
                <IntListRow label="指定花朵" value={friendSteal?.flowerIds ?? []} onChange={(value) => updateFriendSteal({ flowerIds: value })} />
                <IntListRow label="排除花朵" value={friendSteal?.excludeFlowerIds ?? []} onChange={(value) => updateFriendSteal({ excludeFlowerIds: value })} />
                <ToggleRow label="购买偷取次数" checked={friendSteal?.autoBuyTimes ?? false} onChange={(checked) => updateFriendSteal({ autoBuyTimes: checked })} status={SETTING_STATUS.adapterMissing} />
                <NumberRow label="购买次数" value={friendSteal?.buyCount || 0} min={0} onChange={(value) => updateFriendSteal({ buyCount: value })} />
                <BigIntNumberRow label="元宝上限" value={friendSteal?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateFriendSteal({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="花灵与密令" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动种花灵" checked={elves?.enabled ?? false} onChange={(checked) => updateElves({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                <IntListRow label="指定花灵" value={elves?.selectedIds ?? []} onChange={(value) => updateElves({ selectedIds: value })} />
                <ToggleRow label="申请协助" checked={elves?.requestAid ?? false} onChange={(checked) => updateElves({ requestAid: checked })} />
                <ToggleRow label="领取协助" checked={elves?.receiveAid ?? false} onChange={(checked) => updateElves({ receiveAid: checked })} />
                <ToggleRow label="协助好友" checked={elves?.helpFriend ?? false} onChange={(checked) => updateElves({ helpFriend: checked })} />
                <ToggleRow label="派遣花灵" checked={elves?.dispatch ?? false} onChange={(checked) => updateElves({ dispatch: checked })} />
                <ToggleRow label="仅双倍花灵" checked={elves?.dispatchOnlyDoubleBuff ?? false} onChange={(checked) => updateElves({ dispatchOnlyDoubleBuff: checked })} />
                <ToggleRow label="加速派遣" checked={elves?.speedUpDispatch ?? false} onChange={(checked) => updateElves({ speedUpDispatch: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="派遣奖励" checked={elves?.receiveDispatchReward ?? false} onChange={(checked) => updateElves({ receiveDispatchReward: checked })} />
                <ToggleRow label="花灵密令等级" checked={elves?.passRewardEnabled ?? false} onChange={(checked) => updateElves({ passRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="花灵密令任务" checked={elves?.passTaskRewardEnabled ?? false} onChange={(checked) => updateElves({ passTaskRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="花之密令等级" checked={elves?.flowerPassRewardEnabled ?? false} onChange={(checked) => updateElves({ flowerPassRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="花之密令任务" checked={elves?.flowerPassTaskRewardEnabled ?? false} onChange={(checked) => updateElves({ flowerPassTaskRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <BigIntNumberRow label="元宝上限" value={elves?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateElves({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="花贸市场" icon={<ShoppingBag />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="解锁货架" checked={market?.autoUnlockShelf ?? false} onChange={(checked) => updateMarket({ autoUnlockShelf: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="自动上架" checked={market?.putEnabled ?? false} onChange={(checked) => updateMarket({ putEnabled: checked })} status={SETTING_STATUS.syncOnly} />
                <SegmentedRow label="上架策略" value={market?.putMode || MarketPutMode.INVENTORY} options={MARKET_PUT_MODE_OPTIONS} onChange={(value) => updateMarket({ putMode: value })} />
                <IntListRow label="上架花朵" value={market?.specificFlowerIds ?? []} onChange={(value) => updateMarket({ specificFlowerIds: value })} />
                <NumberRow label="上架价格" value={market?.priceIndex ?? 2} min={0} onChange={(value) => updateMarket({ priceIndex: value })} />
                <NumberRow label="上架数量" value={market?.maxSell || 25} min={1} onChange={(value) => updateMarket({ maxSell: value })} />
                <TextRow label="上架密码" value={market?.putFlowerPassword ?? ""} onChange={(value) => updateMarket({ putFlowerPassword: value })} />
                <ToggleRow label="好友摊位扫货" checked={market?.autoBuyFromFriend ?? false} onChange={(checked) => updateMarket({ autoBuyFromFriend: checked })} status={SETTING_STATUS.syncOnly} />
                <SegmentedRow label="扫货策略" value={market?.buyMode || MarketBuyMode.ALL} options={MARKET_BUY_MODE_OPTIONS} onChange={(value) => updateMarket({ buyMode: value })} />
                <IntListRow label="扫货花朵" value={market?.buySpecificFlowerIds ?? []} onChange={(value) => updateMarket({ buySpecificFlowerIds: value })} />
                <QualityRow label="扫货品质" value={market?.buyQualities ?? []} onChange={(value) => updateMarket({ buyQualities: value })} />
                <NumberRow label="最小上架秒" value={market?.minPutTimeSeconds || 0} min={0} onChange={(value) => updateMarket({ minPutTimeSeconds: value })} />
                <BigIntNumberRow label="元宝上限" value={market?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateMarket({ maxSpendDiamond: value })} />
                <BigIntNumberRow label="金币上限" value={market?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateMarket({ maxSpendGold: value })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "order" && (
          <div className="space-y-4">
            <PolicyGroup title="居民订单" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="普通居民订单" checked={resident?.normalEnabled ?? false} onChange={(checked) => updateResident({ normalEnabled: checked })} />
                <NumberRow label="普通订单上限" value={resident?.normalDailyLimit || 1200} min={0} onChange={(value) => updateResident({ normalDailyLimit: value })} />
                <ToggleRow label="绸缎订单" checked={resident?.satinEnabled ?? false} onChange={(checked) => updateResident({ satinEnabled: checked })} />
                <NumberRow label="绸缎订单上限" value={resident?.satinDailyLimit || 120} min={0} onChange={(value) => updateResident({ satinDailyLimit: value })} />
                <ToggleRow label="建材订单" checked={resident?.decorateEnabled ?? false} onChange={(checked) => updateResident({ decorateEnabled: checked })} />
                <NumberRow label="建材订单上限" value={resident?.decorateDailyLimit || 120} min={0} onChange={(value) => updateResident({ decorateDailyLimit: value })} />
                <ToggleRow label="居民领奖" checked={resident?.rewardEnabled ?? false} onChange={(checked) => updateResident({ rewardEnabled: checked })} />
                <QualityRow label="品质限定" value={resident?.qualities ?? []} onChange={(value) => updateResident({ qualities: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="顾客、宫廷、组团" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="顾客订单" checked={customer?.enabled ?? false} onChange={(checked) => updateCustomer({ enabled: checked })} />
                <ToggleRow label="自动拒绝" checked={customer?.rejectEnabled ?? false} onChange={(checked) => updateCustomer({ rejectEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="自动制作花艺" checked={customer?.craftEnabled ?? false} onChange={(checked) => updateCustomer({ craftEnabled: checked })} />
                <ToggleRow label="宫廷订单" checked={palace?.enabled ?? false} onChange={(checked) => updatePalace({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                <QualityRow label="宫廷品质" value={palace?.qualities ?? []} onChange={(value) => updatePalace({ qualities: value })} />
                <ToggleRow label="组团订单" checked={team?.enabled ?? false} onChange={(checked) => updateTeam({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                <ToggleRow label="再来一单" checked={team?.oneMoreEnabled ?? false} onChange={(checked) => updateTeam({ oneMoreEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="仅已培育" checked={team?.submitOnlyCultivated ?? false} onChange={(checked) => updateTeam({ submitOnlyCultivated: checked })} />
                <QualityRow label="组团品质" value={team?.qualities ?? []} onChange={(value) => updateTeam({ qualities: value })} />
                <BigIntNumberRow label="组团元宝上限" value={team?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateTeam({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="花艺上架" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="解锁花架" checked={flowerArt?.autoUnlockStand ?? false} onChange={(checked) => updateFlowerArt({ autoUnlockStand: checked })} status={SETTING_STATUS.adapterMissing} />
                <ToggleRow label="自动上架" checked={flowerArt?.sellEnabled ?? false} onChange={(checked) => updateFlowerArt({ sellEnabled: checked })} />
                <ToggleRow label="自动制作" checked={flowerArt?.craftEnabled ?? false} onChange={(checked) => updateFlowerArt({ craftEnabled: checked })} />
                <ToggleRow label="提前下架" checked={flowerArt?.earlyCancelEnabled ?? false} onChange={(checked) => updateFlowerArt({ earlyCancelEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <IntListRow label="指定花艺" value={flowerArt?.specifiedArtIds ?? []} onChange={(value) => updateFlowerArt({ specifiedArtIds: value })} />
                <NumberRow label="每架数量" value={flowerArt?.perRackCount || 12} min={0} onChange={(value) => updateFlowerArt({ perRackCount: value })} />
                <ToggleRow label="花艺经验" checked={flowerArt?.createRewardEnabled ?? false} onChange={(checked) => updateFlowerArt({ createRewardEnabled: checked })} />
                <ToggleRow label="图鉴奖励" checked={flowerArt?.collectRewardEnabled ?? false} onChange={(checked) => updateFlowerArt({ collectRewardEnabled: checked })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "union" && (
          <div className="space-y-4">
            <PolicyGroup title="公会土地" icon={<Building2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动收获" checked={unionLand?.harvestEnabled ?? false} onChange={(checked) => updateUnionLand({ harvestEnabled: checked })} />
                <ToggleRow label="自动种植" checked={unionLand?.autoPlantEnabled ?? false} onChange={(checked) => updateUnionLand({ autoPlantEnabled: checked })} status={SETTING_STATUS.paused} />
                <SegmentedRow label="种植策略" value={unionLand?.plantMode || PlantingMode.QUALITY} options={PLANTING_MODE_OPTIONS} onChange={(value) => updateUnionLand({ plantMode: value })} />
                <QualityRow label="指定品质" value={unionLand?.qualities ?? []} onChange={(value) => updateUnionLand({ qualities: value })} />
                <IntListRow label="指定花朵" value={unionLand?.flowerIds ?? []} onChange={(value) => updateUnionLand({ flowerIds: value })} />
                <NumberRow label="最高花朵等级" value={unionLand?.maxFlowerLevel || 0} min={0} onChange={(value) => updateUnionLand({ maxFlowerLevel: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会建设" icon={<Coins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="视频建设" checked={unionBuild?.freeEnabled ?? false} onChange={(checked) => updateUnionBuild({ freeEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />
                <ToggleRow label="金币建设" checked={unionBuild?.goldEnabled ?? false} onChange={(checked) => updateUnionBuild({ goldEnabled: checked })} />
                <ToggleRow label="元宝建设" checked={unionBuild?.diamondEnabled ?? false} onChange={(checked) => updateUnionBuild({ diamondEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                <BigIntNumberRow label="金币上限" value={unionBuild?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendGold: value })} />
                <BigIntNumberRow label="元宝上限" value={unionBuild?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendDiamond: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会分享与摸花" icon={<HandCoins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动分享" checked={unionFlower?.shareEnabled ?? false} onChange={(checked) => updateUnionFlower({ shareEnabled: checked })} status={SETTING_STATUS.paused} />
                <SegmentedRow label="分享模式" value={unionFlower?.shareMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ shareMode: value })} />
                <QualityRow label="分享品质" value={unionFlower?.shareQualities ?? []} onChange={(value) => updateUnionFlower({ shareQualities: value })} />
                <IntListRow label="分享花朵" value={unionFlower?.shareFlowerIds ?? []} onChange={(value) => updateUnionFlower({ shareFlowerIds: value })} />
                <ToggleRow label="自动摸花" checked={unionFlower?.takeEnabled ?? false} onChange={(checked) => updateUnionFlower({ takeEnabled: checked })} />
                <SegmentedRow label="摸花模式" value={unionFlower?.takeMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ takeMode: value })} />
                <QualityRow label="摸花品质" value={unionFlower?.takeQualities ?? []} onChange={(value) => updateUnionFlower({ takeQualities: value })} />
                <IntListRow label="摸花花朵" value={unionFlower?.takeFlowerIds ?? []} onChange={(value) => updateUnionFlower({ takeFlowerIds: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会竞赛" icon={<Trophy />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动完成" checked={unionRace?.enabled ?? false} onChange={(checked) => updateUnionRace({ enabled: checked })} status={SETTING_STATUS.paused} />
                <ToggleRow label="自动启用模块" checked={unionRace?.autoEnableModules ?? false} onChange={(checked) => updateUnionRace({ autoEnableModules: checked })} status={SETTING_STATUS.paused} />
                <ToggleRow label="任务使用加速卡" checked={unionRace?.useSpeedupTicketInTask ?? false} onChange={(checked) => updateUnionRace({ useSpeedupTicketInTask: checked })} status={SETTING_STATUS.paused} />
                <NumberRow label="最低任务分" value={unionRace?.minTaskScore || 0} min={0} onChange={(value) => updateUnionRace({ minTaskScore: value })} />
                <ToggleRow label="只接已升级" checked={unionRace?.onlyUpgradeTask ?? false} onChange={(checked) => updateUnionRace({ onlyUpgradeTask: checked })} />
                <ToggleRow label="排除他人升级" checked={unionRace?.excludeOthersUpgradeTask ?? false} onChange={(checked) => updateUnionRace({ excludeOthersUpgradeTask: checked })} />
                <ToggleRow label="自动升级任务" checked={unionRace?.upgradeTask ?? false} onChange={(checked) => updateUnionRace({ upgradeTask: checked })} status={SETTING_STATUS.paused} />
                <ToggleRow label="删除低分任务" checked={unionRace?.deleteLowScoreTask ?? false} onChange={(checked) => updateUnionRace({ deleteLowScoreTask: checked })} status={SETTING_STATUS.paused} />
                <NumberRow label="删除分数上限" value={unionRace?.deleteTaskMaxScore || 0} min={0} onChange={(value) => updateUnionRace({ deleteTaskMaxScore: value })} />
                <BigIntNumberRow label="元宝上限" value={unionRace?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateUnionRace({ maxSpendDiamond: value })} />
              </div>
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                {UNION_RACE_TASKS.map((taskID) => (
                  <NumberRow
                    key={taskID}
                    label={`任务 ${taskID}`}
                    value={unionRace?.taskTypePriority?.[taskID] ?? 0}
                    min={0}
                    onChange={(value) => updateUnionRace({ taskTypePriority: { ...(unionRace?.taskTypePriority ?? {}), [taskID]: value } })}
                  />
                ))}
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会其他" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="公会红包" checked={union?.redPacketEnabled ?? false} onChange={(checked) => updateUnion({ redPacketEnabled: checked })} status={SETTING_STATUS.paused} />
                <ToggleRow label="能量森林" checked={union?.forestEnabled ?? false} onChange={(checked) => updateUnion({ forestEnabled: checked })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "activity" && (
          <div className="space-y-4">
            <PolicyGroup title="活动总开关" icon={<CalendarDays />}>
              <ToggleRow label="活动自动化" checked={activity?.enabled ?? false} onChange={(checked) => updateActivity({ enabled: checked })} />
            </PolicyGroup>
            <div className="grid gap-3">
              {ACTIVITY_MODULES.map((module) => {
                const modulePolicy = activity?.modules[module.id];
                return (
                  <PolicyGroup key={module.id} title={module.label} icon={<Play />}>
                    <div className="grid gap-2 sm:grid-cols-2">
                      <ToggleRow label="启用" checked={modulePolicy?.enabled ?? false} onChange={(checked) => updateActivityModule(module.id, { enabled: checked })} status={module.status} />
                      {module.boolParams?.map((param) => (
                        <ToggleRow
                          key={param.key}
                          label={param.label}
                          checked={modulePolicy?.boolParams?.[param.key] ?? false}
                          onChange={(checked) => updateActivityBoolParam(module.id, param.key, checked)}
                        />
                      ))}
                      {module.intParams?.map((param) => (
                        <BigIntNumberRow
                          key={param.key}
                          label={param.label}
                          value={modulePolicy?.intParams?.[param.key] ?? BigInt(0)}
                          min={param.min ?? 0}
                          onChange={(value) => updateActivityIntParam(module.id, param.key, value)}
                        />
                      ))}
                      {module.stringParams?.map((param) => (
                        <TextRow
                          key={param.key}
                          label={param.label}
                          value={modulePolicy?.stringParams?.[param.key] ?? ""}
                          onChange={(value) => updateActivityStringParam(module.id, param.key, value)}
                        />
                      ))}
                      {module.id === "cyclicNote" && (
                        <IntListRow
                          label="临时启用模块"
                          value={modulePolicy?.intListParams?.auto_enable_feature_ids?.values ?? []}
                          onChange={(value) => updateActivityIntListParam(module.id, "auto_enable_feature_ids", value)}
                        />
                      )}
                    </div>
                  </PolicyGroup>
                );
              })}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function PolicyGroup({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) {
  return (
    <section className="space-y-3 rounded-md border border-border/70 p-3">
      <SectionTitle icon={icon}>{title}</SectionTitle>
      {children}
    </section>
  );
}

function TextRow({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Input className="h-8 w-36 text-right text-sm" value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function BigIntNumberRow({
  label,
  value,
  min,
  onChange,
}: {
  label: string;
  value: bigint;
  min: number;
  onChange: (value: bigint) => void;
}) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Input
        type="number"
        className="h-8 w-28 text-right text-sm"
        min={min}
        value={value.toString()}
        onChange={(event) => onChange(parseBigInt(event.target.value, min))}
      />
    </div>
  );
}

function IntListRow({ label, value, onChange }: { label: string; value: number[]; onChange: (value: number[]) => void }) {
  return (
    <div className="space-y-2 rounded-md border border-border/70 px-3 py-2">
      <Label className="text-sm">{label}</Label>
      <Input
        className="h-8 text-sm"
        value={formatIntList(value)}
        onChange={(event) => onChange(parseIntList(event.target.value))}
        placeholder="用逗号分隔 ID"
      />
    </div>
  );
}

function QualityRow({ label, value, onChange }: { label: string; value: number[]; onChange: (value: number[]) => void }) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2">
      <Label className="min-w-0 text-sm">{label}</Label>
      <div className="flex gap-1">
        {QUALITY_OPTIONS.map((quality) => {
          const selected = value.includes(quality);
          return (
            <button
              key={quality}
              type="button"
              onClick={() => onChange(toggleNumber(value, quality))}
              className={cn(
                "flex size-7 items-center justify-center rounded border text-xs font-medium",
                selected ? "border-primary bg-primary text-primary-foreground" : "border-border/70 text-muted-foreground hover:text-foreground",
              )}
            >
              {quality}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function SegmentedRow<T extends number>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  return (
    <div className="space-y-2 rounded-md border border-border/70 px-3 py-2">
      <Label className="text-sm">{label}</Label>
      <div className="flex flex-wrap gap-1">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "min-h-8 rounded border px-2 text-xs font-medium",
              option.value === value ? "border-primary bg-primary text-primary-foreground" : "border-border/70 text-muted-foreground hover:text-foreground",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function EventPanel({ events }: { events: Event[] }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>操作日志</CardTitle>
          <Badge variant="secondary">{events.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <EmptyState title="暂无日志" />
        ) : (
          <div className="dark-scrollbar max-h-80 space-y-2 overflow-y-auto pr-1">
            {events.map((event, index) => (
              <div key={`${event.kind}-${index}-${event.message}`} className="rounded-md border border-border/70 px-3 py-2">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{event.label || event.kind}</div>
                    <div className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">{event.message || event.payloadJson}</div>
                  </div>
                  <Badge variant={event.level === "error" ? "destructive" : "outline"}>{event.category || event.domain || "system"}</Badge>
                </div>
                <div className="mt-1 text-xs text-muted-foreground">{formatTimestamp(event.ts)}</div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function MetricCard({ icon, label, value }: { icon: ReactNode; label: string; value: ReactNode }) {
  return (
    <Card>
      <CardContent className="flex items-center gap-3">
        <div className="flex size-9 items-center justify-center rounded-md bg-primary/10 text-primary [&_svg]:size-4">{icon}</div>
        <div>
          <div className="text-xs text-muted-foreground">{label}</div>
          <div className="text-lg font-semibold">{value}</div>
        </div>
      </CardContent>
    </Card>
  );
}

function MetricMini({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-0 rounded-md bg-background/35 px-2 py-1">
      <div className="truncate">{label}</div>
      <div className="truncate font-medium text-foreground">{value}</div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function ToggleRow({
  label,
  checked,
  onChange,
  status,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  status?: SettingStatus;
}) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2">
      <span className="flex min-w-0 flex-wrap items-center gap-2 text-sm">
        <span>{label}</span>
        {status && <SettingStatusBadge status={status} />}
      </span>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  );
}

function SettingStatusBadge({ status }: { status: SettingStatus }) {
  const variant = status.kind === "adapter_missing" ? "destructive" : "outline";
  return (
    <Badge variant={variant} title={status.detail} className="shrink-0">
      {status.label}
    </Badge>
  );
}

function NumberRow({
  label,
  value,
  min,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  onChange: (value: number) => void;
}) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Input
        type="number"
        className="h-8 w-24 text-right text-sm"
        min={min}
        value={Number.isFinite(value) ? value : min}
        onChange={(event) => onChange(parseNumber(event.target.value, min))}
      />
    </div>
  );
}

function SectionTitle({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2 text-sm font-semibold">
      <span className="text-primary [&_svg]:size-4">{icon}</span>
      {children}
    </div>
  );
}

function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/80 px-3 py-4 text-center">
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}

function HealthBadge({ account, status }: { account: Account; status?: AccountStatus }) {
  const connected = status?.connected ?? account.connected;
  if (!connected) return <Badge variant="outline">离线</Badge>;
  if (status?.health === "blocked" || status?.lastError) return <Badge variant="destructive">异常</Badge>;
  if (status?.automationEnabled) return <Badge variant="secondary">运行</Badge>;
  return <Badge>在线</Badge>;
}

function OperationStatusBadge({ operation }: { operation: PlannedOperation }) {
  if (operation.status === "blocked") return <Badge variant="destructive">阻塞</Badge>;
  if (operation.syncOnly) return <Badge variant="outline">同步</Badge>;
  if (!operation.executable) return <Badge variant="outline">{operation.status || "等待"}</Badge>;
  if (operation.status === "managed") return <Badge variant="secondary">调度</Badge>;
  return <Badge>可执行</Badge>;
}

function ReasonList({ reasons, fallback }: { reasons: string[]; fallback: string }) {
  if (reasons.length === 0) return <span className="text-muted-foreground">{fallback}</span>;
  return (
    <div className="flex flex-wrap gap-1">
      {reasons.map((reason) => (
        <Badge key={reason} variant="outline">
          {reason}
        </Badge>
      ))}
    </div>
  );
}

function OperationTarget({ operation }: { operation: PlannedOperation }) {
  const parts = [
    operation.targetId ? `目标 ${operation.targetId}` : "",
    operation.itemId ? itemName(operation.itemId) : "",
    operation.flowerId ? itemName(operation.flowerId) : "",
    operation.count ? `x${operation.count}` : "",
    operation.landIds.length ? `土地 ${operation.landIds.join(",")}` : "",
  ].filter(Boolean);
  return <span className="text-sm">{parts.join(" · ") || "-"}</span>;
}

function CostView({ operation }: { operation: PlannedOperation }) {
  const itemCosts = Object.entries(operation.itemCost)
    .filter(([, count]) => count > 0)
    .map(([id, count]) => `${itemName(Number(id))}x${count}`);
  const costs = [
    operation.goldCost ? `金币 ${operation.goldCost}` : "",
    operation.diamondCost ? `钻石 ${operation.diamondCost}` : "",
    ...itemCosts,
  ].filter(Boolean);
  return <span className="text-sm text-muted-foreground">{costs.join(" · ") || "-"}</span>;
}

function collectBlocked(snapshot: GetSnapshotResponse | null, status?: AccountStatus): BlockedItem[] {
  const items: BlockedItem[] = [];
  for (const domain of [...(status?.domainStatuses ?? []), ...(snapshot?.domainStatuses ?? [])]) {
    if (domain.blockedReasons.length || domain.lastError) {
      items.push({
        source: `domain:${domain.domain}`,
        label: domainStatusLabel(domain),
        reasons: [...domain.blockedReasons, domain.lastError].filter(Boolean),
      });
    }
  }
  for (const demand of snapshot?.demands ?? []) {
    if (demand.blockedReasons.length) {
      items.push({
        source: `demand:${demand.id}`,
        label: demand.label || itemName(demand.itemId),
        reasons: demand.blockedReasons,
      });
    }
  }
  for (const operation of snapshot?.plannedOperations ?? []) {
    if (operation.blockedReasons.length) {
      items.push({
        source: `operation:${operation.operationId || operation.rpc}`,
        label: operation.label || `${operation.domain}.${operation.action}`,
        reasons: operation.blockedReasons,
      });
    }
  }
  return dedupeBlocked(items);
}

function dedupeBlocked(items: BlockedItem[]) {
  const seen = new Set<string>();
  const deduped: BlockedItem[] = [];
  for (const item of items) {
    const key = `${item.source}:${item.reasons.join("|")}`;
    if (seen.has(key)) continue;
    seen.add(key);
    deduped.push(item);
  }
  return deduped;
}

function domainStatusLabel(domain: DomainStatus) {
  return `${categoryLabel(domain.category)} / ${domain.domain || "unknown"}`;
}

function categoryLabel(category: string) {
  switch (category) {
    case "basic":
      return "基础";
    case "plant":
      return "种植";
    case "order":
      return "订单";
    case "union":
      return "公会";
    case "activity":
      return "活动";
    case "account":
      return "账号";
    case "system":
      return "系统";
    default:
      return category || "-";
  }
}

function goalLabel(goalId: string) {
  return GOAL_OPTIONS.find((goal) => goal.id === goalId)?.label || goalId || "-";
}

function recommendationLabel(value: string) {
  switch (value) {
    case "harvest":
      return "可采收";
    case "plant":
      return "可种植";
    case "water":
      return "可浇水";
    case "wait":
      return "等待";
    default:
      return value || "未知";
  }
}

function formatTimestamp(ts?: Timestamp) {
  if (!ts) return "-";
  const milliseconds = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(milliseconds));
}

function parseNumber(value: string, min: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return min;
  return Math.max(min, Math.trunc(parsed));
}

function parseBigInt(value: string, min: number) {
  const cleaned = value.trim();
  if (!cleaned) return BigInt(min);
  try {
    const parsed = BigInt(cleaned);
    const floor = BigInt(min);
    return parsed < floor ? floor : parsed;
  } catch {
    return BigInt(min);
  }
}

function parseIntList(value: string) {
  const seen = new Set<number>();
  const out: number[] = [];
  for (const part of value.split(/[,\s，、]+/)) {
    const parsed = Number(part.trim());
    if (!Number.isInteger(parsed) || parsed <= 0 || seen.has(parsed)) {
      continue;
    }
    seen.add(parsed);
    out.push(parsed);
  }
  return out;
}

function formatIntList(value: number[]) {
  return value.join(", ");
}

function toggleNumber(values: number[], value: number) {
  if (values.includes(value)) {
    return values.filter((item) => item !== value);
  }
  return [...values, value].sort((a, b) => a - b);
}
