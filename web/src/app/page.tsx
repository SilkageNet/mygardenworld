"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ComponentProps, type FormEvent, type PointerEvent, type ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import {
  AlertTriangle,
  ArrowLeft,
  BadgeCheck,
  Building2,
  CalendarDays,
  Check,
  ChevronDown,
  Cloud,
  Coins,
  Flower2,
  Gem,
  GripVertical,
  HandCoins,
  ListChecks,
  Loader2,
  LogOut,
  Minus,
  Package,
  Play,
  Plus,
  RefreshCw,
  Save,
  Search,
  Send,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
  Sprout,
  Trash2,
  Trophy,
  Users,
  Waves,
} from "lucide-react";

import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  ActivityModulePolicySchema,
  ActivityPolicySchema,
  BasicPolicySchema,
  BasicTaskPolicySchema,
  BenefitPolicySchema,
  CultivatePolicySchema,
  CustomerOrderPolicySchema,
  FlowerElvesPolicySchema,
  FlowerMarketPolicySchema,
  FlowerArtPolicySchema,
  FriendStealPolicySchema,
  MarketBuyMode,
  MarketPutMode,
  OrderPolicySchema,
  PalaceOrderPolicySchema,
  PearlPolicySchema,
  PlantPolicySchema,
  PlantingPolicySchema,
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
  ZooPolicySchema,
} from "@/gen/mygardenworld/v1/policy_pb";
import type {
  ActivityModulePolicy,
  ActivityPolicy,
  BasicPolicy,
  BasicTaskPolicy,
  BenefitPolicy,
  CultivatePolicy,
  CustomerOrderPolicy,
  FlowerElvesPolicy,
  FlowerMarketPolicy,
  FlowerArtPolicy,
  FriendStealPolicy,
  OrderPolicy,
  PalaceOrderPolicy,
  PearlPolicy,
  PlantPolicy,
  PlantingPolicy,
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
  ZooPolicy,
} from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import { ExecutionLane, PlanStatus, QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import type {
  AccountStatus,
  ActivityItem,
  CyclicNoteMilestone,
  CyclicNoteTaskSlot,
  CyclicNoteView,
  DessertCelebrityLikeView,
  DessertMilestoneView,
  DessertModeView,
  DessertRuntimeView,
  DessertTaskView,
  DessertView,
  Event,
  GetSnapshotResponse,
  InventoryLedgerItem,
  InventoryLedgerView,
  LandView,
  OrderStatisticsView,
  PendingTaskView,
  PlantableFlowerView,
  PlannedOperation,
  RequirementView,
  RuntimeActionTotal,
  RuntimeResourceTotal,
  RuntimeStatisticsView,
} from "@/gen/mygardenworld/v1/query_service_pb";
import AppShell from "@/components/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
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
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatAPIError, transport } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { flowerDisplay, itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);
const queryClient = createClient(QueryService, transport);

const NUMBER_FORMATTER = new Intl.NumberFormat("zh-CN");
const EVENT_LIMIT = 120;
const EVENT_RECONNECT_INITIAL_MS = 1000;
const EVENT_RECONNECT_MAX_MS = 15000;
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
  { id: "order.customer", label: "顾客订单", defaultPriority: 90 },
  { id: "order.resident", label: "居民订单", defaultPriority: 80 },
  { id: "basic.task.main", label: "主线任务", defaultPriority: 70 },
  { id: "basic.task.daily", label: "日常任务", defaultPriority: 60 },
  { id: "basic.task.weekly", label: "周常任务", defaultPriority: 55 },
  { id: "order.flower_art", label: "花艺/花架", defaultPriority: 40 },
  { id: "fallback.auto_replant", label: "自主补种", defaultPriority: 10 },
];

type DashboardTabId = "monitor" | "settings" | "logs";
type PolicyTabId = "basic" | "order" | "union" | "other" | "activity";
type AccountQuota = {
  current: number;
  max: number;
  reached: boolean;
};
type WarehouseCategory = "flower" | "art" | "item";

const WAREHOUSE_CATEGORIES: { id: WarehouseCategory; label: string; icon: ReactNode }[] = [
  { id: "flower", label: "鲜花", icon: <Flower2 /> },
  { id: "art", label: "花艺", icon: <Sparkles /> },
  { id: "item", label: "道具", icon: <Package /> },
];

const SHOW_UNSUPPORTED_SETTINGS = false;

const POLICY_TABS: { id: PolicyTabId; label: string; icon: ReactNode }[] = [
  { id: "basic", label: "基础", icon: <Sprout /> },
  { id: "order", label: "订单", icon: <ListChecks /> },
  { id: "union", label: "公会", icon: <Users /> },
  { id: "other", label: "其他", icon: <ShoppingBag /> },
  { id: "activity", label: "活动", icon: <CalendarDays /> },
];

const DASHBOARD_TABS: { id: DashboardTabId; label: string; icon: ReactNode }[] = [
  { id: "monitor", label: "监控", icon: <BadgeCheck /> },
  { id: "settings", label: "设置", icon: <ShieldCheck /> },
  { id: "logs", label: "日志", icon: <CalendarDays /> },
];

const QUALITY_OPTIONS = [1, 2, 3, 4, 5];

const SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.QUALITY, label: "品质" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

const AUTO_REPLANT_SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
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
  intParams?: { key: string; label: string; defaultValue: number; min: number; max?: number }[];
};

const ACTIVITY_MODULES: ActivityModuleMeta[] = [
  {
    id: "cyclicNote",
    label: "花笺集芳",
    boolParams: [
      { key: "auto_claim_task_rewards", label: "自动领取任务奖励" },
      { key: "auto_claim_progress_boxes", label: "自动领取积分奖励" },
      { key: "satisfy_tasks", label: "驱动已启用模块完成任务" },
    ],
  },
  {
    id: "actDessert",
    label: "香卉甜糕",
    boolParams: [
      { key: "auto_claim_task_rewards", label: "自动领取任务奖励" },
      { key: "auto_like_celebrity", label: "自动免费点赞" },
      { key: "auto_open_reward_boxes", label: "自动开启奖励箱（每次1个）" },
      { key: "auto_play", label: "启用影子诊断（不执行）" },
      { key: "resume_existing_round", label: "请求接管评估（当前硬锁）" },
    ],
    intParams: [
      { key: "mode", label: "影子模式（仅 1 可用）", defaultValue: 1, min: 1, max: 1 },
      { key: "max_energy_per_session", label: "会话体力预算（0=禁用；当前仅诊断）", defaultValue: 0, min: 0, max: 100 },
      { key: "min_energy_reserve", label: "最低体力保留", defaultValue: 0, min: 0 },
    ],
  },
];

const EMPTY_ADD_FORM = {
  channel: Channel.IOS,
  username: "",
  password: "",
};

type AddAccountForm = typeof EMPTY_ADD_FORM;

export default function HomePage() {
  return (
    <AppShell>
      <DashboardContent />
    </AppShell>
  );
}

function DashboardContent() {
  const { user } = useAuth();
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
  const [dashboardTab, setDashboardTab] = useState<DashboardTabId>("monitor");
  const didAutoSelectAccount = useRef(false);
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) ?? null,
    [accounts, selectedAccountId],
  );
  const selectedStatus = selectedAccountId ? statuses.get(selectedAccountId) : undefined;
  const selectedConnected = selectedAccount ? accountConnected(selectedAccount, selectedStatus) : false;
  const hasAccounts = accounts.length > 0;
  const creatingAccount = busyAction === "create";
  const accountQuota = useMemo<AccountQuota | null>(() => {
    if (!user) return null;
    const current = accounts.length;
    const max = user.maxAccounts;
    return {
      current,
      max,
      reached: current >= max,
    };
  }, [accounts.length, user]);

  useEffect(() => {
    accountsRef.current = accounts;
  }, [accounts]);

  useEffect(() => {
    statusesRef.current = statuses;
  }, [statuses]);

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
    setError((current) => (isTransientConnectionMessage(current) ? "" : current));
  }, []);

  const canReadSnapshot = useCallback((accountId: string) => {
    const account = accountsRef.current.find((item) => item.id === accountId);
    if (!account) return false;
    return accountConnected(account, statusesRef.current.get(accountId));
  }, []);

  const refreshSnapshot = useCallback(async (accountId: string, showLoading = false, options?: { force?: boolean }) => {
    if (!accountId) {
      setSnapshot(null);
      return;
    }
    if (!options?.force && !canReadSnapshot(accountId)) {
      setSnapshot(null);
      setSnapshotLoading(false);
      setError((current) => (isRunnerNotStartedError(current) ? "" : current));
      return;
    }
    if (showLoading) {
      setSnapshotLoading(true);
    }
    try {
      setSnapshot(await queryClient.getSnapshot({ accountId }));
    } catch (err) {
      setSnapshot(null);
      if (!isRunnerNotStartedError(err)) {
        setError(formatAPIError(err, "读取快照失败"));
      } else {
        setError((current) => (isRunnerNotStartedError(current) ? "" : current));
      }
    } finally {
      setSnapshotLoading(false);
    }
  }, [canReadSnapshot]);

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
      setPolicyMessage(formatAPIError(err, "读取策略失败"));
    } finally {
      setPolicyLoading(false);
    }
  }, []);

  const refreshWorkspace = useCallback(async () => {
    setError("");
    try {
      await refreshStatus();
    } catch (err) {
      setError(formatAPIError(err, "刷新失败"));
    } finally {
      setLoading(false);
    }
  }, [refreshStatus]);

  useEffect(() => {
    void refreshWorkspace();
  }, [refreshWorkspace]);

  useEffect(() => {
    if (accounts.length === 0) {
      setSelectedAccountId("");
      didAutoSelectAccount.current = false;
      return;
    }
    if (selectedAccountId && !accounts.some((account) => account.id === selectedAccountId)) {
      setSelectedAccountId(accounts[0].id);
      didAutoSelectAccount.current = true;
      return;
    }
    if (!selectedAccountId && !didAutoSelectAccount.current) {
      setSelectedAccountId(accounts[0].id);
      didAutoSelectAccount.current = true;
    }
  }, [accounts, selectedAccountId]);

  useEffect(() => {
    setDashboardTab("monitor");
  }, [selectedAccountId]);

  useEffect(() => {
    if (!selectedAccountId) {
      setSnapshot(null);
      setPolicy(null);
      setEvents([]);
      return;
    }
    if (selectedConnected) {
      void refreshSnapshot(selectedAccountId, true);
    } else {
      setSnapshot(null);
      setSnapshotLoading(false);
      setError((current) => (isRunnerNotStartedError(current) ? "" : current));
    }
    void refreshPolicy(selectedAccountId);
  }, [refreshPolicy, refreshSnapshot, selectedAccountId, selectedConnected]);

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
    if (!selectedAccountId) {
      setEvents([]);
      return;
    }
    const controller = new AbortController();
    let active = true;
    setEvents([]);

    async function readEvents() {
      let retryDelayMs = EVENT_RECONNECT_INITIAL_MS;
      let lastEventId = BigInt(0);
      while (active && !controller.signal.aborted) {
        let receivedEvent = false;
        try {
          for await (const event of queryClient.streamEvents(
            { accountId: selectedAccountId, replayLimit: EVENT_LIMIT, afterEventId: lastEventId },
            { signal: controller.signal },
          )) {
            if (!active || controller.signal.aborted) return;
            if (event.id > BigInt(0)) {
              if (event.id <= lastEventId) continue;
              lastEventId = event.id;
            }
            receivedEvent = true;
            retryDelayMs = EVENT_RECONNECT_INITIAL_MS;
            setError((current) => (isTransientConnectionMessage(current) ? "" : current));
            setEvents((prev) => [event, ...prev].slice(0, EVENT_LIMIT));
            if (SNAPSHOT_REFRESH_EVENT_KINDS.has(event.kind)) {
              void refreshSnapshot(selectedAccountId).catch(() => undefined);
            }
          }
        } catch (err) {
          if (!active || controller.signal.aborted) return;
          const streamError = formatAPIError(err, "事件流中断");
          setError((current) => (current && !isTransientConnectionMessage(current) ? current : streamError));
        }

        if (!active || controller.signal.aborted) return;
        const retry = await waitForAbortableDelay(retryDelayMs, controller.signal);
        if (!retry) return;
        retryDelayMs = receivedEvent
          ? EVENT_RECONNECT_INITIAL_MS
          : Math.min(retryDelayMs * 2, EVENT_RECONNECT_MAX_MS);
      }
    }

    void readEvents();
    return () => {
      active = false;
      controller.abort();
    };
  }, [refreshSnapshot, selectedAccountId]);

  async function runAccountAction(action: "login" | "logout") {
    if (!selectedAccount) return;
    setBusyAction(action);
    setError("");
    try {
      if (action === "login") await accountClient.loginAccount({ id: selectedAccount.id });
      if (action === "logout") await accountClient.logoutAccount({ id: selectedAccount.id });
      await refreshStatus();
      await refreshSnapshot(selectedAccount.id, action === "login", { force: action === "login" });
      await refreshPolicy(selectedAccount.id);
    } catch (err) {
      setError(formatAPIError(err, "操作失败"));
    } finally {
      setBusyAction("");
    }
  }

  async function createAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!addForm.username.trim() || !addForm.password) return;
    if (accountQuota?.reached) {
      setError(`账号已满（${accountQuota.current}/${accountQuota.max}）`);
      return;
    }
    setBusyAction("create");
    setError("");
    try {
      const res = await accountClient.createAccount({
        username: addForm.username.trim(),
        password: addForm.password,
        channel: addForm.channel,
        loginNow: true,
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
      setError(formatAPIError(err, "新增账号失败"));
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
      setError(formatAPIError(err, "删除账号失败"));
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
      setPolicyMessage("");
      await refreshStatus();
      await refreshSnapshot(selectedAccount.id);
    } catch (err) {
      setPolicyMessage(formatAPIError(err, "保存失败"));
    } finally {
      setSavingPolicy(false);
    }
  }

  return (
    <div className="relative z-10 min-h-0 xl:h-full">
      {error && (
        <div className="mb-4 rounded-md border border-destructive/25 bg-white/72 px-3 py-2 text-sm text-destructive shadow-sm backdrop-blur-xl dark:bg-destructive/12">
          {error}
        </div>
      )}

      <div
        className={cn(
          "min-h-0 gap-3 sm:gap-4 xl:h-full",
          hasAccounts ? "grid xl:grid-cols-[320px_minmax(0,1fr)]" : "flex justify-center",
        )}
      >
        <aside className={cn("min-h-0 min-w-0", selectedAccount && "hidden xl:block", !hasAccounts && "w-full max-w-md")}>
          <AccountListPanel
            accounts={accounts}
            statuses={statuses}
            selectedAccountId={selectedAccountId}
            loading={loading}
            quota={accountQuota}
            onRefresh={() => void refreshWorkspace()}
            onAdd={() => setAddOpen(true)}
            onSelect={setSelectedAccountId}
          />
        </aside>

        {hasAccounts && (
          <section className={cn("dark-scrollbar min-h-0 min-w-0 w-full xl:h-full xl:overflow-y-auto xl:pr-1", !selectedAccount && "hidden xl:block")}>
            {selectedAccount ? (
              <AccountDetailView
                account={selectedAccount}
                status={selectedStatus}
                snapshot={snapshot}
                snapshotLoading={snapshotLoading}
                busyAction={busyAction}
                activeTab={dashboardTab}
                events={events}
                policy={policy}
                policyLoading={policyLoading}
                savingPolicy={savingPolicy}
                policyMessage={policyMessage}
                onBack={() => setSelectedAccountId("")}
                onTabChange={setDashboardTab}
                onRefresh={() => void refreshSnapshot(selectedAccount.id, true)}
                onAction={runAccountAction}
                onDelete={() => void deleteSelectedAccount()}
                onPolicyChange={setPolicy}
                onPolicySave={() => void savePolicy()}
              />
            ) : (
              <SelectAccountPlaceholder />
            )}
          </section>
        )}
      </div>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增账号</DialogTitle>
          </DialogHeader>
          <form className="space-y-4" onSubmit={createAccount}>
            <Field label="渠道">
              <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label="渠道">
                <button
                  type="button"
                  role="radio"
                  aria-checked={addForm.channel === Channel.IOS}
                  className={cn(
                    "h-10 rounded-md border px-3 text-sm font-medium transition-colors",
                    addForm.channel === Channel.IOS
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border/70 text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => setAddForm((prev) => ({ ...prev, channel: Channel.IOS }))}
                  disabled={creatingAccount}
                >
                  iOS
                </button>
                <button
                  type="button"
                  className="h-10 cursor-not-allowed rounded-md border border-border/70 px-3 text-sm font-medium text-muted-foreground/50"
                  disabled
                >
                  安卓
                </button>
                <button
                  type="button"
                  className="h-10 cursor-not-allowed rounded-md border border-border/70 px-3 text-sm font-medium text-muted-foreground/50"
                  disabled
                >
                  微信
                </button>
              </div>
            </Field>
            <Field label="账号">
              <Input
                value={addForm.username}
                onChange={(event) => setAddForm((prev) => ({ ...prev, username: event.target.value }))}
                autoComplete="username"
                disabled={creatingAccount}
              />
            </Field>
            <Field label="密码">
              <Input
                type="password"
                value={addForm.password}
                onChange={(event) => setAddForm((prev) => ({ ...prev, password: event.target.value }))}
                autoComplete="current-password"
                disabled={creatingAccount}
              />
            </Field>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)} disabled={creatingAccount}>
                取消
              </Button>
              <Button type="submit" disabled={creatingAccount || accountQuota?.reached}>
                {creatingAccount ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
                {creatingAccount ? "添加中" : "新增"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AccountListPanel({
  accounts,
  statuses,
  selectedAccountId,
  loading,
  quota,
  onRefresh,
  onAdd,
  onSelect,
}: {
  accounts: Account[];
  statuses: Map<string, AccountStatus>;
  selectedAccountId: string;
  loading: boolean;
  quota: AccountQuota | null;
  onRefresh: () => void;
  onAdd: () => void;
  onSelect: (accountId: string) => void;
}) {
  const hasAccounts = accounts.length > 0;
  const quotaReached = quota?.reached ?? false;
  return (
    <Card className={cn("cloud-surface min-h-[340px]", hasAccounts ? "xl:h-full xl:min-h-[480px]" : "xl:min-h-[360px]")}>
      <CardHeader className="border-b border-border/45 pb-2.5 sm:pb-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="flex size-8 items-center justify-center rounded-md bg-white/72 text-sky-500 shadow-sm dark:bg-white/8 dark:text-sky-300">
              <Cloud className="size-4" />
            </span>
            <CardTitle>账号</CardTitle>
            {quota ? (
              <Badge variant={quotaReached ? "destructive" : "secondary"}>{quota.current}/{quota.max}</Badge>
            ) : (
              hasAccounts && <Badge variant="secondary">{accounts.length}</Badge>
            )}
          </div>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon-sm" onClick={onRefresh} aria-label="刷新" disabled={loading}>
              <RefreshCw className={cn("size-4", loading && "animate-spin")} />
            </Button>
            {hasAccounts && (
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                onClick={onAdd}
                aria-label="新增账号"
                disabled={quotaReached}
              >
                <Plus className="size-4" />
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
        {!hasAccounts ? (
          <div className="flex min-h-[220px] flex-1 flex-col items-center justify-center px-4 py-8 text-center">
            <div className="mb-4 flex size-14 items-center justify-center rounded-full bg-white/78 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300">
              <Cloud className="size-6" />
            </div>
            <div className="text-base font-semibold">还没有账号</div>
            <div className="mt-1 text-sm text-muted-foreground">添加后开始监控。</div>
            <Button type="button" className="mt-5 w-full max-w-xs" onClick={onAdd} disabled={quotaReached}>
              <Plus className="size-4" />
              添加账号
            </Button>
          </div>
        ) : (
          <div className="dark-scrollbar flex-1 space-y-2 overflow-y-auto pr-0.5 sm:pr-1">
            {accounts.map((account) => {
              const status = statuses.get(account.id);
              const selected = account.id === selectedAccountId;
              const identity = accountIdentity(account);
              return (
                <button
                  key={account.id}
                  type="button"
                  className={cn(
                    "w-full rounded-md border p-3 text-left shadow-sm transition-all active:scale-[0.99]",
                    selected
                      ? "border-primary/45 bg-white/78 shadow-[0_10px_20px_rgba(255,111,97,0.12)] dark:bg-primary/12 dark:shadow-black/20"
                      : "border-border/58 bg-white/42 hover:border-ring/45 hover:bg-white/66 dark:bg-white/5 dark:hover:bg-white/8",
                  )}
                  onClick={() => onSelect(account.id)}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{identity.nickname}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                        <span>{identity.area}</span>
                        <span>{identity.channel}</span>
                      </div>
                    </div>
                    <HealthBadge status={status} account={account} />
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function SelectAccountPlaceholder() {
  return (
    <Card className="cloud-surface flex h-full min-h-[480px] items-center justify-center">
      <CardContent className="max-w-md text-center">
        <div className="mx-auto mb-3 flex size-14 items-center justify-center rounded-full bg-white/76 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300">
          <Send className="size-5" />
        </div>
        <div className="text-base font-semibold">选择账号</div>
        <div className="mt-1 text-sm text-muted-foreground">从左侧进入监控。</div>
      </CardContent>
    </Card>
  );
}

function AccountDetailView({
  account,
  status,
  snapshot,
  snapshotLoading,
  busyAction,
  activeTab,
  events,
  policy,
  policyLoading,
  savingPolicy,
  policyMessage,
  onBack,
  onTabChange,
  onRefresh,
  onAction,
  onDelete,
  onPolicyChange,
  onPolicySave,
}: {
  account: Account;
  status?: AccountStatus;
  snapshot: GetSnapshotResponse | null;
  snapshotLoading: boolean;
  busyAction: string;
  activeTab: DashboardTabId;
  events: Event[];
  policy: Policy | null;
  policyLoading: boolean;
  savingPolicy: boolean;
  policyMessage: string;
  onBack: () => void;
  onTabChange: (tab: DashboardTabId) => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
  onPolicyChange: (policy: Policy | null) => void;
  onPolicySave: () => void;
}) {
  return (
    <div className="flex min-h-0 w-full min-w-0 max-w-full flex-col gap-3 sm:gap-4 xl:h-full">
      <div className="shrink-0">
        <HeaderPanel
          account={account}
          status={status}
          snapshotLoading={snapshotLoading}
          busyAction={busyAction}
          onBack={onBack}
          onRefresh={onRefresh}
          onAction={onAction}
          onDelete={onDelete}
        />
      </div>
      <DashboardTabBar activeTab={activeTab} onChange={onTabChange} />
      {activeTab === "monitor" && (
        <div className="min-h-0">
          <MonitorTab snapshot={snapshot} status={status} />
        </div>
      )}
      {activeTab === "logs" && (
        <div className="flex min-h-0 flex-1">
          <EventPanel events={events} />
        </div>
      )}
      {activeTab === "settings" && (
        <div className="min-h-0">
          <PolicyPanel
            policy={policy}
            snapshot={snapshot}
            loading={policyLoading}
            saving={savingPolicy}
            message={policyMessage}
            onPolicyChange={onPolicyChange}
            onSave={onPolicySave}
          />
        </div>
      )}
    </div>
  );
}

function DashboardTabBar({
  activeTab,
  onChange,
}: {
  activeTab: DashboardTabId;
  onChange: (tab: DashboardTabId) => void;
}) {
  return (
    <div className="dark-scrollbar sticky top-[3.25rem] z-10 flex shrink-0 gap-1 overflow-x-auto rounded-md border border-white/58 bg-white/62 p-1 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/72 sm:top-14 xl:static">
      {DASHBOARD_TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          className={cn(
            "flex h-9 min-w-[6.25rem] shrink-0 items-center justify-center gap-2 rounded px-3 text-sm font-semibold transition-all active:scale-[0.99] sm:min-w-20",
            activeTab === tab.id
              ? "bg-primary text-primary-foreground shadow-[0_8px_18px_rgba(255,111,97,0.24)]"
              : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
          )}
          onClick={() => onChange(tab.id)}
        >
          <span className="[&_svg]:size-4">{tab.icon}</span>
          {tab.label}
        </button>
      ))}
    </div>
  );
}

function MonitorTab({ snapshot, status }: { snapshot: GetSnapshotResponse | null; status?: AccountStatus }) {
  const runtimeStatistics = snapshot?.runtimeStatistics ?? status?.runtimeStatistics;
  return (
    <div className="space-y-3 sm:space-y-4">
      <StatusOverviewPanel snapshot={snapshot} status={status} />
      <RuntimeStatisticsPanel runtimeStatistics={runtimeStatistics} />
      <OperationPanel operations={snapshot?.plannedOperations ?? []} />
      <TaskOrderMonitorPanel tasks={snapshot?.pendingTasks ?? []} statistics={snapshot?.orderStatistics} />
      <LandWarehouseMonitorPanel lands={snapshot?.lands ?? []} ledger={snapshot?.inventoryLedger} />
      <CyclicNoteMonitorPanel activity={snapshot?.cyclicNote} />
      <DessertMonitorPanel activity={snapshot?.dessert} />
    </div>
  );
}

function HeaderPanel({
  account,
  status,
  snapshotLoading,
  busyAction,
  onBack,
  onRefresh,
  onAction,
  onDelete,
}: {
  account: Account;
  status?: AccountStatus;
  snapshotLoading: boolean;
  busyAction: string;
  onBack: () => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
}) {
  const connected = accountConnected(account, status);
  const sessionAction = connected ? "logout" : "login";
  const identity = accountIdentity(account);
  const statusIssues = accountStatusIssues(status);
  return (
    <Card className="cloud-surface bg-card/88">
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-start gap-3 sm:items-center">
            <Button type="button" variant="ghost" size="icon-sm" className="mt-0.5 shrink-0 xl:hidden" onClick={onBack} aria-label="返回账号列表">
              <ArrowLeft className="size-4" />
            </Button>
            <div className="hidden size-12 shrink-0 items-center justify-center rounded-full bg-white/72 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300 sm:flex">
              <Cloud className="size-6" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                <h1 className="min-w-0 max-w-full truncate text-xl font-semibold leading-tight sm:text-xl">{identity.nickname}</h1>
                <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
                  <span>{identity.area}</span>
                  <span>·</span>
                  <span>{identity.channel}</span>
                </div>
                <HealthBadge account={account} status={status} />
              </div>
            </div>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-1">
            <IconButtonWithTooltip label="刷新" type="button" variant="outline" size="icon-sm" onClick={onRefresh} disabled={snapshotLoading || !connected}>
              <RefreshCw className={cn("size-4", snapshotLoading && "animate-spin")} />
            </IconButtonWithTooltip>
            <IconButtonWithTooltip
              label={connected ? "退出登录" : "登录"}
              type="button"
              variant="outline"
              size="icon-sm"
              onClick={() => void onAction(sessionAction)}
              disabled={busyAction === sessionAction}
            >
              {busyAction === sessionAction ? (
                <Loader2 className="size-4 animate-spin" />
              ) : connected ? (
                <LogOut className="size-4" />
              ) : (
                <Play className="size-4" />
              )}
            </IconButtonWithTooltip>
            <IconButtonWithTooltip label="删除账号" type="button" variant="destructive" size="icon-sm" onClick={onDelete} disabled={busyAction === "delete"}>
              <Trash2 className="size-4" />
            </IconButtonWithTooltip>
          </div>
        </div>
        {statusIssues.length > 0 && (
          <div className="rounded-md border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive shadow-sm">
            <div className="flex items-start gap-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <div className="min-w-0 space-y-1">
                <div className="font-medium">异常信息</div>
                {statusIssues.map((issue) => (
                  <div key={issue} className="break-words text-destructive/90">
                    {issue}
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function IconButtonWithTooltip({
  label,
  children,
  ...props
}: ComponentProps<typeof Button> & { label: string }) {
  return (
    <Tooltip disabled={props.disabled}>
      <TooltipTrigger
        render={
          <Button {...props} aria-label={props["aria-label"] ?? label}>
            {children}
          </Button>
        }
      />
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

function accountIdentity(account: Account) {
  return {
    nickname: accountNickname(account),
    area: accountAreaLabel(account),
    channel: channelLabel(account.channel),
  };
}

function accountNickname(account: Account) {
  const withoutArea = account.name
    .replace(/\s*·\s*第\s*\d+\s*区(?:\s*#\d+)?\s*$/, "")
    .replace(/\s+第\s*\d+\s*区(?:\s*#\d+)?\s*$/, "")
    .trim();
  const withoutServerPrefix = withoutArea.replace(/^s\d{2,}[.．·\s_-]+/i, "").trim();
  return withoutServerPrefix || withoutArea || account.name || "账号";
}

function accountAreaLabel(account: Account) {
  if (account.gsIdx > 0) return `第${account.gsIdx}区`;
  const match = account.name.match(/第\s*(\d+)\s*区/);
  if (match) return `第${match[1]}区`;
  const serverMatch = account.name.match(/^s(\d{2,})[.．·\s_-]+/i);
  if (serverMatch) return `第${serverMatch[1]}区`;
  return "未知区";
}

function channelLabel(channel: Channel) {
  switch (channel) {
    case Channel.IOS:
      return "iOS";
    default:
      return "未知渠道";
  }
}

function accountConnected(account: Account, status?: AccountStatus) {
  return status?.connected ?? account.connected;
}

function isRunnerNotStartedError(err: unknown) {
  const message = err instanceof Error ? err.message : String(err ?? "");
  return message.includes("runner not started") || message.includes("failed_precondition");
}

function isTransientConnectionMessage(message: string) {
  return /network\s*error|networkerror|failed to fetch|load failed|无法连接到后端服务|事件流中断|后端服务暂时不可用|请求超时/i.test(message);
}

function waitForAbortableDelay(delayMs: number, signal: AbortSignal): Promise<boolean> {
  if (signal.aborted) return Promise.resolve(false);
  return new Promise((resolve) => {
    const onTimeout = () => {
      signal.removeEventListener("abort", onAbort);
      resolve(true);
    };
    const timeout = window.setTimeout(onTimeout, delayMs);
    const onAbort = () => {
      window.clearTimeout(timeout);
      signal.removeEventListener("abort", onAbort);
      resolve(false);
    };
    signal.addEventListener("abort", onAbort, { once: true });
    if (signal.aborted) onAbort();
  });
}

const FLORAL_COIN_ITEM_ID = 1002;

function CollapsibleCard({
  title,
  actions,
  children,
  className,
  contentClassName,
  defaultOpen = true,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <Card className={cn("cloud-surface bg-card/88", !open && "gap-0", className)}>
      <CardHeader className="border-b border-border/42 px-3 pb-3 sm:px-4">
        <div className="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
          <button
            type="button"
            className="flex min-w-0 items-center gap-2 text-left text-foreground transition-colors hover:text-primary active:scale-[0.99]"
            aria-expanded={open}
            onClick={() => setOpen((value) => !value)}
          >
            <ChevronDown className={cn("size-4 shrink-0 transition-transform", !open && "-rotate-90")} />
            <CardTitle className="truncate">{title}</CardTitle>
          </button>
          {actions && <div className="flex min-w-0 flex-wrap justify-end gap-1.5">{actions}</div>}
        </div>
      </CardHeader>
      {open && <CardContent className={cn("px-3 sm:px-4", contentClassName)}>{children}</CardContent>}
    </Card>
  );
}

function StatusOverviewPanel({ snapshot, status }: { snapshot: GetSnapshotResponse | null; status?: AccountStatus }) {
  const floralCoins = snapshot?.inventory[FLORAL_COIN_ITEM_ID] ?? 0;
  const reputationObserved = snapshot?.reputationObserved ?? status?.reputationObserved ?? false;
  const reputationScore = snapshot?.reputationScore ?? status?.reputationScore ?? 0;
  const reputationTime = firstPositiveUnixTime(
    snapshot?.reputationLastViewTimeMs,
    snapshot?.reputationLastSyncTimeMs,
    status?.reputationLastViewTimeMs,
    status?.reputationLastSyncTimeMs,
  );
  const level = snapshot?.level ?? status?.level ?? 0;
  const experience = snapshot?.experience ?? status?.experience ?? 0;
  const reputationDetail = reputationObserved ? (reputationTime ? `同步 ${formatUnixTime(reputationTime)}` : "已同步") : "未同步";
  return (
    <CollapsibleCard title="监控概览" actions={snapshot?.capturedAt && <Badge variant="outline">快照 {formatTimestamp(snapshot.capturedAt)}</Badge>}>
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat
          icon={<ShieldCheck />}
          label="礼仪分"
          value={reputationObserved ? formatCount(reputationScore) : "-"}
          detail={reputationDetail}
        />
        <OverviewStat icon={<Trophy />} label="等级" value={level > 0 ? `${level}级` : "-"} detail={`经验 ${formatCount(experience)}`} />
        <OverviewStat icon={<Waves />} label="水滴" value={`${formatCount(snapshot?.waterDrops ?? 0)}/${formatCount(snapshot?.waterDropsTotal ?? 0)}`} />
        <OverviewStat icon={<Coins />} label="金币" value={formatCount(snapshot?.gold ?? 0)} />
        <OverviewStat icon={<Gem />} label="元宝" value={formatCount(snapshot?.diamondsFree ?? 0)} />
        <OverviewStat icon={<HandCoins />} label="花坊币" value={formatCount(floralCoins)} />
      </div>
    </CollapsibleCard>
  );
}

function RuntimeStatisticsPanel({ runtimeStatistics }: { runtimeStatistics?: RuntimeStatisticsView }) {
  const runtimeOrderCompletions = runtimeStatistics?.orderCompletions ?? [];
  const runtimeTaskCompletions = runtimeStatistics?.taskCompletions ?? [];
  const runtimeTotalOperations = runtimeStatistics?.totalOperations ?? BigInt(0);
  const runtimeOrderTotal = sumRuntimeActionTotals(runtimeOrderCompletions);
  const runtimeTaskTotal = sumRuntimeActionTotals(runtimeTaskCompletions);
  const runtimeResourceGains = runtimeStatistics?.resourceGains ?? [];
  const showCompletionGroups =
    runtimeOrderCompletions.length > 0 || runtimeTaskCompletions.length > 0 || runtimeTotalOperations > BigInt(0);

  return (
    <CollapsibleCard
      title="本次运行统计"
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={runtimeStatistics?.running ? "secondary" : "outline"}>{runtimeStatistics ? (runtimeStatistics.running ? "运行中" : "已停止") : "暂无"}</Badge>
          {runtimeTotalOperations > BigInt(0) && <Badge variant="outline">操作 {formatCount(runtimeTotalOperations)}</Badge>}
        </>
      }
    >
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat icon={<CalendarDays />} label="本次运行" value={runtimeStatistics ? (runtimeStatistics.running ? "运行中" : "已停止") : "-"} detail={runtimeWindowLabel(runtimeStatistics)} />
        <OverviewStat icon={<Sparkles />} label="本次获取" value={runtimeResourcePrimaryValue(runtimeResourceGains)} detail={runtimeResourceGainSummary(runtimeResourceGains)} />
        <OverviewStat icon={<ShoppingBag />} label="本次订单" value={runtimeStatistics ? formatCount(runtimeOrderTotal) : "-"} detail={runtimeActionSummary(runtimeOrderCompletions)} />
        <OverviewStat icon={<ListChecks />} label="本次任务" value={runtimeStatistics ? formatCount(runtimeTaskTotal) : "-"} detail={runtimeActionSummary(runtimeTaskCompletions)} />
      </div>
      {showCompletionGroups && (
        <div className="grid gap-2 xl:grid-cols-2">
          <RuntimeCompletionGroup title="订单完成" items={runtimeOrderCompletions} emptyText="本次暂无订单完成" />
          <RuntimeCompletionGroup title="任务完成" items={runtimeTaskCompletions} emptyText="本次暂无任务完成" />
        </div>
      )}
    </CollapsibleCard>
  );
}

function CyclicNoteMonitorPanel({ activity }: { activity?: CyclicNoteView }) {
  const activeTasks = activity?.tasks.filter((task) => task.unlocked) ?? [];
  const readyTasks = activity?.valid ? activeTasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const readyMilestones = activity?.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;
  const phase = activity?.phase ?? 0;

  return (
    <CollapsibleCard
      title={activity?.name || "花笺集芳"}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          {activity?.found && <Badge variant="outline">批次 {activity.batchId}</Badge>}
          {activity?.found && !activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity?.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyTasks + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyTasks + readyMilestones}</Badge>}
        </>
      }
    >
      {!activity?.observed ? (
        <EmptyState title="花笺集芳状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.found ? (
        <EmptyState title="当前未发现花笺集芳活动" detail="活动批次按类型与服务端时间动态识别，不固定使用历史批次。" />
      ) : !activity.valid ? (
        <EmptyState title="花笺集芳配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示任务详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat
              icon={<CalendarDays />}
              label="活动阶段"
              value={cyclicNotePhaseLabel(phase)}
              detail={cyclicNotePhaseDetail(activity)}
            />
            <OverviewStat icon={<Trophy />} label="累计积分" value={formatCount(activity.score)} detail={`完成任务 ${formatCount(activity.finishCount)} 次`} />
            <OverviewStat
              icon={<Flower2 />}
              label="花笺余额"
              value={formatCount(activity.currencyBalance)}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "活动货币未识别"}
            />
            <OverviewStat
              icon={<ListChecks />}
              label="任务槽"
              value={`${activeTasks.length}/${activity.tasks.length}`}
              detail={readyTasks > 0 ? `${readyTasks} 个奖励可领取` : activity.taskListObserved ? "已同步" : "等待进入活动同步"}
            />
          </div>

          {activity.description && <div className="rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>任务详情</span>
              <Badge variant="secondary">{activity.tasks.length} 槽</Badge>
            </div>
            {activity.tasks.length === 0 ? (
              <div className="p-3">
                <EmptyState title={activity.taskListObserved ? "当前没有任务槽" : "任务列表尚未同步"} />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.tasks.map((task) => (
                  <CyclicNoteTaskCard key={`${activity.batchId}:${task.slotId}:${task.taskId}`} task={task} />
                ))}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>积分里程碑</span>
              <Badge variant="secondary">积分 {formatCount(activity.score)}</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3">
                <EmptyState title="暂无里程碑配置" />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => (
                  <CyclicNoteMilestoneCard key={milestone.index} milestone={milestone} />
                ))}
              </div>
            )}
          </section>

          {activity.items.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {activity.items.map((item) => (
                <ActivityItemChip key={item.itemId} item={item} />
              ))}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

function CyclicNoteTaskCard({ task }: { task: CyclicNoteTaskSlot }) {
  if (!task.unlocked) {
    return (
      <div className="rounded-md border border-dashed border-border/70 bg-muted/15 p-3 text-sm text-muted-foreground">
        <div className="flex items-center justify-between gap-2">
          <span className="font-medium">任务槽 {task.slotId}</span>
          <Badge variant="outline">未解锁</Badge>
        </div>
        <div className="mt-3 text-xs">仅监控，不会自动解锁付费槽位。</div>
      </div>
    );
  }
  const progress = Math.max(0, Math.min(task.progress, task.target > 0 ? task.target : task.progress));
  const percent = task.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / task.target) * 100))) : 0;
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">任务槽 {task.slotId} · #{task.taskId}</div>
          <div className="mt-1 line-clamp-2 font-medium">{task.title || `任务 #${task.taskId}`}</div>
        </div>
        <CyclicNoteStatusBadge status={task.status} received={task.received} unknown={!task.catalogKnown} />
      </div>
      {task.target > 0 && (
        <>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>进度</span>
            <span className="tabular-nums">{formatCount(progress)}/{formatCount(task.target)}</span>
          </div>
          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
          </div>
        </>
      )}
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {task.reward.length > 0 ? task.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

function CyclicNoteMilestoneCard({ milestone }: { milestone: CyclicNoteMilestone }) {
  const progress = Math.max(0, Math.min(milestone.progress, milestone.target > 0 ? milestone.target : milestone.progress));
  const percent = milestone.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / milestone.target) * 100))) : 0;
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="font-medium">积分 {formatCount(milestone.target)}</span>
        <CyclicNoteStatusBadge status={milestone.status} received={milestone.received} />
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>进度</span>
        <span className="tabular-nums">{formatCount(progress)}/{formatCount(milestone.target)}</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {milestone.reward.length > 0 ? milestone.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

function CyclicNoteStatusBadge({ status, received, unknown = false }: { status: PlanStatus; received: boolean; unknown?: boolean }) {
  if (unknown) return <Badge variant="destructive">未识别</Badge>;
  if (received) return <Badge variant="outline">已领取</Badge>;
  if (status === PlanStatus.READY) return <Badge variant="secondary">可领取</Badge>;
  if (status === PlanStatus.BLOCKED) return <Badge variant="destructive">阻塞</Badge>;
  if (status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">进行中</Badge>;
  return <Badge variant="outline">{planStatusLabel(status)}</Badge>;
}

function DessertMonitorPanel({ activity }: { activity?: DessertView }) {
  const phase = activity?.phase ?? 0;
  const readyTasks = activity?.valid ? activity.tasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const celebrityReady = activity?.valid && activity.celebrity?.status === PlanStatus.READY && !activity.celebrity.likedThisBatch;
  const actionable = readyTasks + (celebrityReady ? 1 : 0);

  return (
    <CollapsibleCard
      title={activity?.name || "香卉甜糕"}
      defaultOpen={false}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          {activity?.found && <Badge variant="outline">批次 {activity.batchId}</Badge>}
          {activity?.found && !activity.valid && <Badge variant="destructive">状态异常</Badge>}
          {actionable > 0 && <Badge variant="secondary">可处理 {actionable}</Badge>}
        </>
      }
    >
      {!activity?.observed ? (
        <>
          <EmptyState title="香卉甜糕状态尚未同步" detail="连接游戏后，会按活动类型和服务端时间自动发现当前批次。" />
          {activity?.runtime && <DessertRuntimePanel runtime={activity.runtime} />}
        </>
      ) : !activity.found ? (
        <>
          <EmptyState title="当前未发现香卉甜糕活动" detail="不会固定使用历史批次，也不会探测已结束活动。" />
          <DessertRuntimePanel runtime={activity.runtime} />
        </>
      ) : !activity.valid ? (
        <>
          <EmptyState title="香卉甜糕配置或状态异常" detail="自动操作已阻塞；请等待活动背包、模板和模式状态完整同步。" />
          <DessertObservationStatus activity={activity} />
          <DessertRuntimePanel runtime={activity.runtime} />
        </>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 lg:grid-cols-3 xl:grid-cols-6">
            <OverviewStat icon={<CalendarDays />} label="活动阶段" value={cyclicNotePhaseLabel(phase)} detail={dessertPhaseDetail(activity)} />
            <OverviewStat
              icon={<Sparkles />}
              label="活动体力"
              value={activity.bagObserved ? formatCount(activity.energyBalance) : "-"}
              detail={activity.energyItemId > 0 ? `${itemName(activity.energyItemId)} #${activity.energyItemId}` : "等待识别"}
            />
            <OverviewStat
              icon={<Play />}
              label="累计投放"
              value={activity.dropCountObserved ? formatCount(activity.dropCount) : "-"}
              detail={activity.dropCountObserved ? "服务端累计次数" : "等待同步"}
            />
            <OverviewStat
              icon={<Trophy />}
              label="累计积分"
              value={activity.totalScoreObserved ? formatCount(activity.totalScore) : "-"}
              detail={activity.totalScoreObserved ? "合成累计积分" : "等待同步"}
            />
            <OverviewStat
              icon={<Coins />}
              label="花糕币"
              value={activity.bagObserved ? formatCount(activity.currencyBalance) : "-"}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "等待识别"}
            />
            <OverviewStat
              icon={<Package />}
              label="未开箱"
              value={activity.bagObserved ? formatCount(activity.rewardBoxBalance) : "-"}
              detail="可在设置中开启单次安全开箱"
            />
          </div>

          {activity.description && (
            <div className="break-words rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>
          )}

          <DessertObservationStatus activity={activity} />
          <DessertRuntimePanel runtime={activity.runtime} />

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>合成模式</span>
              <Badge variant="outline">仅展示棋盘统计</Badge>
            </div>
            {activity.modes.length === 0 ? (
              <div className="p-3"><EmptyState title="模式状态尚未同步" /></div>
            ) : (
              <div className="grid grid-cols-1 gap-2 p-2 min-[480px]:grid-cols-2 xl:grid-cols-5">
                {activity.modes.map((mode) => <DessertModeCard key={mode.mode} mode={mode} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>固定任务</span>
              <Badge variant={readyTasks > 0 ? "secondary" : "outline"}>{readyTasks > 0 ? `可领取 ${readyTasks}` : `${activity.tasks.length} 项`}</Badge>
            </div>
            {activity.tasks.length === 0 ? (
              <div className="p-3"><EmptyState title={activity.taskRecordObserved ? "当前没有固定任务" : "任务记录尚未同步"} /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.tasks.map((task) => <DessertTaskCard key={`${activity.batchId}:${task.taskIndex}:${task.taskId}`} task={task} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>累计进度奖励</span>
              <Badge variant="outline">等待协议确认</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3"><EmptyState title="暂无累计进度奖励配置" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => <DessertMilestoneCard key={milestone.index} milestone={milestone} />)}
              </div>
            )}
          </section>

          <DessertCelebrityCard celebrity={activity.celebrity} />

          <div className="grid gap-2 sm:grid-cols-2">
            <div className="rounded-md border border-border/58 bg-muted/20 p-3 text-sm">
              <div className="font-medium">奖励箱</div>
              <div className="mt-1 text-xs text-muted-foreground">当前余额 {formatCount(activity.rewardBoxBalance)}；自动开箱默认关闭，开启后每次只开 1 个。</div>
            </div>
            <div className="rounded-md border border-border/58 bg-muted/20 p-3 text-sm">
              <div className="font-medium">合成游戏</div>
              <div className="mt-1 text-xs text-muted-foreground">可展示影子运行时诊断；连续轨迹门禁未通过，不会发送任何游戏 RPC。</div>
            </div>
          </div>

          {activity.items.length > 0 && (
            <div className="flex min-w-0 flex-wrap gap-1.5">
              {activity.items.map((item) => <ActivityItemChip key={item.itemId} item={item} />)}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

function DessertRuntimePanel({ runtime }: { runtime?: DessertRuntimeView }) {
  const observed = runtime?.observed ?? false;
  const shortHash = runtime?.boardHash ? truncateMiddle(runtime.boardHash, 8, 6) : "-";
  const waitingValue = runtime?.waiting
    ? `${formatCount(runtime.waitingRemainingMs > BigInt(0) ? runtime.waitingRemainingMs : BigInt(0))} ms`
    : "未等待";
  const waitingDetail = runtime?.waiting && runtime.frozenWaitingLevel > 0
    ? `冻结等级 ${runtime.frozenWaitingLevel}`
    : "waiting ball 未冻结";

  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>影子运行时</span>
        <span className="flex min-w-0 flex-wrap justify-end gap-1.5">
          <Badge variant={observed ? "outline" : "destructive"}>{observed ? "已观察" : "未观察"}</Badge>
          <Badge variant={runtime?.policyEnabled ? "secondary" : "outline"}>{runtime?.policyEnabled ? "策略已启用" : "策略未启用"}</Badge>
          <Badge variant="outline">{runtime?.shadowOnly ? "仅影子" : "实跑硬锁"}</Badge>
          <Badge variant={runtime?.liveEvidenceReady ? "outline" : "destructive"}>{runtime?.liveEvidenceReady ? "证据门禁已满足" : "证据门禁未满足"}</Badge>
          <Badge variant={runtime?.liveExecutionAllowed ? "destructive" : "outline"}>{runtime?.liveExecutionAllowed ? "执行门禁状态异常" : "执行硬锁"}</Badge>
          {runtime?.failureLocked && <Badge variant="destructive">失败锁定</Badge>}
        </span>
      </div>
      <div className="space-y-2 p-2">
        <div className="break-words rounded-md border border-amber-500/30 bg-amber-500/8 px-3 py-2 text-xs leading-5 text-muted-foreground dark:bg-amber-400/8">
          当前只展示影子诊断。证据门禁与执行门禁分别显示，gameStart / gameSync / gameOver 不会注册或发送。
        </div>
        {!runtime?.observed ? (
          <div className="space-y-1 rounded-md border border-border/55 bg-background/72 px-3 py-2 text-xs text-muted-foreground">
            <div>登录并同步活动后显示会话、权威棋盘版本和建议；未观察期间保持游戏 RPC 硬锁。</div>
            {runtime && <div>会话体力上限 {formatCount(runtime.maxSessionEnergy)}；最低保留 {formatCount(runtime.minEnergyReserve)}。</div>}
            {runtime?.blockedReason && <div className="break-words text-foreground">阻塞原因：{runtime.blockedReason}</div>}
          </div>
        ) : (
          <div className="grid min-w-0 grid-cols-1 gap-2 min-[420px]:grid-cols-2 lg:grid-cols-4">
            <DessertRuntimeMetric
              label="策略 / 模式"
              value={`${runtime.policyEnabled ? "已启用" : "未启用"} · 模式 ${runtime.mode || "-"}`}
              detail={runtime.shadowOnly ? "只计算建议，不执行" : "实跑仍被硬锁"}
            />
            <DessertRuntimeMetric
              label="会话 / 权威版本"
              value={`#${runtime.sessionEpoch.toString()} · r${runtime.authorityRevision.toString()}`}
              detail={runtime.batchId > 0 ? `批次 ${runtime.batchId}` : "批次未识别"}
            />
            <DessertRuntimeMetric
              label="棋盘"
              value={runtime.boardOwned ? "本会话拥有" : "未拥有"}
              detail={runtime.takeoverRequested ? "已请求评估接管" : "未请求接管"}
            />
            <DessertRuntimeMetric label="棋盘摘要" value={shortHash} detail="仅展示截断哈希" title={runtime.boardHash} mono />
            <DessertRuntimeMetric label="waiting ball" value={waitingValue} detail={waitingDetail} />
            <DessertRuntimeMetric
              label="会话体力预算"
              value={`已用 ${formatCount(runtime.sessionEnergyUsed)} / 上限 ${formatCount(runtime.maxSessionEnergy)}`}
              detail={`最低保留 ${formatCount(runtime.minEnergyReserve)}；当前构建不会实际扣除`}
            />
            <DessertRuntimeMetric
              label="证据 / 执行门禁"
              value={`${runtime.liveEvidenceReady ? "证据已满足" : "证据未满足"} · ${runtime.liveExecutionAllowed ? "状态异常" : "执行硬锁"}`}
              detail="执行硬锁独立于策略开关与证据状态"
            />
            <DessertRuntimeMetric
              label="影子建议"
              value={runtime.suggestion || "暂无建议"}
              detail="仅供诊断，不会转为 RPC"
              className="min-[420px]:col-span-2"
            />
            <DessertRuntimeMetric
              label="阻塞 / 锁定"
              value={runtime.blockedReason || (runtime.failureLocked ? "本会话已锁定" : "无额外原因")}
              detail={runtime.failureLocked ? "需重新登录或关闭后重新开启策略" : "连续轨迹门禁仍保持硬锁"}
              className="min-[420px]:col-span-2 lg:col-span-4"
            />
          </div>
        )}
      </div>
    </section>
  );
}

function DessertRuntimeMetric({
  label,
  value,
  detail,
  title,
  mono = false,
  className,
}: {
  label: string;
  value: string;
  detail: string;
  title?: string;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0 rounded-md border border-border/55 bg-background/72 px-3 py-2 text-xs", className)}>
      <div className="text-muted-foreground">{label}</div>
      <div className={cn("mt-1 min-w-0 break-words font-medium text-foreground", mono && "font-mono")} title={title}>{value}</div>
      <div className="mt-1 break-words text-muted-foreground">{detail}</div>
    </div>
  );
}

function DessertObservationStatus({ activity }: { activity: DessertView }) {
  const observations = [
    { label: "活动背包", ok: activity.bagObserved },
    { label: "扩展状态", ok: activity.extensionObserved && activity.extensionValid },
    { label: "模式地图", ok: activity.modeMapObserved && activity.modeMapValid },
    { label: "任务模板", ok: activity.taskGroupsObserved && activity.taskGroupsValid },
    { label: "任务记录", ok: activity.taskRecordObserved },
    { label: "进度回执", ok: activity.milestoneReceiptsObserved },
  ];
  return (
    <div className="flex min-w-0 flex-wrap gap-1.5">
      {observations.map((item) => (
        <Badge key={item.label} variant={item.ok ? "outline" : "destructive"}>{item.label} {item.ok ? "已同步" : "缺失"}</Badge>
      ))}
    </div>
  );
}

function DessertModeCard({ mode }: { mode: DessertModeView }) {
  const modeLabel = mode.mode === 1 ? "普通模式" : `${formatCount(mode.multiplier)} 倍模式`;
  const levelSummary = mode.levelCounts
    .filter((level) => level.count > 0)
    .map((level) => `${level.level}级×${formatCount(level.count)}`);
  const status = !mode.unlocked ? "未解锁" : mode.isRunning ? "进行中" : mode.observed ? "待开始" : "待同步";
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate font-medium">{modeLabel}</div>
          <div className="mt-1 text-xs text-muted-foreground">倍率 ×{formatCount(mode.multiplier)}</div>
        </div>
        <Badge variant={mode.isRunning ? "secondary" : "outline"}>{status}</Badge>
      </div>
      {!mode.unlocked && <div className="mt-3 text-xs text-muted-foreground">解锁积分 {formatCount(mode.unlockScore)}</div>}
      {mode.observed && (
        <div className="mt-3 grid grid-cols-2 gap-1.5 text-xs text-muted-foreground">
          <span>投放 {formatCount(mode.step)}</span>
          <span>得分 {formatCount(mode.score)}</span>
          <span>当前 {mode.currentId > 0 ? `${mode.currentId}级` : "-"}</span>
          <span>对象 {formatCount(mode.objectCount)}</span>
        </div>
      )}
      <div className="mt-2 break-words text-xs text-muted-foreground">
        {levelSummary.length > 0 ? levelSummary.join("、") : mode.observed ? "棋盘暂无对象" : "等待模式状态"}
      </div>
      {mode.rawGameStatus !== mode.effectiveGameStatus && (
        <div className="mt-2 text-xs text-muted-foreground">状态恢复 {mode.rawGameStatus} → {mode.effectiveGameStatus}</div>
      )}
    </div>
  );
}

function DessertTaskCard({ task }: { task: DessertTaskView }) {
  const progress = Math.max(0, task.progress);
  const percent = task.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / task.target) * 100))) : 0;
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">任务 {task.taskIndex}:{task.taskId}</div>
          <div className="mt-1 line-clamp-2 break-words font-medium">{task.title || `任务 #${task.taskId}`}</div>
        </div>
        <DessertTaskStatusBadge task={task} />
      </div>
      {task.target > 0 && (
        <>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>进度</span>
            <span className="tabular-nums">{formatCount(progress)}/{formatCount(task.target)}</span>
          </div>
          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
          </div>
        </>
      )}
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {task.reward.length > 0 ? task.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

function DessertTaskStatusBadge({ task }: { task: DessertTaskView }) {
  if (!task.catalogKnown) return <Badge variant="destructive">未识别</Badge>;
  if (task.received) return <Badge variant="outline">已领取</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可领取</Badge>;
  if (task.status === PlanStatus.BLOCKED) return <Badge variant="destructive">阻塞</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">进行中</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

function DessertMilestoneCard({ milestone }: { milestone: DessertMilestoneView }) {
  const progress = Math.max(0, milestone.progress);
  const percent = milestone.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / milestone.target) * 100))) : 0;
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="font-medium">积分 {formatCount(milestone.target)}</span>
        <Badge variant="outline">{milestone.received ? "已领取" : "等待协议确认"}</Badge>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>进度</span>
        <span className="tabular-nums">{formatCount(progress)}/{formatCount(milestone.target)}</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {milestone.reward.length > 0 ? milestone.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

function DessertCelebrityCard({ celebrity }: { celebrity?: DessertCelebrityLikeView }) {
  const label = !celebrity?.observed
    ? "待同步"
    : celebrity.likedThisBatch
      ? "本期已点赞"
      : celebrity.status === PlanStatus.READY
        ? "可免费点赞"
        : celebrity.status === PlanStatus.BLOCKED
          ? "已阻塞"
          : "待同步";
  const variant = celebrity?.status === PlanStatus.BLOCKED ? "destructive" : celebrity?.status === PlanStatus.READY && !celebrity.likedThisBatch ? "secondary" : "outline";
  return (
    <section className="min-w-0 rounded-md border border-border/58 bg-white/34 p-3 dark:bg-white/5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="font-medium">名人榜免费点赞</div>
          <div className="mt-1 text-xs text-muted-foreground">
            {celebrity?.rankingObserved ? `榜单共 ${formatCount(celebrity.rankingCount)} 条，仅展示数量` : "榜单尚未完成受控同步"}
          </div>
        </div>
        <Badge variant={variant}>{label}</Badge>
      </div>
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {celebrity?.reward?.length ? celebrity.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>等待配置确认</span>}
      </div>
    </section>
  );
}

function ActivityItemChip({ item, compact = false }: { item: ActivityItem; compact?: boolean }) {
  const label = item.itemName || itemName(item.itemId);
  return (
    <span className={cn("inline-flex max-w-full items-center gap-1 rounded border border-border/58 bg-white/52 dark:bg-white/5", compact ? "px-1.5 py-0.5" : "px-2 py-1 text-xs")}>
      <span className="min-w-0 truncate" title={item.itemId > 0 ? `${label} #${item.itemId}` : label}>{label}</span>
      <span className="shrink-0 font-semibold tabular-nums">×{formatCount(item.count)}</span>
    </span>
  );
}

function TaskOrderMonitorPanel({
  tasks,
  statistics,
}: {
  tasks: PendingTaskView[];
  statistics?: OrderStatisticsView;
}) {
  const monitoredTasks = useMemo(() => [...tasks].sort(comparePendingTasks), [tasks]);
  const orderTasks = useMemo(() => monitoredTasks.filter(isOrderPendingTask), [monitoredTasks]);
  const taskItems = useMemo(() => monitoredTasks.filter((task) => !isOrderPendingTask(task)), [monitoredTasks]);
  const readyCount = monitoredTasks.filter((task) => task.status === PlanStatus.READY && !pendingTaskCooling(task)).length;
  const coolingCount = monitoredTasks.filter(pendingTaskCooling).length;
  const shortageCount = monitoredTasks.filter(pendingTaskHasShortage).length;
  const blockedCount = monitoredTasks.filter(pendingTaskBlocked).length;
  const missingItemCount = monitoredTasks.reduce((sum, task) => sum + task.requirements.filter((req) => req.missing > 0).length, 0);
  const missingSummary = useMemo(() => requirementShortageSummary(monitoredTasks), [monitoredTasks]);
  const orderStats = orderStatisticItems(statistics);

  return (
    <CollapsibleCard
      title="任务/订单监控"
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant="secondary">总计 {monitoredTasks.length}</Badge>
          {readyCount > 0 && <Badge variant="secondary">可处理 {readyCount}</Badge>}
          {coolingCount > 0 && <Badge variant="outline">冷却 {coolingCount}</Badge>}
          {shortageCount > 0 && <Badge variant="outline">缺口 {shortageCount}</Badge>}
          {blockedCount > 0 && <Badge variant="destructive">阻塞 {blockedCount}</Badge>}
        </>
      }
    >
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat icon={<ListChecks />} label="任务" value={taskItems.length} detail={taskMonitorDetail(taskItems)} />
        <OverviewStat icon={<Package />} label="订单" value={orderTasks.length} detail={taskMonitorDetail(orderTasks)} />
        <OverviewStat icon={<AlertTriangle />} label="缺项" value={missingItemCount} detail={missingSummary || "暂无资源缺口"} />
        <OverviewStat
          icon={<Check />}
          label="订单完成"
          value={statistics?.observed ? orderStats.reduce((sum, item) => sum + item.value, 0) : "-"}
          detail={statistics?.observed ? `更新 ${formatUnixTime(statistics.updatedAtMs)}` : "未同步"}
        />
      </div>

      {statistics?.observed && (
        <div className="dark-scrollbar flex gap-2 overflow-x-auto rounded-md border border-border/70 bg-muted/20 p-2">
          {orderStats.map((item) => (
            <div key={item.label} className="flex min-w-[5.5rem] shrink-0 items-center justify-between gap-3 rounded bg-background/70 px-3 py-2 text-sm sm:min-w-24">
              <span className="text-muted-foreground">{item.label}</span>
              <span className="font-semibold tabular-nums">{formatCount(item.value)}</span>
            </div>
          ))}
        </div>
      )}

      {monitoredTasks.length === 0 ? (
        <EmptyState title="暂无任务/订单快照" />
      ) : (
        <div className="grid gap-3 xl:grid-cols-2">
          <PendingTaskGroup title="任务" tasks={taskItems} emptyText="暂无任务待监控" />
          <PendingTaskGroup title="订单" tasks={orderTasks} emptyText="暂无订单待监控" />
        </div>
      )}
    </CollapsibleCard>
  );
}

function RuntimeCompletionGroup({ title, items, emptyText }: { title: string; items: RuntimeActionTotal[]; emptyText: string }) {
  const total = sumRuntimeActionTotals(items);
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{formatCount(total)}</Badge>
      </div>
      <div className="p-2">
        {items.length === 0 ? (
          <EmptyState title={emptyText} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {items.map((item) => (
              <span key={item.key} className="inline-flex min-h-8 items-center gap-2 rounded border border-border/58 bg-background/72 px-3 py-1 text-sm">
                <span className="text-muted-foreground">{item.label || item.key}</span>
                <span className="font-semibold tabular-nums">{formatCount(item.count)}</span>
              </span>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function PendingTaskGroup({ title, tasks, emptyText }: { title: string; tasks: PendingTaskView[]; emptyText: string }) {
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex h-9 items-center justify-between gap-2 bg-secondary/55 px-3 text-sm font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{tasks.length}</Badge>
      </div>
      {tasks.length === 0 ? (
        <div className="p-3">
          <EmptyState title={emptyText} />
        </div>
      ) : (
        <div className="dark-scrollbar max-h-[300px] divide-y divide-border/70 overflow-y-auto sm:max-h-[360px]">
          {tasks.map((task, index) => (
            <PendingTaskRow key={`${task.category}-${task.id}-${index}`} task={task} />
          ))}
        </div>
      )}
    </section>
  );
}

function PendingTaskRow({ task }: { task: PendingTaskView }) {
  return (
    <div className="min-h-[4.5rem] px-3 py-2.5">
      <div className="flex items-start gap-3">
        <PendingTaskStatusBadge task={task} />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-sm">
            <span className="shrink-0 text-xs text-muted-foreground">{pendingTaskCategoryLabel(task.category)}</span>
            <span className="min-w-0 truncate font-medium">{task.title || `#${task.id}`}</span>
            {task.id && <span className="shrink-0 font-mono text-xs text-muted-foreground">#{task.id}</span>}
            {taskProgressLabel(task) && <span className="shrink-0 text-xs text-muted-foreground">{taskProgressLabel(task)}</span>}
          </div>
          {task.target > 0 && (
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary" style={{ width: `${pendingTaskProgressPercent(task)}%` }} />
            </div>
          )}
          {task.requirements.length > 0 && <RequirementChips requirements={task.requirements} />}
        </div>
      </div>
    </div>
  );
}

function PendingTaskStatusBadge({ task }: { task: PendingTaskView }) {
  if (pendingTaskCooling(task)) return <Badge variant="outline">冷却</Badge>;
  if (pendingTaskBlocked(task)) return <Badge variant="destructive">阻塞</Badge>;
  if (pendingTaskHasShortage(task)) return <Badge variant="destructive">缺口</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可处理</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">同步</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

function RequirementChips({ requirements }: { requirements: RequirementView[] }) {
  const visible = requirements.slice(0, 4);
  return (
    <div className="flex flex-wrap gap-1.5">
      {visible.map((req, index) => (
        <span
          key={`${req.itemId}-${req.required}-${req.owned}-${index}`}
          className={cn(
            "inline-flex min-h-6 max-w-full items-center gap-1 rounded border px-2 py-0.5 text-xs",
            req.missing > 0 ? "border-destructive/35 bg-destructive/10 text-destructive" : "border-border/58 bg-white/42 text-muted-foreground dark:bg-white/5",
          )}
          title={req.blockedReasons.join("、")}
        >
          <span className="truncate">{req.itemName || itemName(req.itemId)}</span>
          <span className="shrink-0 tabular-nums">
            {formatCount(req.owned)}/{formatCount(req.required)}
          </span>
        </span>
      ))}
      {requirements.length > visible.length && (
        <span className="inline-flex min-h-6 items-center rounded border border-border/58 bg-white/42 px-2 py-0.5 text-xs text-muted-foreground dark:bg-white/5">
          +{requirements.length - visible.length}
        </span>
      )}
    </div>
  );
}

function LandWarehouseMonitorPanel({ lands, ledger }: { lands: LandView[]; ledger?: InventoryLedgerView }) {
  const openedCount = lands.filter((land) => land.landStatus === "opened").length;
  const inventoryCount = (ledger?.items ?? []).filter((item) => item.owned > 0 || item.allocated > 0).length;
  return (
    <CollapsibleCard
      title="土地/仓库监控"
      contentClassName="grid gap-3 xl:grid-cols-2"
      actions={
        <>
          <Badge variant="secondary">土地 {openedCount}</Badge>
          <Badge variant="secondary">仓库 {inventoryCount}</Badge>
        </>
      }
    >
      <LandStatusPanel lands={lands} />
      <InventoryLedgerPanel ledger={ledger} />
    </CollapsibleCard>
  );
}

function LandStatusPanel({ lands }: { lands: LandView[] }) {
  const visibleLands = useMemo(
    () => lands.filter((land) => land.landStatus === "opened").sort((a, b) => a.landId - b.landId),
    [lands],
  );
  const recommendationStats = useMemo(() => {
    const stats = new Map<string, number>();
    for (const land of visibleLands) {
      stats.set(land.recommendation || "unknown", (stats.get(land.recommendation || "unknown") ?? 0) + 1);
    }
    const visibleKeys = new Set(["harvest", "plant", "water", "wait"]);
    return [...stats.entries()].filter(([key]) => visibleKeys.has(key)).sort((a, b) => b[1] - a[1]);
  }, [visibleLands]);
  const openedCount = lands.filter((land) => land.landStatus === "opened").length;
  const unopenedCount = lands.filter((land) => land.landStatus === "unopened").length;
  const lockedCount = lands.filter((land) => land.landStatus === "locked").length;

  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>土地</span>
        <div className="flex flex-wrap justify-end gap-1.5">
          <Badge variant="secondary">已开 {openedCount}</Badge>
          {unopenedCount > 0 && <Badge variant="outline">未开 {unopenedCount}</Badge>}
          {lockedCount > 0 && <Badge variant="outline">锁定 {lockedCount}</Badge>}
        </div>
      </div>
      <div className="p-3">
        {lands.length === 0 ? (
          <EmptyState title="暂无土地快照" />
        ) : visibleLands.length === 0 ? (
          <EmptyState title="暂无已开放土地" />
        ) : (
          <div className="space-y-4">
            <div className="flex flex-wrap gap-2">
              {recommendationStats.map(([key, count]) => (
                <Badge key={key} variant="outline">
                  {recommendationLabel(key)} {count}
                </Badge>
              ))}
            </div>
            <div className="dark-scrollbar grid max-h-[440px] gap-2 overflow-y-auto pr-0.5 sm:h-[560px] sm:max-h-none sm:grid-cols-2 sm:pr-1">
              {visibleLands.map((land) => (
                <LandTile key={land.landId} land={land} />
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

function LandTile({ land }: { land: LandView }) {
  const planted = land.flowerId > 0;
  const status = land.landStatus || (land.observed ? "opened" : "unknown");
  const recommendation = recommendationLabel(land.recommendation);
  const timing = landTimingLabel(land, status);
  return (
    <div
      className={cn(
        "min-h-[78px] rounded-md border border-border/58 bg-white/58 p-2 shadow-sm transition-colors dark:bg-white/6",
        land.recommendation === "harvest" && "border-primary/50 bg-primary/8",
        land.recommendation === "water" && "border-sky-300/70 bg-sky-50/72 dark:bg-sky-500/10",
        land.recommendation === "plant" && "border-amber-300/70 bg-amber-50/76 dark:bg-amber-400/10",
        !land.observed && "opacity-70",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="font-mono text-sm font-medium">{landDisplayName(land.landId)}</div>
        </div>
        <Badge variant={land.recommendation === "harvest" ? "secondary" : "outline"} className="h-5 px-1.5 text-[11px]">
          {recommendation}
        </Badge>
      </div>
      <div className="mt-1 truncate text-sm">{planted ? itemName(land.flowerId) : "空地"}</div>
      <div className="mt-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="truncate">
          {land.lvl ? `${land.lvl}级` : "-"}
          {planted ? ` · 收${land.harvestCnt || 0}` : ""}
        </span>
        <span className="shrink-0">{timing}</span>
      </div>
    </div>
  );
}

function warehouseCategoryForItem(item: InventoryLedgerItem): WarehouseCategory {
  const id = item.itemId;
  if (id >= 23000 && id < 24000) return "flower";
  if (id >= 300000 && id < 400000) return "art";
  return "item";
}

function warehouseCategoryLabel(category: WarehouseCategory) {
  return WAREHOUSE_CATEGORIES.find((entry) => entry.id === category)?.label ?? "仓库";
}

function warehouseSearchPlaceholder(category: WarehouseCategory) {
  switch (category) {
    case "flower":
      return "搜索花朵或 ID";
    case "art":
      return "搜索花艺或 ID";
    case "item":
      return "搜索道具或 ID";
  }
}

function InventoryLedgerPanel({ ledger }: { ledger?: InventoryLedgerView }) {
  const [inventoryQuery, setInventoryQuery] = useState("");
  const [warehouseCategory, setWarehouseCategory] = useState<WarehouseCategory>("flower");
  const inventoryItems = useMemo(() => {
    return [...(ledger?.items ?? [])]
      .filter((item) => item.owned > 0 || item.allocated > 0)
      .sort((a, b) => b.owned - a.owned || b.allocated - a.allocated || a.itemId - b.itemId);
  }, [ledger]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<WarehouseCategory, number>();
    for (const category of WAREHOUSE_CATEGORIES) counts.set(category.id, 0);
    for (const item of inventoryItems) {
      const category = warehouseCategoryForItem(item);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [inventoryItems]);
  const categoryItems = useMemo(() => {
    return inventoryItems.filter((item) => warehouseCategoryForItem(item) === warehouseCategory);
  }, [inventoryItems, warehouseCategory]);
  const visibleItems = useMemo(() => {
    const query = inventoryQuery.trim().toLowerCase();
    if (!query) return categoryItems;
    return categoryItems.filter((item) => {
      const name = item.itemName || itemName(item.itemId);
      return name.toLowerCase().includes(query) || String(item.itemId).includes(query);
    });
  }, [categoryItems, inventoryQuery]);
  const categoryLabel = warehouseCategoryLabel(warehouseCategory);

  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>仓库</span>
        {inventoryItems.length > 0 && (
          <div className="flex flex-wrap justify-end gap-1.5">
            <Badge variant="secondary">种类 {inventoryItems.length}</Badge>
            {inventoryQuery.trim() && <Badge variant="outline">匹配 {visibleItems.length}</Badge>}
          </div>
        )}
      </div>
      <div className="p-3">
        {inventoryItems.length > 0 && (
          <div className="mb-3 grid gap-2 lg:grid-cols-[minmax(296px,1fr)_minmax(150px,0.65fr)] lg:items-center">
            <div className="grid min-w-0 grid-cols-3 rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5">
              {WAREHOUSE_CATEGORIES.map((category) => (
                <button
                  key={category.id}
                  type="button"
                  aria-pressed={warehouseCategory === category.id}
                  onClick={() => {
                    setWarehouseCategory(category.id);
                    setInventoryQuery("");
                  }}
                  className={cn(
                    "flex h-8 min-w-0 items-center justify-center gap-1.5 rounded px-2 text-xs font-medium transition-colors",
                    warehouseCategory === category.id
                      ? "bg-white text-foreground shadow-sm dark:bg-muted"
                      : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
                  )}
                >
                  <span className="shrink-0 [&_svg]:size-3.5">{category.icon}</span>
                  <span className="shrink-0 whitespace-nowrap">{category.label}</span>
                  <span className="shrink-0 tabular-nums text-muted-foreground">{categoryCounts.get(category.id) ?? 0}</span>
                </button>
              ))}
            </div>
            <div className="relative min-w-0">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={inventoryQuery}
                onChange={(event) => setInventoryQuery(event.target.value)}
                placeholder={warehouseSearchPlaceholder(warehouseCategory)}
                className="h-10 rounded-md pl-9"
              />
            </div>
          </div>
        )}
        {inventoryItems.length === 0 ? (
          <EmptyState title="暂无仓库数据" />
        ) : categoryItems.length === 0 ? (
          <EmptyState title={`暂无${categoryLabel}`} />
        ) : visibleItems.length === 0 ? (
          <EmptyState title={`没有匹配${categoryLabel}`} detail="换个名称或 ID 再试试" />
        ) : (
          <div className="dark-scrollbar max-h-[440px] overflow-y-auto rounded-md border border-border/58 bg-white/42 sm:h-[560px] sm:max-h-none dark:bg-white/5">
            <Table>
              <TableHeader className="sticky top-0 z-10 bg-card/92 shadow-[0_1px_0_0_var(--border)] backdrop-blur-xl">
                <TableRow className="hover:bg-transparent">
                  <TableHead className="h-9 text-xs">名称</TableHead>
                  <TableHead className="h-9 text-xs">数量</TableHead>
                  <TableHead className="h-9 text-xs">预留</TableHead>
                  <TableHead className="h-9 text-xs">可用</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visibleItems.map((item) => (
                  <TableRow key={item.itemId} className="h-10 hover:bg-muted/35">
                    <TableCell className="min-w-0">
                      <div className="flex min-w-0 items-baseline gap-2">
                        <span className="truncate font-medium">{item.itemName || itemName(item.itemId)}</span>
                        <span className="shrink-0 text-xs text-muted-foreground">{item.itemId}</span>
                      </div>
                    </TableCell>
                    <TableCell>{item.owned}</TableCell>
                    <TableCell className={cn(item.allocated > 0 && "text-primary")}>{item.allocated}</TableCell>
                    <TableCell className={cn(item.available < 0 && "text-destructive")}>{item.available}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </section>
  );
}

function OperationPanel({ operations }: { operations: PlannedOperation[] }) {
  const queueOperations = operations.filter(isQueueOperation);
  const farmOperations = queueOperations.filter((operation) => operation.lane === ExecutionLane.FARM);
  const sideOperations = queueOperations.filter((operation) => operation.lane !== ExecutionLane.FARM);
  return (
    <CollapsibleCard title="执行队列" actions={<Badge variant="secondary">{queueOperations.length}</Badge>}>
      <div className="max-h-[360px] overflow-hidden rounded-md border border-border/58 bg-white/34 md:h-[220px] md:max-h-none dark:bg-white/5">
        {queueOperations.length === 0 ? (
          <div className="flex min-h-28 items-center justify-center px-3 text-sm text-muted-foreground md:h-full md:min-h-0">当前无可执行操作</div>
        ) : (
          <div className="grid min-h-0 md:h-full md:grid-cols-2">
            <OperationLaneSection title="种植通道" operations={farmOperations} emptyText="暂无收获、播种或浇水" />
            <OperationLaneSection title="其他通道" operations={sideOperations} emptyText="暂无任务、订单或活动操作" />
          </div>
        )}
      </div>
    </CollapsibleCard>
  );
}

function OperationLaneSection({ title, operations, emptyText }: { title: string; operations: PlannedOperation[]; emptyText: string }) {
  return (
    <section className="flex min-h-0 min-w-0 flex-col border-b border-border/58 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <div className="flex h-8 items-center justify-between bg-secondary/55 px-3 text-xs font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{operations.length}</Badge>
      </div>
      {operations.length === 0 ? (
        <div className="flex min-h-14 flex-1 items-center px-3 py-3 text-sm text-muted-foreground md:min-h-0">{emptyText}</div>
      ) : (
        <div className="dark-scrollbar min-h-0 flex-1 divide-y divide-border/70 overflow-auto">
          {operations.map((operation, index) => (
            <OperationRow key={operation.operationId || `${operation.rpc}-${index}`} operation={operation} />
          ))}
        </div>
      )}
    </section>
  );
}

function OperationRow({ operation }: { operation: PlannedOperation }) {
  const target = operationTargetLabel(operation);
  const cost = operationCostLabel(operation);
  const note = operationNoteLabel(operation);
  return (
    <div className="flex min-h-12 items-center gap-3 px-3 py-2" title={[operation.rpc, operation.domain, operation.reason].filter(Boolean).join(" · ")}>
      <div className="shrink-0">
        <OperationStatusBadge operation={operation} />
      </div>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
        <span className="font-medium">{operationTitle(operation)}</span>
        {target && <span className="text-muted-foreground">{target}</span>}
        {cost && <span className="text-muted-foreground">{cost}</span>}
        {note && <span className="text-muted-foreground">{note}</span>}
      </div>
    </div>
  );
}

function isQueueOperation(operation: PlannedOperation) {
  return isRunnableOperation(operation) || isOperationCooling(operation);
}

function isRunnableOperation(operation: PlannedOperation) {
  return (
    operation.executable &&
    !operation.syncOnly &&
    operation.status !== PlanStatus.ADAPTER_MISSING &&
    operation.status !== PlanStatus.BLOCKED &&
    operation.blockedReasons.length === 0
  );
}

function PolicyPanel({
  policy,
  snapshot,
  loading,
  saving,
  message,
  onPolicyChange,
  onSave,
}: {
  policy: Policy | null;
  snapshot: GetSnapshotResponse | null;
  loading: boolean;
  saving: boolean;
  message: string;
  onPolicyChange: (policy: Policy | null) => void;
  onSave: () => void;
}) {
  const [activeTab, setActiveTab] = useState<PolicyTabId>("basic");
  useEffect(() => {
    if (!POLICY_TABS.some((tab) => tab.id === activeTab)) {
      setActiveTab("basic");
    }
  }, [activeTab]);
  const plant = policy?.plant;
  const planting = plant?.planting;
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
  const zoo = basic?.zoo;
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
  const customerOrdersObserved = snapshot?.observedNamespaces.includes("109") ?? false;
  const customerOrderCount = snapshot?.pendingTasks.filter((taskItem) => taskItem.category === "顾客订单").length ?? 0;
  const customerOrderSyncLabel = snapshot
    ? customerOrdersObserved
      ? `已同步 109，当前 ${customerOrderCount} 单`
      : "未同步 109"
    : "状态未加载";

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
  const updateZoo = (patch: Partial<ZooPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.zoo ?? create(ZooPolicySchema);
    updateBasic({ zoo: { ...current, ...patch } });
  };
  const updatePlanting = (patch: Partial<PlantingPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.planting ?? create(PlantingPolicySchema);
    updatePlant({ planting: { ...current, ...patch } });
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
  const updateActivityIntParam = (moduleID: string, key: string, value: number, fallback: number) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { intParams: { ...current.intParams, [key]: safeNumberToBigInt(value, fallback) } });
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
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <CardTitle>策略</CardTitle>
          <Button type="button" size="sm" className="w-full sm:w-auto" onClick={onSave} disabled={saving}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            {saving ? "保存中" : "保存"}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {message && <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm">{message}</div>}

        <section className="space-y-3">
          <SectionTitle icon={<ShieldCheck />}>运行参数</SectionTitle>
          <div className="grid gap-2 sm:grid-cols-2">
            <NumberRow label="决策间隔" value={policy.decisionIntervalSeconds || 4} min={1} onChange={(value) => updatePolicy({ decisionIntervalSeconds: value })} />
          </div>
        </section>

        <div className="grid grid-cols-5 gap-1 rounded-md border border-border/70 bg-muted/20 p-1">
          {POLICY_TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded px-1 text-xs font-medium transition-colors sm:min-h-9 sm:flex-row sm:gap-2 sm:px-3 sm:text-sm [&_svg]:size-4",
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
            <PolicyGroup title="土地与种植" icon={<Sprout />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动种植" checked={planting?.autoEnabled ?? false} onChange={(checked) => updatePlanting({ autoEnabled: checked })} />
                <ToggleRow label="自动收获" checked={planting?.autoHarvestEnabled ?? false} onChange={(checked) => updatePlanting({ autoHarvestEnabled: checked })} />
                <ToggleRow label="解锁土地" checked={planting?.autoUnlockLand ?? false} onChange={(checked) => updatePlanting({ autoUnlockLand: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频加速" checked={planting?.videoSpeedUpEnabled ?? false} onChange={(checked) => updatePlanting({ videoSpeedUpEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />}
                <ToggleRow label="使用加速券" checked={planting?.useSpeedUpTicket ?? false} onChange={(checked) => updatePlanting({ useSpeedUpTicket: checked })} />
                <NumberRow label="加速券上限" value={planting?.speedUpTicketMax || 0} min={0} onChange={(value) => updatePlanting({ speedUpTicketMax: value })} />
                <NumberRow label="保留水滴" value={planting?.minWaterDrops || 0} min={0} onChange={(value) => updatePlanting({ minWaterDrops: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="自主补种" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <SegmentedRow
                  label="补种范围"
                  value={planting?.autoReplantMode || SelectionMode.ALL}
                  options={AUTO_REPLANT_SELECTION_MODE_OPTIONS}
                  onChange={(value) => updatePlanting({ autoReplantMode: value })}
                />
                <FlowerMultiSelectRow
                  label={(planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE ? "排除补种" : "指定补种"}
                  value={(planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE ? planting?.autoReplantExcludeFlowerIds ?? [] : planting?.autoReplantFlowerIds ?? []}
                  plantableFlowers={snapshot?.plantableFlowers ?? []}
                  synced={Boolean(snapshot)}
                  onChange={(value) =>
                    (planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE
                      ? updatePlanting({ autoReplantExcludeFlowerIds: value })
                      : updatePlanting({ autoReplantFlowerIds: value })
                  }
                />
              </div>
              <DemandPriorityEditor value={planting?.demandPriority ?? {}} onChange={(demandPriority) => updatePlanting({ demandPriority })} />
            </PolicyGroup>

            <PolicyGroup title="培育配置" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动培育" checked={cultivate?.enabled ?? false} onChange={(checked) => updateCultivate({ enabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频加速培育" checked={cultivate?.videoSpeedUpEnabled ?? false} onChange={(checked) => updateCultivate({ videoSpeedUpEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />}
                <ToggleRow label="鲜花升级" checked={cultivate?.upgradeEnabled ?? false} onChange={(checked) => updateCultivate({ upgradeEnabled: checked })} />
                <NumberRow label="目标等级" value={cultivate?.targetLevel || 20} min={1} onChange={(value) => updateCultivate({ targetLevel: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="基础配置" icon={<ShieldCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="礼仪分监控" checked={reputation?.enabled ?? false} onChange={(checked) => updateReputation({ enabled: checked })} />
                <NumberRow label="礼仪分阈值" value={reputation?.threshold || 80} min={0} onChange={(value) => updateReputation({ threshold: value })} />
                <ToggleRow
                  label="被挤号后自动重登"
                  checked={basic?.displacedSessionReloginEnabled ?? false}
                  onChange={(checked) => updateBasic({ displacedSessionReloginEnabled: checked })}
                />
                <NumberRow
                  label="自动重登间隔（秒）"
                  value={basic?.reconnectIntervalSeconds || 300}
                  min={1}
                  max={86400}
                  disabled={!basic?.displacedSessionReloginEnabled}
                  onChange={(value) => updateBasic({ reconnectIntervalSeconds: value })}
                />
                <p className="px-1 text-xs leading-5 text-muted-foreground sm:col-span-2">
                  {basic?.displacedSessionReloginEnabled
                    ? "已启用：明确检测到异地登录或被挤下线后，将等待上述时间再自动登录。主动退出和普通业务失败不会触发。"
                    : "默认关闭。开启后仅在明确检测到异地登录或被挤下线时自动重登；关闭时不会自动登录。"}
                </p>
              </div>
            </PolicyGroup>

            <PolicyGroup title="任务与剧情" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="主线任务" checked={task?.mainEnabled ?? false} onChange={(checked) => updateBasicTask({ mainEnabled: checked })} />
                <ToggleRow label="每日任务" checked={task?.dailyEnabled ?? false} onChange={(checked) => updateBasicTask({ dailyEnabled: checked })} />
                <ToggleRow label="每周任务" checked={task?.weeklyEnabled ?? false} onChange={(checked) => updateBasicTask({ weeklyEnabled: checked })} />
                <ToggleRow label="主线剧情" checked={task?.storyEnabled ?? false} onChange={(checked) => updateBasicTask({ storyEnabled: checked })} />
                <ToggleRow label="成就任务" checked={task?.achievementEnabled ?? false} onChange={(checked) => updateBasicTask({ achievementEnabled: checked })} />
                <ToggleRow label="地图随机事件" checked={basic?.mapEventEnabled ?? false} onChange={(checked) => updateBasic({ mapEventEnabled: checked })} />
              </div>
            </PolicyGroup>

          </div>
        )}

        {activeTab === "other" && (
          <div className="space-y-4">
            <PolicyGroup title="邮件、福利、签到" icon={<BadgeCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="邮件" checked={basic?.mailEnabled ?? false} onChange={(checked) => updateBasic({ mailEnabled: checked })} />
                <ToggleRow label="水车水滴" checked={basic?.waterwheelEnabled ?? false} onChange={(checked) => updateBasic({ waterwheelEnabled: checked })} />
                <ToggleRow label="限时水滴" checked={basic?.freeWaterEnabled ?? false} onChange={(checked) => updateBasic({ freeWaterEnabled: checked })} />
                <NumberRow label="水滴领取阈值" value={basic?.waterClaimThreshold || 0} min={0} onChange={(value) => updateBasic({ waterClaimThreshold: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="双倍金币" checked={benefit?.doubleCoinEnabled ?? false} onChange={(checked) => updateBenefit({ doubleCoinEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />}
                <ToggleRow label="福利宝箱" checked={benefit?.boxEnabled ?? false} onChange={(checked) => updateBenefit({ boxEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="分享奖励" checked={benefit?.shareRewardEnabled ?? false} onChange={(checked) => updateBenefit({ shareRewardEnabled: checked })} status={SETTING_STATUS.syncOnly} />}
                <ToggleRow label="防骗宝箱" checked={benefit?.antiScamBoxEnabled ?? false} onChange={(checked) => updateBenefit({ antiScamBoxEnabled: checked })} />
                <ToggleRow label="防诈骗签到奖励" checked={sign?.dailyEnabled ?? false} onChange={(checked) => updateSign({ dailyEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="自动补签" checked={sign?.patchEnabled ?? false} onChange={(checked) => updateSign({ patchEnabled: checked })} status={SETTING_STATUS.adapterMissing} />}
                <ToggleRow label="成长之路" checked={basic?.roadGrowRewardEnabled ?? false} onChange={(checked) => updateBasic({ roadGrowRewardEnabled: checked })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="珍珠" icon={<Gem />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="免费珍珠" checked={pearl?.freeEnabled ?? false} onChange={(checked) => updatePearl({ freeEnabled: checked })} />
                <ToggleRow label="安全雇佣劳工" checked={pearl?.autoHireEnabled ?? false} onChange={(checked) => updatePearl({ autoHireEnabled: checked })} />
                <NumberRow label="雇佣等级上限（0=不限）" value={pearl?.maxHireLevel || 0} min={0} onChange={(value) => updatePearl({ maxHireLevel: value })} />
                <NumberRow label="同时在岗上限（0=关闭）" value={pearl?.maxHireTicketUsage || 0} min={0} onChange={(value) => updatePearl({ maxHireTicketUsage: value })} />
                <ToggleRow label="自动开珍珠" checked={pearl?.drawEnabled ?? false} onChange={(checked) => updatePearl({ drawEnabled: checked })} />
                <ToggleRow label="开启防身" checked={pearl?.protectEnabled ?? false} onChange={(checked) => updatePearl({ protectEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="买雇佣书" checked={pearl?.autoBuyHireTicket ?? false} onChange={(checked) => updatePearl({ autoBuyHireTicket: checked })} status={SETTING_STATUS.adapterMissing} />
                    <BigIntNumberRow label="元宝上限" value={pearl?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updatePearl({ maxSpendDiamond: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            <PolicyGroup title="商城" icon={<ShoppingBag />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="视频礼包" checked={shop?.videoFreeGiftEnabled ?? false} onChange={(checked) => updateShop({ videoFreeGiftEnabled: checked })} />
                <ToggleRow label="材料商店" checked={cultivateShop?.autoBuy ?? false} onChange={(checked) => updateCultivateShop({ autoBuy: checked })} />
                <BigIntNumberRow label="材料金币上限" value={cultivateShop?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateCultivateShop({ maxSpendGold: value })} />
                <IntListRow label="材料商品 ID" value={cultivateShop?.itemIds ?? []} onChange={(value) => updateCultivateShop({ itemIds: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="VIP 商店" checked={vipShop?.autoBuy ?? false} onChange={(checked) => updateVipShop({ autoBuy: checked })} status={SETTING_STATUS.adapterMissing} />
                    <BigIntNumberRow label="VIP 元宝上限" value={vipShop?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendDiamond: value })} />
                    <BigIntNumberRow label="VIP 花坊币上限" value={vipShop?.maxSpendFloralCoin ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendFloralCoin: value })} />
                    <IntListRow label="VIP 商品 ID" value={vipShop?.itemIds ?? []} onChange={(value) => updateVipShop({ itemIds: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            <PolicyGroup title="宠物" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="宠物模块" checked={zoo?.enabled ?? false} onChange={(checked) => updateZoo({ enabled: checked })} />
                <ToggleRow label="宠物外出/事件处理" checked={zoo?.autoEventEnabled ?? false} onChange={(checked) => updateZoo({ autoEventEnabled: checked })} />
                <ToggleRow label="自动补充食盆" checked={zoo?.autoFeed ?? false} onChange={(checked) => updateZoo({ autoFeed: checked })} />
                <ToggleRow label="自动互动" checked={zoo?.autoStroke ?? false} onChange={(checked) => updateZoo({ autoStroke: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="购买饲料" checked={zoo?.autoBuyFood ?? false} onChange={(checked) => updateZoo({ autoBuyFood: checked })} status={SETTING_STATUS.adapterMissing} />
                    <BigIntNumberRow label="宠物金币上限" value={zoo?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateZoo({ maxSpendGold: value })} />
                    <BigIntNumberRow label="宠物元宝上限" value={zoo?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateZoo({ maxSpendDiamond: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            {SHOW_UNSUPPORTED_SETTINGS && (
              <>
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

                <PolicyGroup title="鲜花摊位" icon={<ShoppingBag />}>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <ToggleRow label="解锁货架" checked={market?.autoUnlockShelf ?? false} onChange={(checked) => updateMarket({ autoUnlockShelf: checked })} status={SETTING_STATUS.adapterMissing} />
                    <ToggleRow label="自动上架鲜花" checked={market?.putEnabled ?? false} onChange={(checked) => updateMarket({ putEnabled: checked })} status={SETTING_STATUS.syncOnly} />
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
              </>
            )}
          </div>
        )}

        {activeTab === "order" && (
          <div className="space-y-4">
            <PolicyGroup title="居民订单" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="普通居民订单" checked={resident?.normalEnabled ?? false} onChange={(checked) => updateResident({ normalEnabled: checked })} />
                <NumberRow label="普通订单上限" value={resident?.normalDailyLimit || 1260} min={0} onChange={(value) => updateResident({ normalDailyLimit: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="绸缎订单" checked={resident?.satinEnabled ?? false} onChange={(checked) => updateResident({ satinEnabled: checked })} />
                    <NumberRow label="绸缎订单上限" value={resident?.satinDailyLimit || 120} min={0} onChange={(value) => updateResident({ satinDailyLimit: value })} />
                    <ToggleRow label="建材订单" checked={resident?.decorateEnabled ?? false} onChange={(checked) => updateResident({ decorateEnabled: checked })} />
                    <NumberRow label="建材订单上限" value={resident?.decorateDailyLimit || 120} min={0} onChange={(value) => updateResident({ decorateDailyLimit: value })} />
                  </>
                )}
                <ToggleRow label="居民领奖" checked={resident?.rewardEnabled ?? false} onChange={(checked) => updateResident({ rewardEnabled: checked })} />
                <QualityRow label="品质限定" value={resident?.qualities ?? []} onChange={(value) => updateResident({ qualities: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="顾客订单" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="顾客订单" checked={customer?.enabled ?? false} onChange={(checked) => updateCustomer({ enabled: checked })} />
                <ToggleRow label="暂时无货" checked={customer?.rejectUnavailableEnabled ?? false} onChange={(checked) => updateCustomer({ rejectUnavailableEnabled: checked })} />
                <StatusRow label="订单状态" value={customerOrderSyncLabel} tone={customerOrdersObserved ? "ready" : "muted"} />
              </div>
            </PolicyGroup>

            {SHOW_UNSUPPORTED_SETTINGS && (
              <PolicyGroup title="宫廷、组团" icon={<Package />}>
                <div className="grid gap-2 sm:grid-cols-2">
                  <ToggleRow label="宫廷订单" checked={palace?.enabled ?? false} onChange={(checked) => updatePalace({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                  <QualityRow label="宫廷品质" value={palace?.qualities ?? []} onChange={(value) => updatePalace({ qualities: value })} />
                  <ToggleRow label="组团订单" checked={team?.enabled ?? false} onChange={(checked) => updateTeam({ enabled: checked })} status={SETTING_STATUS.syncOnly} />
                  <ToggleRow label="再来一单" checked={team?.oneMoreEnabled ?? false} onChange={(checked) => updateTeam({ oneMoreEnabled: checked })} status={SETTING_STATUS.adapterMissing} />
                  <ToggleRow label="仅已培育" checked={team?.submitOnlyCultivated ?? false} onChange={(checked) => updateTeam({ submitOnlyCultivated: checked })} />
                  <QualityRow label="组团品质" value={team?.qualities ?? []} onChange={(value) => updateTeam({ qualities: value })} />
                  <BigIntNumberRow label="组团元宝上限" value={team?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateTeam({ maxSpendDiamond: value })} />
                </div>
              </PolicyGroup>
            )}

            <PolicyGroup title="花架售卖" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="解锁花架" checked={flowerArt?.autoUnlockStand ?? false} onChange={(checked) => updateFlowerArt({ autoUnlockStand: checked })} status={SETTING_STATUS.adapterMissing} />}
                <ToggleRow label="自动上架花艺" checked={flowerArt?.sellEnabled ?? false} onChange={(checked) => updateFlowerArt({ sellEnabled: checked })} />
                <ToggleRow label="自动制作" checked={flowerArt?.craftEnabled ?? false} onChange={(checked) => updateFlowerArt({ craftEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="提前下架" checked={flowerArt?.earlyCancelEnabled ?? false} onChange={(checked) => updateFlowerArt({ earlyCancelEnabled: checked })} status={SETTING_STATUS.adapterMissing} />}
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
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="自动种植" checked={unionLand?.autoPlantEnabled ?? false} onChange={(checked) => updateUnionLand({ autoPlantEnabled: checked })} status={SETTING_STATUS.paused} />
                    <QualityRow label="指定品质" value={unionLand?.qualities ?? []} onChange={(value) => updateUnionLand({ qualities: value })} />
                    <IntListRow label="指定花朵" value={unionLand?.flowerIds ?? []} onChange={(value) => updateUnionLand({ flowerIds: value })} />
                    <NumberRow label="最高花朵等级" value={unionLand?.maxFlowerLevel || 0} min={0} onChange={(value) => updateUnionLand({ maxFlowerLevel: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会建设" icon={<Coins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频建设" checked={unionBuild?.freeEnabled ?? false} onChange={(checked) => updateUnionBuild({ freeEnabled: checked })} status={SETTING_STATUS.videoTokenMissing} />}
                <ToggleRow label="金币建设" checked={unionBuild?.goldEnabled ?? false} onChange={(checked) => updateUnionBuild({ goldEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="元宝建设" checked={unionBuild?.diamondEnabled ?? false} onChange={(checked) => updateUnionBuild({ diamondEnabled: checked })} status={SETTING_STATUS.adapterMissing} />}
                <BigIntNumberRow label="金币上限" value={unionBuild?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendGold: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && <BigIntNumberRow label="元宝上限" value={unionBuild?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendDiamond: value })} />}
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会分享与摸花" icon={<HandCoins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="自动分享" checked={unionFlower?.shareEnabled ?? false} onChange={(checked) => updateUnionFlower({ shareEnabled: checked })} status={SETTING_STATUS.paused} />
                    <SegmentedRow label="分享模式" value={unionFlower?.shareMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ shareMode: value })} />
                    <QualityRow label="分享品质" value={unionFlower?.shareQualities ?? []} onChange={(value) => updateUnionFlower({ shareQualities: value })} />
                    <IntListRow label="分享花朵" value={unionFlower?.shareFlowerIds ?? []} onChange={(value) => updateUnionFlower({ shareFlowerIds: value })} />
                  </>
                )}
                <ToggleRow label="自动摸花" checked={unionFlower?.takeEnabled ?? false} onChange={(checked) => updateUnionFlower({ takeEnabled: checked })} />
                <SegmentedRow label="摸花模式" value={unionFlower?.takeMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ takeMode: value })} />
                <QualityRow label="摸花品质" value={unionFlower?.takeQualities ?? []} onChange={(value) => updateUnionFlower({ takeQualities: value })} />
                <IntListRow label="摸花花朵" value={unionFlower?.takeFlowerIds ?? []} onChange={(value) => updateUnionFlower({ takeFlowerIds: value })} />
              </div>
            </PolicyGroup>

            {SHOW_UNSUPPORTED_SETTINGS && (
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
            )}

            <PolicyGroup title="公会其他" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="公会红包" checked={union?.redPacketEnabled ?? false} onChange={(checked) => updateUnion({ redPacketEnabled: checked })} status={SETTING_STATUS.paused} />}
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
                        <NumberRow
                          key={param.key}
                          label={param.label}
                          value={safeBigIntToNumber(modulePolicy?.intParams?.[param.key], param.defaultValue)}
                          min={param.min}
                          max={param.max}
                          onChange={(value) => updateActivityIntParam(module.id, param.key, value, param.defaultValue)}
                        />
                      ))}
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
    <section className="space-y-3 rounded-md border border-border/55 bg-white/34 p-3 dark:bg-white/5">
      <SectionTitle icon={icon}>{title}</SectionTitle>
      {children}
    </section>
  );
}

function StatusRow({ label, value, tone }: { label: string; value: string; tone: "ready" | "muted" }) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Badge variant={tone === "ready" ? "secondary" : "outline"}>{value}</Badge>
    </div>
  );
}

function TextRow({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex min-h-9 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Input className="h-8 w-full text-right text-sm sm:w-36" value={value} onChange={(event) => onChange(event.target.value)} />
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
  const floor = BigInt(min);
  const normalizedValue = value < floor ? floor : value;

  return (
    <div className="flex min-h-12 items-center justify-between gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="min-w-0 leading-5">{label}</Label>
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        decrementDisabled={normalizedValue <= floor}
        onDecrement={() => onChange(normalizedValue - BigInt(1))}
        onIncrement={() => onChange(normalizedValue + BigInt(1))}
        onValueChange={(nextValue) => onChange(parseBigInt(nextValue, min))}
        wide
      />
    </div>
  );
}

function NumericStepper({
  label,
  value,
  min,
  max,
  disabled = false,
  decrementDisabled = false,
  incrementDisabled = false,
  wide = false,
  onDecrement,
  onIncrement,
  onValueChange,
}: {
  label: string;
  value: string;
  min: number;
  max?: number;
  disabled?: boolean;
  decrementDisabled?: boolean;
  incrementDisabled?: boolean;
  wide?: boolean;
  onDecrement: () => void;
  onIncrement: () => void;
  onValueChange: (value: string) => void;
}) {
  const buttonClassName =
    "flex h-full items-center justify-center text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn(
        "grid h-9 shrink-0 grid-cols-[2.25rem_minmax(0,1fr)_2.25rem] overflow-hidden rounded-lg border border-input/85 bg-white/66 shadow-[inset_0_1px_0_rgba(255,255,255,0.78)] transition-[border-color,box-shadow,background-color] focus-within:border-ring focus-within:bg-white/88 focus-within:ring-3 focus-within:ring-ring/24 dark:bg-input/42 dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] dark:focus-within:bg-input/58",
        wide ? "w-40" : "w-36",
        disabled && "opacity-55",
      )}
    >
      <button
        type="button"
        aria-label={`减少${label}`}
        className={cn(buttonClassName, "border-r border-input/65")}
        disabled={disabled || decrementDisabled}
        onClick={onDecrement}
      >
        <Minus className="size-3.5" />
      </button>
      <input
        type="number"
        inputMode="numeric"
        aria-label={label}
        className="h-full min-w-0 bg-transparent px-1 text-center text-sm font-semibold tabular-nums text-foreground outline-none [appearance:textfield] disabled:cursor-not-allowed [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
        min={min}
        max={max}
        step={1}
        disabled={disabled}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
      />
      <button
        type="button"
        aria-label={`增加${label}`}
        className={cn(buttonClassName, "border-l border-input/65")}
        disabled={disabled || incrementDisabled}
        onClick={onIncrement}
      >
        <Plus className="size-3.5" />
      </button>
    </div>
  );
}

function IntListRow({ label, value, onChange }: { label: string; value: number[]; onChange: (value: number[]) => void }) {
  return (
    <div className="space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
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

type FlowerPickerOption = {
  id: number;
  name: string;
  seedName: string;
  stock: number;
  gold: number;
  experience: number;
  plantable: boolean;
};

function FlowerMultiSelectRow({
  label,
  value,
  plantableFlowers,
  synced,
  onChange,
}: {
  label: string;
  value: number[];
  plantableFlowers: PlantableFlowerView[];
  synced: boolean;
  onChange: (value: number[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const flowers = useMemo<FlowerPickerOption[]>(() => {
    const options = plantableFlowers.map((flower) => {
      const display = flowerDisplay(flower.flowerId);
      return {
        id: flower.flowerId,
        name: flower.flowerName || display.name,
        seedName: display.seedName,
        stock: flower.stock,
        gold: flower.gold,
        experience: flower.experience,
        plantable: true,
      };
    });
    const known = new Set(options.map((option) => option.id));
    for (const id of value) {
      if (known.has(id)) continue;
      const display = flowerDisplay(id);
      options.push({
        id,
        name: display.name,
        seedName: display.seedName,
        stock: 0,
        gold: display.flower?.gold ?? 0,
        experience: display.flower?.experience ?? 0,
        plantable: false,
      });
    }
    return options.sort((a, b) => {
      if (a.plantable !== b.plantable) return a.plantable ? -1 : 1;
      return a.id - b.id;
    });
  }, [plantableFlowers, value]);
  const visibleFlowers = useMemo(() => {
    const text = query.trim().toLowerCase();
    if (!text) return flowers;
    return flowers.filter((flower) => {
      return String(flower.id).includes(text) || flower.name.toLowerCase().includes(text) || flower.seedName.toLowerCase().includes(text);
    });
  }, [flowers, query]);
  const selectedPreview = value.slice(0, 4).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 4 ? value.length - 4 : 0;
  const toggleFlower = (flowerID: number) => onChange(toggleNumber(value, flowerID));

  return (
    <div className="space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex items-center justify-between gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">可种 {plantableFlowers.length}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "未选择"}</Badge>
        </div>
      </div>
      <div className="flex min-h-8 items-center justify-between gap-2">
        <div className="min-w-0 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择时不限制" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Flower2 className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) setQuery("");
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花名、种子或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleFlowers.length === 0 ? (
                <EmptyState title={synced ? "没有匹配花种" : "尚未同步可种花种"} detail={synced ? undefined : "登录账号并同步培育状态后可选择"} />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleFlowers.map((flower) => {
                    const selected = selectedSet.has(flower.id);
                    const display = flowerDisplay(flower.id);
                    const color = display.item?.color;
                    return (
                      <button
                        key={flower.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleFlower(flower.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex min-w-0 items-center gap-1.5">
                            <span className="truncate text-sm font-medium">{flower.name}</span>
                            {!flower.plantable && <Badge variant="outline">当前不可种</Badge>}
                          </span>
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            <span>{flower.id}</span>
                            {color ? <span>品质 {color}</span> : null}
                            {flower.stock > 0 ? <span>库存 {formatCount(flower.stock)}</span> : null}
                            {flower.gold ? <span>金币 {formatCount(flower.gold)}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button type="button" variant="ghost" className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted" onClick={() => onChange([])} disabled={value.length === 0}>
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                setQuery("");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function QualityRow({ label, value, onChange }: { label: string; value: number[]; onChange: (value: number[]) => void }) {
  return (
    <div className="flex min-h-9 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
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
                selected ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
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
    <div className="space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="text-sm">{label}</Label>
      <div className="flex flex-wrap gap-1">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "min-h-8 rounded border px-2 text-xs font-medium",
              option.value === value ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function DemandPriorityEditor({
  value,
  onChange,
}: {
  value: Record<string, number>;
  onChange: (value: Record<string, number>) => void;
}) {
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const orderedGoals = useMemo(() => {
    return [...GOAL_OPTIONS].sort((a, b) => {
      const priorityDelta = priorityForGoal(value, b) - priorityForGoal(value, a);
      if (priorityDelta !== 0) return priorityDelta;
      return GOAL_OPTIONS.findIndex((goal) => goal.id === a.id) - GOAL_OPTIONS.findIndex((goal) => goal.id === b.id);
    });
  }, [value]);

  const commitOrder = (goals: typeof GOAL_OPTIONS) => {
    const total = goals.length;
    onChange(Object.fromEntries(goals.map((goal, index) => [goal.id, (total - index) * 10])));
  };

  const moveGoal = (sourceId: string, targetId: string) => {
    if (sourceId === targetId) return;
    const from = orderedGoals.findIndex((goal) => goal.id === sourceId);
    const to = orderedGoals.findIndex((goal) => goal.id === targetId);
    if (from < 0 || to < 0) return;
    const next = [...orderedGoals];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    commitOrder(next);
  };

  const handlePointerUp = (event: PointerEvent<HTMLDivElement>, sourceId: string) => {
    const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
    const targetId = target?.dataset.goalId;
    if (targetId) moveGoal(sourceId, targetId);
    setDraggingId(null);
  };

  return (
    <div className="mt-3 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex items-center justify-between gap-3">
        <Label className="text-sm">生产需求优先级</Label>
        <span className="text-xs text-muted-foreground">缺花补种排序</span>
      </div>
      <div
        className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3"
        onMouseUp={(event) => {
          if (!draggingId) return;
          const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
          const targetId = target?.dataset.goalId;
          if (targetId) moveGoal(draggingId, targetId);
          setDraggingId(null);
        }}
        onMouseLeave={(event) => {
          if (event.buttons === 0) setDraggingId(null);
        }}
      >
        {orderedGoals.map((goal, index) => (
          <div
            key={goal.id}
            data-goal-id={goal.id}
            aria-grabbed={draggingId === goal.id}
            onMouseDown={(event) => {
              if (event.button !== 0) return;
              setDraggingId(goal.id);
            }}
            onPointerDown={(event) => {
              if (event.button !== 0) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              setDraggingId(goal.id);
            }}
            onPointerUp={(event) => handlePointerUp(event, goal.id)}
            onPointerCancel={() => setDraggingId(null)}
            className={cn(
              "flex min-h-11 cursor-grab touch-none items-center gap-2 rounded-md border border-border/70 bg-card px-2.5 py-2 text-sm shadow-sm transition active:cursor-grabbing",
              draggingId === goal.id ? "opacity-60 ring-1 ring-primary" : "hover:border-primary/50 hover:bg-white/66 dark:hover:bg-white/8",
            )}
          >
            <GripVertical className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <span className="flex size-6 shrink-0 items-center justify-center rounded bg-secondary text-xs font-medium text-muted-foreground dark:bg-white/8">{index + 1}</span>
            <span className="min-w-0 flex-1 truncate font-medium">{goal.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function priorityForGoal(value: Record<string, number>, goal: (typeof GOAL_OPTIONS)[number]) {
  return value[goal.id] || goal.defaultPriority;
}

function EventPanel({ events }: { events: Event[] }) {
  const [activeCategory, setActiveCategory] = useState("all");
  const categoryCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const event of events) {
      const category = eventCategory(event);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [events]);
  const categories = useMemo(() => {
    const order = ["basic", "plant", "order", "union", "activity", "account", "system"];
    return [...categoryCounts.keys()].sort((a, b) => {
      const ai = order.indexOf(a);
      const bi = order.indexOf(b);
      if (ai >= 0 && bi >= 0) return ai - bi;
      if (ai >= 0) return -1;
      if (bi >= 0) return 1;
      return a.localeCompare(b);
    });
  }, [categoryCounts]);
  const visibleEvents = useMemo(() => {
    if (activeCategory === "all") return events;
    return events.filter((event) => eventCategory(event) === activeCategory);
  }, [activeCategory, events]);

  useEffect(() => {
    if (activeCategory !== "all" && !categoryCounts.has(activeCategory)) {
      setActiveCategory("all");
    }
  }, [activeCategory, categoryCounts]);

  return (
    <Card className="cloud-surface min-h-0 flex-1">
      <CardHeader className="shrink-0">
        <CardTitle>日志</CardTitle>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col gap-3">
        <div className="dark-scrollbar flex shrink-0 gap-1 overflow-x-auto rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5">
          <button
            type="button"
            className={cn(
              "flex h-8 shrink-0 items-center gap-2 rounded px-3 text-xs font-medium transition-colors",
              activeCategory === "all" ? "bg-white text-foreground shadow-sm dark:bg-muted" : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
            )}
            onClick={() => setActiveCategory("all")}
          >
            全部
          </button>
          {categories.map((category) => (
            <button
              key={category}
              type="button"
              className={cn(
                "flex h-8 shrink-0 items-center gap-2 rounded px-3 text-xs font-medium transition-colors",
                activeCategory === category ? "bg-white text-foreground shadow-sm dark:bg-muted" : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
              )}
              onClick={() => setActiveCategory(category)}
            >
              {categoryLabel(category)}
            </button>
          ))}
        </div>

        {visibleEvents.length === 0 ? (
          <div className="flex min-h-0 flex-1 items-center justify-center">
            <EmptyState title="暂无日志" />
          </div>
        ) : (
          <div className="dark-scrollbar min-h-0 flex-1 space-y-2 overflow-y-auto rounded-md border border-border/58 bg-white/34 p-2 font-mono text-xs sm:space-y-0 sm:p-0 dark:bg-white/5">
            {visibleEvents.map((event, index) => (
              <div
                key={event.id || `${event.kind}-${index}-${event.message}`}
                className="grid gap-1 rounded-md border border-border/55 bg-card/72 px-3 py-2 last:border-b-0 sm:rounded-none sm:border-x-0 sm:border-t-0 sm:bg-transparent sm:grid-cols-[108px_64px_minmax(0,1fr)] sm:gap-3"
              >
                <span className="text-muted-foreground">{formatTimestamp(event.ts)}</span>
                <span
                  className={cn(
                    "font-sans text-xs font-medium",
                    event.level === "error" ? "text-destructive" : event.level === "warn" ? "text-amber-600 dark:text-amber-300" : "text-primary",
                  )}
                >
                  {categoryLabel(eventCategory(event))}
                </span>
                <div className="min-w-0 whitespace-pre-wrap break-words text-foreground">
                  <span className="font-semibold">{eventTitle(event)}</span>
                  {eventMessage(event) && <span className="text-muted-foreground"> - {eventMessage(event)}</span>}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function OverviewStat({ icon, label, value, detail }: { icon: ReactNode; label: string; value: ReactNode; detail?: ReactNode }) {
  return (
    <div className="flex min-h-[72px] min-w-0 items-center gap-2 rounded-md border border-border/55 bg-white/52 px-2.5 py-2 shadow-sm transition-colors hover:bg-white/68 dark:bg-white/6 dark:hover:bg-white/9 sm:min-h-[76px] sm:gap-3 sm:px-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 shadow-sm dark:bg-white/8 dark:text-sky-300 sm:size-9 [&_svg]:size-4">{icon}</div>
      <div className="min-w-0">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div className="truncate text-base font-semibold tabular-nums sm:text-lg">{value}</div>
        {detail && <div className="truncate text-xs text-muted-foreground">{detail}</div>}
      </div>
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
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
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
  max,
  disabled = false,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max?: number;
  disabled?: boolean;
  onChange: (value: number) => void;
}) {
  const normalizedValue = Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, Number.isFinite(value) ? Math.trunc(value) : min));
  const updateValue = (nextValue: number) => onChange(Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, nextValue)));

  return (
    <div
      className={cn(
        "flex min-h-12 items-center justify-between gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2 transition-opacity dark:bg-white/5",
        disabled && "opacity-55",
      )}
    >
      <Label className="min-w-0 leading-5">{label}</Label>
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        max={max}
        disabled={disabled}
        decrementDisabled={normalizedValue <= min}
        incrementDisabled={max !== undefined && normalizedValue >= max}
        onDecrement={() => updateValue(normalizedValue - 1)}
        onIncrement={() => updateValue(normalizedValue + 1)}
        onValueChange={(nextValue) => updateValue(parseNumber(nextValue, min))}
      />
    </div>
  );
}

function SectionTitle({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2 text-sm font-semibold">
      <span className="flex size-7 items-center justify-center rounded-md bg-secondary text-sky-600 dark:bg-white/8 dark:text-sky-300 [&_svg]:size-4">{icon}</span>
      {children}
    </div>
  );
}

function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}

function HealthBadge({ account, status }: { account: Account; status?: AccountStatus }) {
  const connected = accountConnected(account, status);
  if (accountStatusIssues(status).length > 0) return <Badge variant="destructive">异常</Badge>;
  if (!connected) return <Badge variant="outline">离线</Badge>;
  if (status?.health === "blocked" || status?.lastError) return <Badge variant="destructive">异常</Badge>;
  return <Badge variant="secondary">在线</Badge>;
}

function accountStatusIssues(status?: AccountStatus) {
  const diagnostics = status?.diagnostics;
  const issues = [
    status?.lastError,
    diagnostics?.lastOperationError,
    diagnostics?.sessionInvalidatedReason,
    ...(diagnostics?.blockedReasons ?? []),
  ]
    .map((issue) => issue?.trim())
    .filter((issue): issue is string => Boolean(issue));

  if (status?.health === "blocked" && issues.length === 0) {
    issues.push("账号处于异常状态，但后端未返回具体原因。");
  }

  return [...new Set(issues)];
}

function OperationStatusBadge({ operation }: { operation: PlannedOperation }) {
  if (isOperationCooling(operation)) return <Badge variant="secondary">冷却</Badge>;
  if (operation.status === PlanStatus.BLOCKED || operation.blockedReasons.length > 0) return <Badge variant="destructive">阻塞</Badge>;
  if (operation.syncOnly) return <Badge variant="outline">同步</Badge>;
  if (!operation.executable) return <Badge variant="outline">{planStatusLabel(operation.status)}</Badge>;
  if (operation.status === PlanStatus.MANAGED) return <Badge variant="secondary">调度</Badge>;
  return <Badge>可执行</Badge>;
}

function comparePendingTasks(a: PendingTaskView, b: PendingTaskView) {
  const statusDelta = pendingTaskStatusRank(a) - pendingTaskStatusRank(b);
  if (statusDelta !== 0) return statusDelta;
  const categoryDelta = pendingTaskCategoryRank(a.category) - pendingTaskCategoryRank(b.category);
  if (categoryDelta !== 0) return categoryDelta;
  const aID = Number(a.id);
  const bID = Number(b.id);
  if (Number.isFinite(aID) && Number.isFinite(bID) && aID !== bID) return aID - bID;
  return (a.title || a.id).localeCompare(b.title || b.id, "zh-CN");
}

function pendingTaskStatusRank(task: PendingTaskView) {
  if (pendingTaskBlocked(task)) return 0;
  if (pendingTaskHasShortage(task)) return 1;
  if (pendingTaskCooling(task)) return 3;
  switch (task.status) {
    case PlanStatus.READY:
      return 2;
    case PlanStatus.MANAGED:
      return 4;
    case PlanStatus.SYNC_ONLY:
      return 5;
    case PlanStatus.SKIPPED:
      return 6;
    default:
      return 7;
  }
}

function pendingTaskCategoryRank(category: string) {
  const order = ["顾客订单", "居民订单", "主线任务", "主线剧情", "日常任务", "周常任务", "成就任务", "activity", "地图随机事件", "宠物事件"];
  const index = order.indexOf(category);
  return index >= 0 ? index : order.length;
}

function isOrderPendingTask(task: PendingTaskView) {
  if (task.category === "activity") return false;
  return task.category.includes("订单") || task.title.includes("订单");
}

function pendingTaskCategoryLabel(category: string) {
  return category === "activity" ? "活动" : categoryLabel(category);
}

function pendingTaskBlocked(task: PendingTaskView) {
  return (
    task.status === PlanStatus.BLOCKED ||
    task.requirements.some((req) => req.blockedReasons.length > 0)
  );
}

function pendingTaskHasShortage(task: PendingTaskView) {
  return task.requirements.some((req) => req.missing > 0);
}

function pendingTaskCooling(task: PendingTaskView) {
  return Number(task.cooldownUntilMs) > Date.now();
}

function taskMonitorDetail(tasks: PendingTaskView[]) {
  if (tasks.length === 0) return "暂无";
  const ready = tasks.filter((task) => task.status === PlanStatus.READY && !pendingTaskCooling(task)).length;
  const cooling = tasks.filter(pendingTaskCooling).length;
  const shortage = tasks.filter(pendingTaskHasShortage).length;
  const blocked = tasks.filter(pendingTaskBlocked).length;
  return [`可处理 ${ready}`, cooling > 0 ? `冷却 ${cooling}` : "", shortage > 0 ? `缺口 ${shortage}` : "", blocked > 0 ? `阻塞 ${blocked}` : ""].filter(Boolean).join(" / ");
}

function requirementShortageSummary(tasks: PendingTaskView[]) {
  const totals = new Map<number, { name: string; missing: number }>();
  for (const task of tasks) {
    for (const req of task.requirements) {
      if (req.missing <= 0) continue;
      const current = totals.get(req.itemId) ?? { name: req.itemName || itemName(req.itemId), missing: 0 };
      current.missing += req.missing;
      totals.set(req.itemId, current);
    }
  }
  return [...totals.values()]
    .sort((a, b) => b.missing - a.missing || a.name.localeCompare(b.name, "zh-CN"))
    .slice(0, 3)
    .map((item) => `${item.name} ${formatCount(item.missing)}`)
    .join("、");
}

function pendingTaskProgressPercent(task: PendingTaskView) {
  if (task.target <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((task.finished / task.target) * 100)));
}

function taskProgressLabel(task: PendingTaskView) {
  if (pendingTaskCooling(task)) {
    const reason = task.cooldownReason || "冷却中";
    return `${reason}，约 ${pendingTaskCooldownRemaining(task)}后可交付`;
  }
  if (task.target > 0) return `${formatCount(task.finished)}/${formatCount(task.target)}`;
  if (task.requirements.length === 0) return "";
  const missing = task.requirements.reduce((sum, req) => sum + Math.max(0, req.missing), 0);
  return missing > 0 ? `缺 ${formatCount(missing)}` : "资源满足";
}

function pendingTaskCooldownRemaining(task: PendingTaskView) {
  const seconds = Math.max(1, Math.ceil((Number(task.cooldownUntilMs) - Date.now()) / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.ceil(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟`;
  return `${Math.ceil(minutes / 60)} 小时`;
}

function orderStatisticItems(statistics?: OrderStatisticsView) {
  return [
    { label: "居民", value: statistics?.residentNormalFinished ?? 0 },
    { label: "顾客", value: statistics?.customerFinished ?? 0 },
    { label: "宫廷", value: statistics?.palaceFinished ?? 0 },
    { label: "绸缎", value: statistics?.residentSatinFinished ?? 0 },
    { label: "建材", value: statistics?.residentDecorateFinished ?? 0 },
    { label: "花艺", value: statistics?.flowerArtSold ?? 0 },
  ];
}

function sumRuntimeActionTotals(items: RuntimeActionTotal[]) {
  return items.reduce((sum, item) => sum + item.count, BigInt(0));
}

function runtimeWindowLabel(statistics?: RuntimeStatisticsView) {
  if (!statistics) return "暂无运行统计";
  if (statistics.running) {
    const started = formatTimestamp(statistics.startedAt);
    return started === "-" ? "运行中" : `启动 ${started}`;
  }
  const stopped = formatTimestamp(statistics.stoppedAt);
  return stopped === "-" ? "最近已停止" : `停止 ${stopped}`;
}

function runtimeResourcePrimaryValue(items: RuntimeResourceTotal[]) {
  const first = items.find((item) => item.gained > BigInt(0));
  if (!first) return "-";
  return `+${formatCount(first.gained)}`;
}

function runtimeResourceGainSummary(items: RuntimeResourceTotal[]) {
  const visible = items.filter((item) => item.gained > BigInt(0)).slice(0, 3);
  if (visible.length === 0) return "暂无资源进账";
  return visible.map((item) => `${item.label || item.key} +${formatCount(item.gained)}`).join("、");
}

function runtimeActionSummary(items: RuntimeActionTotal[]) {
  const visible = items.filter((item) => item.count > BigInt(0)).slice(0, 3);
  if (visible.length === 0) return "暂无完成";
  return visible.map((item) => `${item.label || item.key} ${formatCount(item.count)}`).join("、");
}

function cyclicNotePhaseLabel(phase: number) {
  switch (phase) {
    case 1:
      return "预告期";
    case 2:
      return "进行中";
    case 3:
      return "领奖期";
    case 4:
      return "已结束";
    default:
      return "未开始";
  }
}

function cyclicNotePhaseDetail(activity: CyclicNoteView) {
  if (activity.phase === 4) return activity.endMs > BigInt(0) ? `结束于 ${formatUnixTime(activity.endMs)}` : "活动已结束";
  const endMs = Number(activity.phaseEndMs);
  if (!Number.isFinite(endMs) || endMs <= 0) return "阶段时间尚未同步";
  const remaining = endMs - Date.now();
  if (remaining <= 0) return "等待服务端阶段更新";
  const prefix = activity.phase === 1 ? "距开始" : activity.phase === 3 ? "领奖剩余" : "剩余";
  return `${prefix} ${formatRemainingMilliseconds(remaining)}`;
}

function dessertPhaseDetail(activity: DessertView) {
  if (activity.phase === 4) return activity.endMs > BigInt(0) ? `结束于 ${formatUnixTime(activity.endMs)}` : "活动已结束";
  const endMs = Number(activity.phaseEndMs);
  if (!Number.isFinite(endMs) || endMs <= 0) return "阶段时间尚未同步";
  const remaining = endMs - Date.now();
  if (remaining <= 0) return "等待服务端阶段更新";
  const prefix = activity.phase === 1 ? "距开始" : activity.phase === 3 ? "领奖剩余" : "剩余";
  return `${prefix} ${formatRemainingMilliseconds(remaining)}`;
}

function formatRemainingMilliseconds(milliseconds: number) {
  const totalMinutes = Math.max(1, Math.ceil(milliseconds / 60_000));
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}天${hours > 0 ? `${hours}小时` : ""}`;
  if (hours > 0) return `${hours}小时${minutes > 0 ? `${minutes}分` : ""}`;
  return `${minutes}分钟`;
}

function planStatusLabel(status: PlanStatus) {
  switch (status) {
    case PlanStatus.READY:
      return "可执行";
    case PlanStatus.MANAGED:
      return "调度";
    case PlanStatus.SYNC_ONLY:
      return "同步";
    case PlanStatus.ADAPTER_MISSING:
      return "缺适配";
    case PlanStatus.BLOCKED:
      return "阻塞";
    case PlanStatus.SKIPPED:
      return "跳过";
    default:
      return "等待";
  }
}

function operationTitle(operation: PlannedOperation) {
  return operation.label || operationActionLabel(operation.action) || operation.domain || operation.rpc || "操作";
}

function operationTargetLabel(operation: PlannedOperation) {
  const landIds = operationLandIds(operation);
  if (landIds.length > 0) {
    return landIds.map(landDisplayName).join("、");
  }
  if (operation.rpc === "flowerArt.makeFlowerArt") {
    const art = operation.itemId ? itemName(operation.itemId) : "花艺";
    const count = operation.count ? `x${operation.count}` : "";
    const prefix = operation.domain === "order.customer" && operation.targetId ? `NPC ${operation.targetId}` : "";
    return [prefix, art, count].filter(Boolean).join(" ");
  }
  if (operation.rpc === "orderCustomer.finishOrder" || operation.rpc === "orderCustomer.rejectOrder") {
    return operation.targetId ? `NPC ${operation.targetId}` : "";
  }
  if (operation.targetUid !== BigInt(0)) {
    return `UID ${operation.targetUid.toString()}${operation.targetId ? ` · 槽位 ${operation.targetId}` : ""}`;
  }
  if (operation.targetUids.length > 0) {
    return `${operation.targetUids.length} 个候选 UID`;
  }
  const parts = [
    operation.targetId ? operationTargetIdLabel(operation) : "",
    operation.itemId ? itemName(operation.itemId) : "",
    operation.flowerId ? itemName(operation.flowerId) : "",
    operation.count ? `x${operation.count}` : "",
  ].filter(Boolean);
  return parts.join(" ");
}

function operationCostLabel(operation: PlannedOperation) {
  if (operation.costGates.length > 0) {
    const gateCosts = operation.costGates
      .filter((gate) => Number(gate.required) > 0)
      .map((gate) => {
        const label = gate.label || (gate.itemId ? itemName(gate.itemId) : "成本");
        const available = Number(gate.available);
        const required = Number(gate.required);
        const availability = available > 0 || gate.status === PlanStatus.BLOCKED ? `/${available}` : "";
        return `${label} ${required}${availability}`;
      });
    if (gateCosts.length > 0) {
      return `成本 ${gateCosts.join("、")}`;
    }
  }
  const itemCosts = Object.entries(operation.itemCost)
    .filter(([, count]) => count > 0)
    .map(([id, count]) => `${itemName(Number(id))}x${count}`);
  const costs = [
    operation.goldCost ? `金币 ${operation.goldCost}` : "",
    operation.diamondCost ? `元宝 ${operation.diamondCost}` : "",
    ...itemCosts,
  ].filter(Boolean);
  return costs.length > 0 ? `成本 ${costs.join("、")}` : "";
}

function operationNoteLabel(operation: PlannedOperation) {
  if (isOperationCooling(operation)) {
    const reason = operation.cooldownReason || "操作冷却中";
    return `${reason}，${operationCooldownRemaining(operation)}后重试`;
  }
  const raw = operation.blockedReasons.length > 0 ? operation.blockedReasons.join("、") : operation.reason;
  return operationReasonLabel(raw);
}

function isOperationCooling(operation: PlannedOperation) {
  return Number(operation.cooldownUntilMs) > Date.now();
}

function operationCooldownRemaining(operation: PlannedOperation) {
  const seconds = Math.max(1, Math.ceil((Number(operation.cooldownUntilMs) - Date.now()) / 1000));
  if (seconds < 60) return `${seconds}秒`;
  const minutes = Math.ceil(seconds / 60);
  if (minutes < 60) return `${minutes}分钟`;
  return `${Math.ceil(minutes / 60)}小时`;
}

function operationLandIds(operation: PlannedOperation) {
  if (operation.landIds.length > 0) return operation.landIds;
  if ((operation.domain.startsWith("farm.") || operation.rpc.startsWith("usrLand.")) && operation.targetId > 0) {
    return [operation.targetId];
  }
  return [];
}

function operationTargetIdLabel(operation: PlannedOperation) {
  if (operation.domain === "order.customer") return `NPC ${operation.targetId}`;
  if (operation.domain.startsWith("order.flower_art")) return `花架 ${operation.targetId}`;
  if (operation.domain.startsWith("union.")) return `目标 ${operation.targetId}`;
  return `#${operation.targetId}`;
}

function operationActionLabel(action: string) {
  switch (action) {
    case "harvest":
      return "收获";
    case "plant":
      return "种植";
    case "water":
      return "浇水";
    case "finish":
    case "submit":
      return "提交";
    case "reject":
      return "暂时无货";
    case "claim":
      return "领取";
    case "craft":
      return "制作";
    case "sell":
      return "上架";
    case "sync":
      return "同步";
    case "buy":
      return "购买";
    case "unlock":
      return "解锁";
    case "feed":
      return "喂食";
    case "stroke":
      return "互动";
    case "find_pet":
      return "寻回";
    case "handle_event":
      return "处理";
    default:
      return action;
  }
}

function operationReasonLabel(reason: string) {
  if (!reason) return "";
  if (reason === "ready land" || reason.includes("initial bloom ready") || reason.includes("elapsed")) return "可收获";
  if (reason === "land is empty") return "空地";
  if (reason.includes("awaiting first water")) return "待浇水";
  if (reason.includes("regrowing")) return "成长中";
  if (reason.includes("not actionable")) return "等待";
  if (reason.includes("no observed")) return "未同步";
  return reason;
}

function eventCategory(event: Event) {
  if (event.category) return event.category;
  if (event.domain) return event.domain.split(".")[0] || "system";
  return "system";
}

function eventTitle(event: Event) {
  return event.label || [event.domain, event.action].filter(Boolean).join(".") || event.kind || "-";
}

function eventMessage(event: Event) {
  return event.message || event.payloadJson || "";
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
    case "unlock":
      return "待开";
    case "locked":
      return "锁定";
    case "unknown":
      return "未知";
    default:
      return value || "未知";
  }
}

function landStatusLabel(status: string) {
  switch (status) {
    case "opened":
      return "已开";
    case "unopened":
      return "未开";
    case "locked":
      return "锁定";
    default:
      return status || "未知";
  }
}

function landDisplayName(landId: number) {
  if (landId >= 1001 && landId < 2000) return `#${landId - 1000}`;
  return `#${landId}`;
}

function landTimingLabel(land: LandView, status: string) {
  switch (land.recommendation) {
    case "harvest":
      return "可收获";
    case "water":
      return "待浇水";
    case "plant":
      return "待种植";
  }
  if (status !== "opened") {
    return landStatusLabel(status);
  }
  const nextTime = formatUnixTime(land.nextTimeMs);
  if (nextTime !== "-") return `成熟 ${nextTime}`;
  return land.flowerId > 0 ? "成长中" : "待同步";
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

function formatUnixTime(value?: bigint) {
  const milliseconds = Number(value ?? BigInt(0));
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(milliseconds));
}

function firstPositiveUnixTime(...values: (bigint | undefined)[]) {
  return values.find((value) => Number(value ?? BigInt(0)) > 0);
}

function formatCount(value: number | bigint) {
  const numeric = typeof value === "bigint" ? Number(value) : value;
  if (!Number.isFinite(numeric)) return "0";
  return NUMBER_FORMATTER.format(numeric);
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

function safeBigIntToNumber(value: bigint | undefined, fallback: number) {
  if (value === undefined) return fallback;
  const upper = BigInt(Number.MAX_SAFE_INTEGER);
  const lower = BigInt(Number.MIN_SAFE_INTEGER);
  if (value > upper) return Number.MAX_SAFE_INTEGER;
  if (value < lower) return Number.MIN_SAFE_INTEGER;
  return Number(value);
}

function safeNumberToBigInt(value: number, fallback: number) {
  if (!Number.isFinite(value)) return BigInt(fallback);
  const integer = Math.trunc(value);
  if (integer > Number.MAX_SAFE_INTEGER) return BigInt(Number.MAX_SAFE_INTEGER);
  if (integer < Number.MIN_SAFE_INTEGER) return BigInt(Number.MIN_SAFE_INTEGER);
  return BigInt(integer);
}

function truncateMiddle(value: string, head: number, tail: number) {
  if (value.length <= head + tail + 1) return value;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
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
