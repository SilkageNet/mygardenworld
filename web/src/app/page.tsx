"use client";

import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ComponentProps, type FormEvent, type ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import {
  AlertTriangle,
  ArrowLeft,
  BadgeCheck,
  CalendarDays,
  Check,
  ChevronDown,
  Cloud,
  Coins,
  Flower2,
  Gem,
  HandCoins,
  ListChecks,
  Loader2,
  LogOut,
  Package,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Search,
  Send,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
  Square,
  Sprout,
  Ticket,
  Trash2,
  TrendingUp,
  Trophy,
  Waves,
} from "lucide-react";

import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import { ExecutionLane, PlanStatus, QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import type {
  AccountStatus,
  ActivityItem,
  CyclicNoteMilestone,
  CyclicNoteTaskSlot,
  CyclicNoteView,
  CyclicStoryOrder,
  CyclicStoryView,
  DessertCelebrityLikeView,
  DessertMilestoneView,
  DessertModeView,
  DessertRuntimeView,
  DessertTaskView,
  DessertView,
  Event,
  FeatureCapability,
  FmlRaceTask,
  FmlRaceTaken,
  FmlRaceView,
  GetSnapshotResponse,
  InventoryLedgerItem,
  InventoryLedgerView,
  LandView,
  OrderStatisticsView,
  PendingTaskView,
  PlannedOperation,
  RequirementView,
  RuntimeActionTotal,
  RuntimeResourceTotal,
  RuntimeStatisticsView,
} from "@/gen/mygardenworld/v1/query_service_pb";
import AppShell from "@/components/app-shell";
import PolicyPanel from "@/components/dashboard/policy-panel";
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
import { experienceToNextLevel, itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);
const queryClient = createClient(QueryService, transport);

const NUMBER_FORMATTER = new Intl.NumberFormat("zh-CN");
const EVENT_LIMIT = 500;
const EVENT_RECONNECT_INITIAL_MS = 1000;
const EVENT_RECONNECT_MAX_MS = 15000;
const STATUS_POLL_MS = 5000;
const SNAPSHOT_REFRESH_EVENT_KINDS = new Set([
  "operation_ack",
  "union_flower_take",
  "resource_changed",
  "inventory_changed",
  "land_changed",
  "order_finish",
  "order_satin_finish",
  "order_decorate_finish",
  "order_customer",
  "flower_art",
  "flower_rack",
  "task_recv",
  "waterwheel",
  "free_water",
  "benefit_box",
]);

type DashboardTabId = "monitor" | "settings" | "logs" | "race" | "land" | "warehouse";
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

const DASHBOARD_TABS: { id: DashboardTabId; label: string; icon: ReactNode }[] = [
  { id: "monitor", label: "监控", icon: <BadgeCheck /> },
  { id: "settings", label: "设置", icon: <ShieldCheck /> },
  { id: "logs", label: "日志", icon: <CalendarDays /> },
  { id: "race", label: "公会竞赛", icon: <Trophy /> },
  { id: "land", label: "土地", icon: <Sprout /> },
  { id: "warehouse", label: "仓库", icon: <Package /> },
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
  const [featureCapabilities, setFeatureCapabilities] = useState<FeatureCapability[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [snapshot, setSnapshot] = useState<GetSnapshotResponse | null>(null);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [policyLoading, setPolicyLoading] = useState(false);
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [busyAction, setBusyAction] = useState("");
  const [busyAutomationAccountId, setBusyAutomationAccountId] = useState("");
  const [busyBulkAutomation, setBusyBulkAutomation] = useState<"" | "start" | "pause">("");
  const [error, setError] = useState("");
  const [policyMessage, setPolicyMessage] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [addForm, setAddForm] = useState<AddAccountForm>(EMPTY_ADD_FORM);
  const [redeemOpen, setRedeemOpen] = useState(false);
  const [redeemCode, setRedeemCode] = useState("");
  const [redeemBusy, setRedeemBusy] = useState(false);
  const [redeemSummary, setRedeemSummary] = useState("");
  const [redeemResults, setRedeemResults] = useState<Array<{ accountName: string; ok: boolean; message: string }>>([]);
  const [dashboardTab, setDashboardTab] = useState<DashboardTabId>("monitor");
  const didAutoSelectAccount = useRef(false);
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());
  const accountsLoadedRef = useRef(false);
  const capabilitiesLoadedRef = useRef(false);

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

  const refreshAccounts = useCallback(async () => {
    const accountRes = await accountClient.listAccounts({});
    setAccounts(accountRes.accounts);
    accountsLoadedRef.current = true;
  }, []);

  const refreshStatuses = useCallback(async () => {
    const includeFeatureCapabilities = !capabilitiesLoadedRef.current;
    const statusRes = await queryClient.getStatus({ includeFeatureCapabilities });
    const nextStatuses = new Map<string, AccountStatus>();
    for (const status of statusRes.accounts) {
      nextStatuses.set(status.accountId, status);
    }
    setStatuses(nextStatuses);
    if (includeFeatureCapabilities) {
      setFeatureCapabilities(statusRes.featureCapabilities);
      capabilitiesLoadedRef.current = statusRes.featureCapabilities.length > 0;
    }
    setError((current) => (isTransientConnectionMessage(current) ? "" : current));
  }, []);

  const refreshAccountCollection = useCallback(async () => {
    await Promise.all([refreshAccounts(), refreshStatuses()]);
  }, [refreshAccounts, refreshStatuses]);

  const refreshDashboardStatus = useCallback(async () => {
    if (!accountsLoadedRef.current) {
      await refreshAccountCollection();
      return;
    }
    await refreshStatuses();
  }, [refreshAccountCollection, refreshStatuses]);

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

  const initializeWorkspace = useCallback(async () => {
    setError("");
    try {
      await refreshAccountCollection();
    } catch (err) {
      setError(formatAPIError(err, "刷新失败"));
    } finally {
      setLoading(false);
    }
  }, [refreshAccountCollection]);

  useEffect(() => {
    void initializeWorkspace();
  }, [initializeWorkspace]);

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
      void refreshStatuses().catch(() => undefined);
      if (selectedAccountId) {
        void refreshSnapshot(selectedAccountId).catch(() => undefined);
      }
    }, STATUS_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refreshSnapshot, refreshStatuses, selectedAccountId]);

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

  function updateCachedAccount(account?: Account) {
    if (!account) return;
    setAccounts((current) => current.map((item) => (item.id === account.id ? account : item)));
  }

  async function runAccountAction(action: "login" | "logout") {
    if (!selectedAccount) return;
    setBusyAction(action);
    setError("");
    try {
      const response = action === "login"
        ? await accountClient.loginAccount({ id: selectedAccount.id })
        : await accountClient.logoutAccount({ id: selectedAccount.id });
      updateCachedAccount(response.account);
      await refreshStatuses();
      await refreshSnapshot(selectedAccount.id, action === "login", { force: action === "login" });
      await refreshPolicy(selectedAccount.id);
    } catch (err) {
      setError(formatAPIError(err, "操作失败"));
    } finally {
      setBusyAction("");
    }
  }

  async function runAutomationToggle(accountId: string) {
    if (busyBulkAutomation) return;
    const account = accountsRef.current.find((item) => item.id === accountId);
    const status = statusesRef.current.get(accountId);
    const online = account ? accountConnected(account, status) : Boolean(status?.connected);
    setBusyAutomationAccountId(accountId);
    setError("");
    try {
      const response = online
        ? await accountClient.logoutAccount({ id: accountId })
        : await accountClient.loginAccount({ id: accountId });
      // Optimistic flip so the list button/badge update before the next poll.
      setStatuses((prev) => {
        const next = new Map(prev);
        const current = next.get(accountId);
        if (current) {
          next.set(accountId, {
            ...current,
            connected: !online,
            automationEnabled: !online,
            health: online ? "offline" : "online",
          });
        }
        return next;
      });
      setAccounts((prev) =>
        prev.map((item) => (
          item.id === accountId ? (response.account ?? { ...item, connected: !online }) : item
        )),
      );
      await refreshStatuses();
      if (accountId === selectedAccountId) {
        await refreshPolicy(accountId);
        await refreshSnapshot(accountId, !online, { force: !online });
      }
    } catch (err) {
      setError(formatAPIError(err, online ? "暂停失败" : "启动失败"));
      await refreshStatuses().catch(() => undefined);
    } finally {
      setBusyAutomationAccountId("");
    }
  }

  async function runAutomationStop(accountId: string) {
    if (busyBulkAutomation) return;
    setBusyAutomationAccountId(accountId);
    setError("");
    try {
      const response = await accountClient.logoutAccount({ id: accountId });
      setStatuses((prev) => {
        const next = new Map(prev);
        const current = next.get(accountId);
        if (current) {
          next.set(accountId, {
            ...current,
            connected: false,
            automationEnabled: false,
            health: "offline",
            lastError: "",
          });
        }
        return next;
      });
      setAccounts((prev) =>
        prev.map((item) => (
          item.id === accountId ? (response.account ?? { ...item, connected: false }) : item
        )),
      );
      await refreshStatuses();
      if (accountId === selectedAccountId) {
        await refreshPolicy(accountId);
        await refreshSnapshot(accountId, false);
      }
    } catch (err) {
      setError(formatAPIError(err, "停止失败"));
      await refreshStatuses().catch(() => undefined);
    } finally {
      setBusyAutomationAccountId("");
    }
  }

  async function runAutomationBulk(action: "start" | "pause") {
    if (busyBulkAutomation || busyAutomationAccountId) return;
    const wantOnline = action === "start";
    const targets = accountsRef.current.filter((account) => {
      const online = accountConnected(account, statusesRef.current.get(account.id));
      return online !== wantOnline;
    });
    if (targets.length === 0) return;

    setBusyBulkAutomation(action);
    setError("");
    const failures: string[] = [];
    let selectedTouched = false;

    try {
      for (const account of targets) {
        setBusyAutomationAccountId(account.id);
        try {
          const response = wantOnline
            ? await accountClient.loginAccount({ id: account.id })
            : await accountClient.logoutAccount({ id: account.id });
          setStatuses((prev) => {
            const next = new Map(prev);
            const current = next.get(account.id);
            if (current) {
              next.set(account.id, {
                ...current,
                connected: wantOnline,
                automationEnabled: wantOnline,
                health: wantOnline ? "online" : "offline",
                ...(wantOnline ? {} : { lastError: "" }),
              });
            }
            return next;
          });
          setAccounts((prev) =>
            prev.map((item) => (
              item.id === account.id ? (response.account ?? { ...item, connected: wantOnline }) : item
            )),
          );
          if (account.id === selectedAccountId) selectedTouched = true;
        } catch (err) {
          failures.push(
            `${accountNickname(account)}: ${formatAPIError(err, wantOnline ? "启动失败" : "暂停失败")}`,
          );
        }
      }

      await refreshStatuses();
      if (selectedTouched && selectedAccountId) {
        await refreshPolicy(selectedAccountId);
        await refreshSnapshot(selectedAccountId, wantOnline, { force: wantOnline });
      }
      if (failures.length > 0) {
        setError(
          failures.length === 1
            ? failures[0]
            : `${failures.length} 个账号失败：${failures.slice(0, 3).join("；")}${failures.length > 3 ? "…" : ""}`,
        );
      }
    } finally {
      setBusyAutomationAccountId("");
      setBusyBulkAutomation("");
    }
  }

  async function submitRedeemCode(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const code = redeemCode.trim();
    if (!code || accounts.length === 0 || redeemBusy) return;
    setRedeemBusy(true);
    setRedeemSummary("");
    setRedeemResults([]);
    setError("");
    try {
      const res = await accountClient.redeemCode({
        code,
        accountIds: accounts.map((account) => account.id),
      });
      setRedeemResults(
        res.results.map((item) => ({
          accountName: item.accountName || item.accountId,
          ok: item.ok,
          message: item.message || (item.ok ? "ok" : "失败"),
        })),
      );
      setRedeemSummary(`成功 ${res.successCount} / 失败 ${res.failureCount}（共 ${res.results.length} 个账号）`);
      await refreshStatuses().catch(() => undefined);
    } catch (err) {
      setRedeemSummary(formatAPIError(err, "兑换失败"));
    } finally {
      setRedeemBusy(false);
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
      await refreshAccountCollection();
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
      await refreshAccountCollection();
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
      await refreshStatuses();
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
            busyAutomationAccountId={busyAutomationAccountId}
            busyBulkAutomation={busyBulkAutomation}
            onRefresh={() => void refreshDashboardStatus()}
            onAdd={() => setAddOpen(true)}
            onRedeem={() => {
              setRedeemCode("");
              setRedeemSummary("");
              setRedeemResults([]);
              setRedeemOpen(true);
            }}
            onSelect={setSelectedAccountId}
            onAutomationToggle={(accountId) => void runAutomationToggle(accountId)}
            onAutomationStop={(accountId) => void runAutomationStop(accountId)}
            onBulkStart={() => void runAutomationBulk("start")}
            onBulkPause={() => void runAutomationBulk("pause")}
          />
        </aside>

        {hasAccounts && (
          <section className={cn("dark-scrollbar min-h-0 min-w-0 w-full xl:h-full xl:overflow-y-auto xl:pr-1", !selectedAccount && "hidden xl:block")}>
            {selectedAccount ? (
              <AccountDetailView
                account={selectedAccount}
                status={selectedStatus}
                featureCapabilities={featureCapabilities}
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

      <Dialog
        open={redeemOpen}
        onOpenChange={(open) => {
          if (redeemBusy) return;
          setRedeemOpen(open);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>兑换码</DialogTitle>
          </DialogHeader>
          <form className="space-y-4" onSubmit={(event) => void submitRedeemCode(event)}>
            <p className="text-sm text-muted-foreground">
              输入一次兑换码，将对当前账号列表中的全部 {accounts.length} 个账号依次兑换。离线账号会先尝试登录。
            </p>
            <Field label="兑换码">
              <Input
                value={redeemCode}
                onChange={(event) => setRedeemCode(event.target.value)}
                placeholder="粘贴兑换码"
                autoComplete="off"
                disabled={redeemBusy}
              />
            </Field>
            {redeemSummary && (
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-sm">{redeemSummary}</div>
            )}
            {redeemResults.length > 0 && (
              <div className="dark-scrollbar max-h-48 space-y-1.5 overflow-y-auto rounded-md border border-border/50 p-2 text-sm">
                {redeemResults.map((item) => (
                  <div key={`${item.accountName}-${item.message}`} className="flex items-start justify-between gap-2">
                    <span className="min-w-0 truncate font-medium">{item.accountName}</span>
                    <span className={cn("min-w-0 text-right", item.ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>
                      {item.ok ? (item.message && item.message !== "ok" ? item.message : "成功") : item.message}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setRedeemOpen(false)} disabled={redeemBusy}>
                关闭
              </Button>
              <Button type="submit" disabled={redeemBusy || !redeemCode.trim() || accounts.length === 0}>
                {redeemBusy ? <Loader2 className="size-4 animate-spin" /> : <Ticket className="size-4" />}
                {redeemBusy ? "兑换中" : "全部兑换"}
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
  busyAutomationAccountId,
  busyBulkAutomation,
  onRefresh,
  onAdd,
  onRedeem,
  onSelect,
  onAutomationToggle,
  onAutomationStop,
  onBulkStart,
  onBulkPause,
}: {
  accounts: Account[];
  statuses: Map<string, AccountStatus>;
  selectedAccountId: string;
  loading: boolean;
  quota: AccountQuota | null;
  busyAutomationAccountId: string;
  busyBulkAutomation: "" | "start" | "pause";
  onRefresh: () => void;
  onAdd: () => void;
  onRedeem: () => void;
  onSelect: (accountId: string) => void;
  onAutomationToggle: (accountId: string) => void;
  onAutomationStop: (accountId: string) => void;
  onBulkStart: () => void;
  onBulkPause: () => void;
}) {
  const hasAccounts = accounts.length > 0;
  const quotaReached = quota?.reached ?? false;
  const bulkBusy = busyBulkAutomation !== "";
  const automationLocked = bulkBusy || busyAutomationAccountId !== "";
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
            <Button type="button" variant="ghost" size="icon-sm" onClick={onRefresh} aria-label="刷新" disabled={loading || bulkBusy}>
              <RefreshCw className={cn("size-4", loading && "animate-spin")} />
            </Button>
            {hasAccounts && (
              <Button type="button" variant="ghost" size="icon-sm" onClick={onRedeem} aria-label="兑换码" disabled={bulkBusy}>
                <Ticket className="size-4" />
              </Button>
            )}
            {hasAccounts && (
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                onClick={onAdd}
                aria-label="新增账号"
                disabled={quotaReached || bulkBusy}
              >
                <Plus className="size-4" />
              </Button>
            )}
          </div>
        </div>
        {hasAccounts && (
          <div className="mt-2.5 flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              className="h-7 flex-1 px-2"
              aria-label="一键启动全部账号"
              disabled={automationLocked}
              onClick={onBulkStart}
            >
              {busyBulkAutomation === "start" ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Play className="size-3.5" />
              )}
              一键启动
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="h-7 flex-1 px-2"
              aria-label="一键暂停全部账号"
              disabled={automationLocked}
              onClick={onBulkPause}
            >
              {busyBulkAutomation === "pause" ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Pause className="size-3.5" />
              )}
              一键暂停
            </Button>
          </div>
        )}
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
              const identity = accountIdentity(account, status);
              const online = accountConnected(account, status);
              const abnormal = accountIsAbnormal(status);
              const automationBusy = bulkBusy || busyAutomationAccountId === account.id;
              const automationSpinning = busyAutomationAccountId === account.id;
              return (
                <div
                  key={account.id}
                  role="button"
                  tabIndex={0}
                  className={cn(
                    "w-full cursor-pointer rounded-md border p-3 text-left shadow-sm transition-all active:scale-[0.99]",
                    selected
                      ? "border-primary/45 bg-white/78 shadow-[0_10px_20px_rgba(255,111,97,0.12)] dark:bg-primary/12 dark:shadow-black/20"
                      : "border-border/58 bg-white/42 hover:border-ring/45 hover:bg-white/66 dark:bg-white/5 dark:hover:bg-white/8",
                  )}
                  onClick={() => onSelect(account.id)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onSelect(account.id);
                    }
                  }}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{identity.nickname}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                        <span>{identity.area}</span>
                        <span>{identity.channel}</span>
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center gap-1.5">
                      <Button
                        type="button"
                        variant={online ? "secondary" : "default"}
                        size="sm"
                        className="h-7 px-2"
                        aria-label={online ? "暂停并离线" : "启动并上线"}
                        disabled={automationBusy}
                        onClick={(event) => {
                          event.stopPropagation();
                          onAutomationToggle(account.id);
                        }}
                      >
                        {automationSpinning ? (
                          <Loader2 className="size-3.5 animate-spin" />
                        ) : online ? (
                          <Pause className="size-3.5" />
                        ) : (
                          <Play className="size-3.5" />
                        )}
                        {online ? "暂停" : "启动"}
                      </Button>
                      {abnormal && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="h-7 px-2"
                          aria-label="停止并离线"
                          disabled={automationBusy}
                          onClick={(event) => {
                            event.stopPropagation();
                            onAutomationStop(account.id);
                          }}
                        >
                          {automationSpinning ? (
                            <Loader2 className="size-3.5 animate-spin" />
                          ) : (
                            <Square className="size-3.5" />
                          )}
                          停止
                        </Button>
                      )}
                      <HealthBadge status={status} account={account} />
                    </div>
                  </div>
                </div>
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
  featureCapabilities,
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
  featureCapabilities: FeatureCapability[];
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
            capabilities={featureCapabilities}
            loading={policyLoading}
            saving={savingPolicy}
            message={policyMessage}
            onPolicyChange={onPolicyChange}
            onSave={onPolicySave}
          />
        </div>
      )}
      {activeTab === "race" && (
        <div className="min-h-0">
          <RaceTab snapshot={snapshot} policy={policy} />
        </div>
      )}
      {activeTab === "land" && (
        <div className="min-h-0">
          <LandTab snapshot={snapshot} policy={policy} />
        </div>
      )}
      {activeTab === "warehouse" && (
        <div className="min-h-0">
          <WarehouseTab snapshot={snapshot} />
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

function MonitorTab({
  snapshot,
  status,
}: {
  snapshot: GetSnapshotResponse | null;
  status?: AccountStatus;
}) {
  const runtimeStatistics = snapshot?.runtimeStatistics ?? status?.runtimeStatistics;
  return (
    <div className="space-y-3 sm:space-y-4">
      <StatusOverviewPanel snapshot={snapshot} status={status} />
      <RuntimeStatisticsPanel runtimeStatistics={runtimeStatistics} />
      <OperationPanel operations={snapshot?.plannedOperations ?? []} />
      <TaskOrderMonitorPanel tasks={snapshot?.pendingTasks ?? []} statistics={snapshot?.orderStatistics} />
      <CyclicNoteMonitorPanel activity={snapshot?.cyclicNote} />
      <CyclicStoryMonitorPanel activity={snapshot?.cyclicStory} />
      <DessertMonitorPanel activity={snapshot?.dessert} />
    </div>
  );
}

function RaceTab({
  snapshot,
  policy,
}: {
  snapshot: GetSnapshotResponse | null;
  policy: Policy | null;
}) {
  return (
    <div className="space-y-3 sm:space-y-4">
      <FmlRaceMonitorPanel race={snapshot?.fmlRace} showTakenTask={policy?.union?.race?.enabled ?? true} />
    </div>
  );
}

function LandTab({
  snapshot,
  policy,
}: {
  snapshot: GetSnapshotResponse | null;
  policy: Policy | null;
}) {
  return (
    <div className="space-y-3 sm:space-y-4">
      <LandMonitorPanel
        lands={snapshot?.lands ?? []}
        waterDrops={snapshot?.waterDrops ?? 0}
        waterDropsTotal={snapshot?.waterDropsTotal ?? 0}
        minWaterDrops={policy?.plant?.planting?.minWaterDrops ?? 0}
      />
    </div>
  );
}

function WarehouseTab({ snapshot }: { snapshot: GetSnapshotResponse | null }) {
  return (
    <div className="space-y-3 sm:space-y-4">
      <WarehouseMonitorPanel ledger={snapshot?.inventoryLedger} />
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
  const identity = accountIdentity(account, status);
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

function accountIdentity(account: Account, status?: AccountStatus) {
  return {
    nickname: accountNickname(account),
    area: accountAreaLabel(account, status),
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

function accountAreaLabel(account: Account, status?: AccountStatus) {
  const gsIdx = status?.gsIdx || account.gsIdx;
  if (gsIdx > 0) return `第${gsIdx}区`;
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

const SPEED_UP_TICKET_ITEM_ID = 1001;
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
  const speedUpTickets = snapshot?.inventory[SPEED_UP_TICKET_ITEM_ID] ?? 0;
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
  const apiNextLevelExperience = snapshot?.nextLevelExperience ?? status?.nextLevelExperience ?? 0;
  const apiLevelMaxed = snapshot?.levelMaxed ?? status?.levelMaxed ?? false;
  const apiHasNextLevel = apiLevelMaxed || apiNextLevelExperience > 0;
  const localNextLevel = experienceToNextLevel(level, experience);
  const levelMaxed = apiHasNextLevel ? apiLevelMaxed : localNextLevel.maxed;
  const nextLevelExperience = apiHasNextLevel ? apiNextLevelExperience : localNextLevel.required;
  const experienceToNext = apiHasNextLevel
    ? (snapshot?.experienceToNextLevel ?? status?.experienceToNextLevel ?? 0)
    : localNextLevel.remaining;
  const reputationDetail = reputationObserved ? (reputationTime ? `同步 ${formatUnixTime(reputationTime)}` : "已同步") : "未同步";
  const nextLevelValue = levelMaxed
    ? "已满级"
    : nextLevelExperience > 0
      ? `${formatCount(experienceToNext)} 经验`
      : "-";
  const nextLevelDetail = levelMaxed
    ? "已达最高等级"
    : nextLevelExperience > 0
      ? `当前 ${formatCount(experience)} / 需要 ${formatCount(nextLevelExperience)}`
      : undefined;
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
        <OverviewStat
          icon={<TrendingUp />}
          label="距下一等级"
          value={nextLevelValue}
          detail={nextLevelDetail}
          wrap
          compact
        />
        <OverviewStat icon={<Waves />} label="水滴" value={`${formatCount(snapshot?.waterDrops ?? 0)}/${formatCount(snapshot?.waterDropsTotal ?? 0)}`} />
        <OverviewStat icon={<Gem />} label="元宝" value={formatCount(snapshot?.diamondsFree ?? 0)} />
        <OverviewStat icon={<Coins />} label="金币" value={formatCount(snapshot?.gold ?? 0)} />
        <OverviewStat icon={<HandCoins />} label="花坊币" value={formatCount(floralCoins)} />
        <OverviewStat icon={<Ticket />} label="加速卡" value={formatCount(speedUpTickets)} />

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
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) {
    return null;
  }

  const activeTasks = activity.tasks.filter((task) => task.unlocked);
  const readyTasks = activity.valid ? activeTasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "花笺集芳"}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyTasks + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyTasks + readyMilestones}</Badge>}
        </>
      }
    >
      {!activity.observed ? (
        <EmptyState title="花笺集芳状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
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

function FmlRaceMonitorPanel({ race, showTakenTask }: { race?: FmlRaceView; showTakenTask: boolean }) {
  const tasks = race?.tasks ?? [];
  const taken = race?.taken;
  const observed = race?.observed ?? false;
  const batchActive = race?.batchActive ?? false;
  const batchStartMs = race?.batchStartMs ?? BigInt(0);
  const batchEndMs = race?.batchEndMs ?? BigInt(0);
  const taskQuotaObserved = race?.taskQuotaObserved ?? false;
  const finishedTaskNum = race?.finishedTaskNum ?? 0;
  const totalTaskNum = race?.totalTaskNum ?? 0;

  const formatMs = (ms: bigint) => {
    if (ms === BigInt(0)) return "";
    return new Date(Number(ms)).toLocaleString("zh-CN", {
      month: "numeric",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <CollapsibleCard
      title="公会竞赛"
      contentClassName="space-y-3"
      actions={
        <>
          {!observed ? (
            <Badge variant="outline">等待同步</Badge>
          ) : !batchActive ? (
            <Badge variant="outline">非竞赛期间</Badge>
          ) : (
            <Badge variant="secondary">竞赛进行中</Badge>
          )}
          {taskQuotaObserved && totalTaskNum > 0 && (
            <Badge variant="outline">
              已做 {finishedTaskNum}/{totalTaskNum}
            </Badge>
          )}
          {taskQuotaObserved && totalTaskNum <= 0 && (
            <Badge variant="outline">已做 {finishedTaskNum}</Badge>
          )}
          {showTakenTask && taken?.hasTask && <Badge variant="secondary">已接任务</Badge>}
          {tasks.length > 0 && <Badge variant="outline">{tasks.length} 个可选</Badge>}
        </>
      }
    >
      {!observed ? (
        <EmptyState title="竞赛状态尚未同步" detail="连接游戏并进入公会界面后，竞赛任务列表会自动同步。" />
      ) : !batchActive ? (
        <EmptyState
          title="当前不在竞赛批次中"
          detail={
            batchStartMs > BigInt(0) && batchEndMs > BigInt(0)
              ? `竞赛按批次开放，非竞赛期间任务池不可用。当前批次：${formatMs(batchStartMs)} ~ ${formatMs(batchEndMs)}`
              : "竞赛按批次开放，非竞赛期间任务池不可用。"
          }
        />
      ) : (
        <>
          {taskQuotaObserved && (
            <div className="flex items-center justify-between gap-2 rounded-md border border-border/58 bg-white/34 px-3 py-2 text-sm dark:bg-white/5">
              <span className="text-muted-foreground">任务次数</span>
              <span className="font-medium">
                {totalTaskNum > 0 ? `已做 ${finishedTaskNum} / 总 ${totalTaskNum}` : `已做 ${finishedTaskNum}`}
              </span>
            </div>
          )}

          {showTakenTask &&
            (taken?.hasTask ? (
              <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
                <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
                  <span>当前已接任务</span>
                </div>
                <div className="p-3">
                  <FmlRaceTakenCard taken={taken} />
                </div>
              </section>
            ) : (
              <div className="rounded-md border border-dashed border-border/58 px-3 py-2 text-sm text-muted-foreground">当前未接取任务</div>
            ))}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span>任务池</span>
                {(race?.tasksSyncedAtMs ?? BigInt(0)) > BigInt(0) && (
                  <span className="text-xs font-normal text-muted-foreground">
                    更新于{" "}
                    {new Date(Number(race!.tasksSyncedAtMs)).toLocaleString("zh-CN", {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}{" "}
                    · 每 10 分钟重新获取
                  </span>
                )}
              </div>
              <Badge variant="secondary">{tasks.length} 个</Badge>
            </div>
            {tasks.length === 0 ? (
              <div className="p-3">
                <EmptyState title="任务池为空" detail="竞赛任务已接完或尚未刷新。" />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {tasks.map((task, index) => (
                  <FmlRaceTaskCard key={task.msId} index={index + 1} task={task} />
                ))}
              </div>
            )}
          </section>
        </>
      )}
    </CollapsibleCard>
  );
}

function FmlRaceTakenCard({ taken }: { taken: FmlRaceTaken }) {
  const [nowMs, setNowMs] = useState<number | null>(null);

  useEffect(() => {
    const updateNow = () => setNowMs(Date.now());
    updateNow();
    const timer = window.setInterval(updateNow, 1000);
    return () => window.clearInterval(timer);
  }, []);

  const progress = taken.targetCnt > 0 ? Math.min(100, Math.round((taken.finishCnt / taken.targetCnt) * 100)) : 0;
  const title = taken.targetLabel
    ? `${taken.taskLabel || `任务 #${taken.taskId}`} · ${taken.targetLabel}`
    : taken.taskLabel || `任务 #${taken.taskId}`;
  const expireMs = Number(taken.expireTimeMs ?? BigInt(0));
  const remainMs = expireMs > 0 && nowMs !== null ? expireMs - nowMs : 0;
  const expireUrgent = expireMs > 0 && nowMs !== null && remainMs > 0 && remainMs <= 10 * 60 * 1000 && progress < 100;
  const expireLabel =
    expireMs > 0
      ? new Date(expireMs).toLocaleString("zh-CN", {
          month: "numeric",
          day: "numeric",
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })
      : "";
  const remainLabel = (() => {
    if (expireMs <= 0 || nowMs === null) return "";
    if (remainMs <= 0) return "已过期";
    const totalSec = Math.floor(remainMs / 1000);
    const h = Math.floor(totalSec / 3600);
    const m = Math.floor((totalSec % 3600) / 60);
    if (h > 0) return `剩余 ${h}小时${m}分`;
    if (m > 0) return `剩余 ${m}分钟`;
    return `剩余 ${totalSec}秒`;
  })();
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{title}</span>
        <Badge variant={progress >= 100 ? "secondary" : "outline"}>{progress}%</Badge>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} />
      </div>
      <div className="text-xs text-muted-foreground">
        进度 {taken.finishCnt} / {taken.targetCnt} · 分数 {taken.score}
      </div>
      <div className={`text-xs ${expireUrgent ? "font-medium text-amber-700 dark:text-amber-400" : "text-muted-foreground"}`}>
        {expireLabel !== "" ? (
          <>
            {progress >= 100 ? "已完成，待提交" : expireUrgent ? "即将过期" : "过期时间"}：{expireLabel}
            {remainLabel !== "" && progress < 100 ? `（${remainLabel}）` : null}
          </>
        ) : (
          "过期时间：等待同步任务时长"
        )}
      </div>
    </div>
  );
}

function FmlRaceTaskCard({ index, task }: { index: number; task: FmlRaceTask }) {
	const skipReason = (task.takeSkipReason ?? "").trim();
	// Empty = ready now. "冷却中…后可接" = passes filters, waiting on AppearTime.
	// Both are tasks automation would take; other skip reasons are hard rejects.
	const takeable = skipReason === "" || skipReason.startsWith("冷却中");
	// The server computes CD using the same lead window as task selection. Using
	// that snapshot keeps rendering pure and the label consistent with automation.
	const onCd = skipReason.startsWith("冷却中") || skipReason.endsWith("后刷新");
	const baseTitle = task.targetLabel
		? `${task.taskLabel || `任务 #${task.taskId}`} · ${task.targetLabel}`
		: task.taskLabel || `任务 #${task.taskId}`;
	const title = onCd ? `CD ${baseTitle}` : baseTitle;
  return (
    <div
      className={cn(
        "rounded-md border-2 bg-white/36 px-3 py-2 dark:bg-white/5",
        takeable ? "border-red-500 bg-red-500/5 dark:bg-red-500/10" : "border-border/55",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 text-sm font-medium">
          <span className="mr-1.5 tabular-nums text-muted-foreground">{index}.</span>
          {title}
        </span>
        <Badge variant={task.isUpgrade ? "secondary" : "outline"}>{task.isUpgrade ? "已升级" : "普通"}</Badge>
      </div>
      <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
        <span>分数 {task.score}</span>
        {task.upgradeUid > 0 && <span>升级人 #{task.upgradeUid}</span>}
      </div>
      {skipReason === "" ? (
        <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">可接取</div>
      ) : skipReason.startsWith("冷却中") ? (
        <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{skipReason}</div>
      ) : (
        <div className="mt-1 text-xs text-muted-foreground">不可接取：{skipReason}</div>
      )}
    </div>
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

function CyclicStoryMonitorPanel({ activity }: { activity?: CyclicStoryView }) {
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) {
    return null;
  }

  const activeOrders = activity.orders.filter((order) => order.orderId > 0 && !order.onCooldown);
  const readyOrders = activity.valid ? activeOrders.filter((order) => order.status === PlanStatus.READY).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "莳花纪闻"}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyOrders + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyOrders + readyMilestones}</Badge>}
        </>
      }
    >
      {!activity.observed ? (
        <EmptyState title="莳花纪闻状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.valid ? (
        <EmptyState title="莳花纪闻配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示订单详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat
              icon={<CalendarDays />}
              label="活动阶段"
              value={cyclicNotePhaseLabel(phase)}
              detail={cyclicStoryPhaseDetail(activity)}
            />
            <OverviewStat icon={<Trophy />} label="累计积分" value={formatCount(activity.score)} detail={`完成订单 ${formatCount(activity.finishCount)} 次`} />
            <OverviewStat
              icon={<Flower2 />}
              label="花史残页"
              value={formatCount(activity.currencyBalance)}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "活动货币未识别"}
            />
            <OverviewStat
              icon={<ListChecks />}
              label="订单槽"
              value={`${activeOrders.length}/${activity.orders.length}`}
              detail={readyOrders > 0 ? `${readyOrders} 个订单可交` : activity.ordersObserved ? "已同步" : "等待进入活动同步"}
            />
          </div>

          {activity.description && <div className="rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>订单详情</span>
              <Badge variant="secondary">{activity.orders.length} 槽</Badge>
            </div>
            {activity.orders.length === 0 ? (
              <div className="p-3">
                <EmptyState title={activity.ordersObserved ? "当前没有订单槽" : "订单列表尚未同步"} />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.orders.map((order) => (
                  <CyclicStoryOrderCard key={`${activity.batchId}:${order.orderIdx}:${order.orderId}`} order={order} />
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
              <div className="p-3"><EmptyState title="当前没有里程碑" /></div>
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

function CyclicStoryOrderCard({ order }: { order: CyclicStoryOrder }) {
  if (order.onCooldown || order.orderId <= 0) {
    return (
      <div className="rounded-md border border-dashed border-border/70 bg-muted/15 p-3 text-sm text-muted-foreground">
        <div className="flex items-center justify-between gap-2">
          <span className="font-medium">订单槽 {order.orderIdx}</span>
          <Badge variant="outline">{order.onCooldown ? "冷却中" : "空闲"}</Badge>
        </div>
        <div className="mt-3 text-xs">仅监控，不会自动付费刷新或清 CD。</div>
      </div>
    );
  }
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">订单槽 {order.orderIdx} · #{order.orderId}</div>
          <div className="mt-1 line-clamp-2 font-medium">
            {order.flowerId > 0 ? `${itemName(order.flowerId)} x${formatCount(order.cost)}` : `订单 #${order.orderId}`}
          </div>
        </div>
        <CyclicNoteStatusBadge status={order.status} received={false} unknown={!order.catalogKnown} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {order.reward.length > 0 ? order.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
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

function LandMonitorPanel({
  lands,
  waterDrops,
  waterDropsTotal,
  minWaterDrops,
}: {
  lands: LandView[];
  waterDrops: number;
  waterDropsTotal: number;
  minWaterDrops: number;
}) {
  const landsByDisplay = useMemo(() => {
    const map = new Map<number, LandView>();
    for (const land of lands) {
      map.set(landDisplayNumber(land.landId), land);
    }
    return map;
  }, [lands]);
  const mapSlots = useMemo(() => {
    // 8×8 map order: left 1-32 by rows of 4, right 33-64 by rows of 4.
    // Row 0: 1-4, 33-36 … Row 7: 29-32, 61-64
    const slots: number[] = [];
    for (let row = 0; row < 8; row++) {
      for (let i = 0; i < 4; i++) slots.push(row * 4 + 1 + i);
      for (let i = 0; i < 4; i++) slots.push(33 + row * 4 + i);
    }
    return slots;
  }, []);
  const recommendationCounts = useMemo(() => {
    const stats = new Map<string, number>();
    for (const land of lands) {
      if (land.landStatus !== "opened") continue;
      stats.set(land.recommendation || "unknown", (stats.get(land.recommendation || "unknown") ?? 0) + 1);
    }
    return stats;
  }, [lands]);
  const availableWaterDrops = Math.max(0, waterDrops - minWaterDrops);
  const openedCount = lands.filter((land) => land.landStatus === "opened").length;
  const unopenedCount = lands.filter((land) => land.landStatus === "unopened").length;
  const lockedCount = lands.filter((land) => land.landStatus === "locked").length;
  const statusOrder = ["harvest", "plant", "water", "wait"] as const;

  return (
    <CollapsibleCard
      title="土地"
      actions={
        <>
          <Badge variant="secondary">已开 {openedCount}</Badge>
          {unopenedCount > 0 && <Badge variant="outline">未开 {unopenedCount}</Badge>}
          {lockedCount > 0 && <Badge variant="outline">锁定 {lockedCount}</Badge>}
        </>
      }
    >
      {lands.length === 0 ? (
        <EmptyState title="暂无土地快照" />
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {statusOrder.map((key) => {
              const count = recommendationCounts.get(key) ?? 0;
              return (
                <Fragment key={key}>
                  {count > 0 && (
                    <Badge variant="outline">
                      {recommendationLabel(key)} {count}
                    </Badge>
                  )}
                  {key === "plant" && (
                    <Badge variant="outline">
                      水滴总数 {formatCount(waterDrops)}/{formatCount(waterDropsTotal)}
                    </Badge>
                  )}
                </Fragment>
              );
            })}
            {minWaterDrops > 0 && (
              <Badge variant="outline">可用水滴数 {formatCount(availableWaterDrops)}</Badge>
            )}
          </div>
          <div className="dark-scrollbar max-h-[440px] overflow-y-auto pr-0.5 sm:h-[560px] sm:max-h-none sm:pr-1">
            <div
              className="grid gap-2"
              style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr)) 0.75rem repeat(4, minmax(0, 1fr))" }}
            >
              {mapSlots.flatMap((display, index) => {
                const tile = (() => {
                  const land = landsByDisplay.get(display);
                  if (!land) {
                    return (
                      <div
                        key={`slot-${display}`}
                        className="flex min-h-[78px] items-center justify-center rounded-md border border-dashed border-border/45 text-xs text-muted-foreground"
                      >
                        #{display}
                      </div>
                    );
                  }
                  return <LandTile key={land.landId} land={land} />;
                })();
                if (index % 8 !== 4) return [tile];
                return [<div key={`aisle-${index}`} className="min-h-[78px]" aria-hidden />, tile];
              })}
            </div>
          </div>
        </div>
      )}
    </CollapsibleCard>
  );
}

function LandTile({ land }: { land: LandView }) {
  const planted = land.flowerId > 0;
  const status = land.landStatus || (land.observed ? "opened" : "unknown");
  const opened = status === "opened";
  const recommendation = recommendationLabel(land.recommendation);
  const timing = landTimingLabel(land, status);
  return (
    <div
      className={cn(
        "min-h-[78px] rounded-md border border-border/58 bg-white/58 p-1.5 shadow-sm transition-colors dark:bg-white/6 sm:p-2",
        opened && land.recommendation === "harvest" && "border-primary/50 bg-primary/8",
        opened && land.recommendation === "water" && "border-sky-300/70 bg-sky-50/72 dark:bg-sky-500/10",
        opened && land.recommendation === "plant" && "border-amber-300/70 bg-amber-50/76 dark:bg-amber-400/10",
        !opened && "opacity-70",
        !land.observed && opened && "opacity-70",
      )}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="min-w-0">
          <div className="font-mono text-xs font-medium sm:text-sm">{landDisplayName(land.landId)}</div>
        </div>
        <Badge variant={opened && land.recommendation === "harvest" ? "secondary" : "outline"} className="h-5 shrink-0 px-1 text-[10px] sm:px-1.5 sm:text-[11px]">
          {opened ? recommendation : landStatusLabel(status)}
        </Badge>
      </div>
      <div className="mt-1 truncate text-xs sm:text-sm">{opened ? (planted ? itemName(land.flowerId) : "空地") : landStatusLabel(status)}</div>
      <div className="mt-1 text-[10px] text-muted-foreground sm:text-xs">
        <div className="truncate">
          {opened ? (
            <>
              {land.lvl ? `${land.lvl}级` : "-"}
              {planted ? ` · 收${land.harvestCnt || 0}` : ""}
            </>
          ) : land.openLevel > 0 ? (
            `${land.openLevel}级解锁`
          ) : (
            "-"
          )}
        </div>
        <div className="text-left">{timing}</div>
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

function WarehouseMonitorPanel({ ledger }: { ledger?: InventoryLedgerView }) {
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
    <CollapsibleCard
      title="仓库"
      actions={
        inventoryItems.length > 0 ? (
          <>
            <Badge variant="secondary">种类 {inventoryItems.length}</Badge>
            {inventoryQuery.trim() && <Badge variant="outline">匹配 {visibleItems.length}</Badge>}
          </>
        ) : undefined
      }
    >
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
    </CollapsibleCard>
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




function EventPanel({ events }: { events: Event[] }) {
  const [activeCategory, setActiveCategory] = useState("all");
  const displayEvents = useMemo(() => collapseRaceSyncLogEvents(events), [events]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const event of displayEvents) {
      const category = eventCategory(event);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [displayEvents]);
  const categories = useMemo(() => {
    const order = ["basic", "water", "plant", "order", "union", "race", "activity", "account", "system"];
    const keys = new Set(categoryCounts.keys());
    return [...keys].sort((a, b) => {
      const ai = order.indexOf(a);
      const bi = order.indexOf(b);
      if (ai >= 0 && bi >= 0) return ai - bi;
      if (ai >= 0) return -1;
      if (bi >= 0) return 1;
      return a.localeCompare(b);
    });
  }, [categoryCounts]);
  const visibleEvents = useMemo(() => {
    if (activeCategory === "all") return displayEvents;
    return displayEvents.filter((event) => eventCategory(event) === activeCategory);
  }, [activeCategory, displayEvents]);

  useEffect(() => {
    if (activeCategory !== "all" && !categories.includes(activeCategory)) {
      setActiveCategory("all");
    }
  }, [activeCategory, categories]);

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

function OverviewStat({
  icon,
  label,
  value,
  detail,
  wrap = false,
  compact = false,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  wrap?: boolean;
  compact?: boolean;
}) {
  return (
    <div className="flex min-h-[72px] min-w-0 items-center gap-2 rounded-md border border-border/55 bg-white/52 px-2.5 py-2 shadow-sm transition-colors hover:bg-white/68 dark:bg-white/6 dark:hover:bg-white/9 sm:min-h-[76px] sm:gap-3 sm:px-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 shadow-sm dark:bg-white/8 dark:text-sky-300 sm:size-9 [&_svg]:size-4">{icon}</div>
      <div className="min-w-0 flex-1">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div
          className={cn(
            "font-semibold tabular-nums",
            compact ? "text-sm sm:text-base" : "text-base sm:text-lg",
            wrap ? "whitespace-normal break-all" : "truncate",
          )}
        >
          {value}
        </div>
        {detail && (
          <div className={cn("text-xs text-muted-foreground", wrap ? "whitespace-normal break-all" : "truncate")}>{detail}</div>
        )}
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


function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}

function accountIsAbnormal(status?: AccountStatus) {
  if (accountStatusIssues(status).length > 0) return true;
  return status?.health === "blocked" || status?.health === "session_expired" || Boolean(status?.lastError);
}

function HealthBadge({ account, status }: { account: Account; status?: AccountStatus }) {
  const connected = accountConnected(account, status);
  if (accountIsAbnormal(status)) return <Badge variant="destructive">异常</Badge>;
  if (!connected) return <Badge variant="outline">离线</Badge>;
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

function cyclicStoryPhaseDetail(activity: CyclicStoryView) {
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
  if (event.category === "flower_art") return "order";
  if (event.category === "redeem") return "system";
  if (event.category) return event.category;
  if (event.domain) {
    const category = event.domain.split(".")[0];
    if (category === "redeem") return "system";
    return category || "system";
  }
  return "system";
}

/** Race getTaskList completions are frequent; keep only the newest one (events are newest-first). */
function collapseRaceSyncLogEvents(events: Event[]): Event[] {
  let keptLatestSyncComplete = false;
  return events.filter((event) => {
    if (isRaceSyncPlannedLogEvent(event)) return false;
    if (!isRaceSyncCompleteLogEvent(event)) return true;
    if (keptLatestSyncComplete) return false;
    keptLatestSyncComplete = true;
    return true;
  });
}

function isRaceSyncLogEvent(event: Event) {
  if (event.domain === "union.race.sync" || event.kind === "race_task_sync") return true;
  const title = eventTitle(event);
  const message = eventMessage(event);
  return title.includes("同步竞赛任务") || message.includes("同步竞赛任务");
}

function isRaceSyncCompleteLogEvent(event: Event) {
  if (!isRaceSyncLogEvent(event)) return false;
  if (event.kind === "operation_planned") return false;
  if (event.kind === "race_task_sync" || event.kind === "operation_ack") return true;
  const title = eventTitle(event);
  const message = eventMessage(event);
  return title === "同步竞赛任务" || message.includes("同步竞赛任务 完成") || message === "完成";
}

function isRaceSyncPlannedLogEvent(event: Event) {
  return event.kind === "operation_planned" && isRaceSyncLogEvent(event);
}

function eventTitle(event: Event) {
  if (event.label) return event.label;
  if (event.kind === "order_satin_finish") return "绸缎订单";
  if (event.kind === "order_decorate_finish") return "建材订单";
  if (event.kind === "waterwheel") return "水车水滴";
  if (event.kind === "free_water") return "限时水滴";
  if (event.domain?.includes("resident.satin")) return "绸缎订单";
  if (event.domain?.includes("resident.decorate")) return "建材订单";
  return [event.domain, event.action].filter(Boolean).join(".") || event.kind || "-";
}

function eventMessage(event: Event) {
  return event.message || event.payloadJson || "";
}

function categoryLabel(category: string) {
  switch (category) {
    case "basic":
      return "基础";
    case "water":
      return "水滴";
    case "plant":
      return "种植";
    case "order":
      return "订单";
    case "union":
      return "公会";
    case "race":
      return "竞赛";
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

function landDisplayNumber(landId: number) {
  if (landId >= 1001 && landId < 2000) return landId - 1000;
  return landId;
}

function landDisplayName(landId: number) {
  return `#${landDisplayNumber(landId)}`;
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


function truncateMiddle(value: string, head: number, tail: number) {
  if (value.length <= head + tail + 1) return value;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}
