"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ComponentProps, type FormEvent, type ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { QRCodeSVG } from "qrcode.react";
import { AlertTriangle, ArrowLeft, BadgeCheck, BarChart3, CalendarDays, Cloud, Loader2, LogOut, Package, Pause, Play, Plus, RefreshCw, Send, ShieldCheck, Square, Sprout, Ticket, Trash2, Trophy } from "lucide-react";
import { AccountService, AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import { QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import type { AccountStatus, Event, FeatureCapability, GetSnapshotResponse } from "@/gen/mygardenworld/v1/query_service_pb";
import AppShell from "@/components/app-shell";
import PolicyPanel from "@/components/dashboard/policy-panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatAPIError, transport } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { cn } from "@/lib/utils";
import { StatusOverviewPanel, RuntimeStatisticsPanel, TaskOrderMonitorPanel, LandMonitorPanel, FmlLandMonitorPanel, WarehouseMonitorPanel, BusinessStatisticsPanel, OperationPanel, EventPanel, Field } from "@/components/dashboard/monitor-panels";
import { CyclicNoteMonitorPanel, FmlRaceMonitorPanel, CyclicStoryMonitorPanel, DessertMonitorPanel } from "@/components/dashboard/activity-monitor-panels";
import { accountIdentity, accountNickname, alipayLoginStatusLabel, accountConnected, isRunnerNotStartedError, isTransientConnectionMessage, waitForAbortableDelay, accountIsAbnormal, HealthBadge, accountStatusIssues } from "@/components/dashboard/dashboard-utils";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);
const queryClient = createClient(QueryService, transport);

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

type DashboardTabId = "monitor" | "settings" | "logs" | "race" | "land" | "warehouse" | "business";
type AccountQuota = {
  current: number;
  max: number;
  reached: boolean;
};
const DASHBOARD_TABS: { id: DashboardTabId; label: string; icon: ReactNode }[] = [
  { id: "monitor", label: "监控", icon: <BadgeCheck /> },
  { id: "settings", label: "设置", icon: <ShieldCheck /> },
  { id: "logs", label: "日志", icon: <CalendarDays /> },
  { id: "race", label: "公会竞赛", icon: <Trophy /> },
  { id: "land", label: "土地", icon: <Sprout /> },
  { id: "warehouse", label: "仓库", icon: <Package /> },
  { id: "business", label: "营业统计", icon: <BarChart3 /> },
];


const EMPTY_ADD_FORM = {
  channel: Channel.IOS,
  name: "",
  username: "",
  password: "",
};

type AddAccountForm = typeof EMPTY_ADD_FORM;
type AlipayQRState = {
  loginId: string;
  content: string;
  status: AlipayLoginStatus;
  error: string;
};

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
  const [alipayQR, setAlipayQR] = useState<AlipayQRState | null>(null);
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
  // Bumped on each getPolicy so a slow response from account A cannot overwrite
  // the panel after the user has switched to account B (and then save A's policy
  // onto B).
  const policyFetchGenRef = useRef(0);
  const policyOwnerAccountIdRef = useRef("");

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
    const fetchGen = ++policyFetchGenRef.current;
    if (!accountId) {
      policyOwnerAccountIdRef.current = "";
      setPolicy(null);
      setPolicyLoading(false);
      return;
    }
    setPolicyLoading(true);
    try {
      const res = await policyClient.getPolicy({ accountId });
      if (fetchGen !== policyFetchGenRef.current) return;
      policyOwnerAccountIdRef.current = accountId;
      setPolicy(res.policy ?? create(PolicySchema));
      setPolicyMessage("");
    } catch (err) {
      if (fetchGen !== policyFetchGenRef.current) return;
      policyOwnerAccountIdRef.current = "";
      setPolicy(null);
      setPolicyMessage(formatAPIError(err, "读取策略失败"));
    } finally {
      if (fetchGen === policyFetchGenRef.current) {
        setPolicyLoading(false);
      }
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
    if (
      !addOpen ||
      addForm.channel !== Channel.ALIPAY ||
      !alipayQR?.loginId ||
      (alipayQR.status !== AlipayLoginStatus.WAITING_FOR_SCAN && alipayQR.status !== AlipayLoginStatus.PROCESSING)
    ) {
      return;
    }
    let active = true;
    let polling = false;

    const poll = async () => {
      if (polling) return;
      polling = true;
      try {
        const response = await accountClient.pollAlipayLogin({ loginId: alipayQR.loginId });
        if (!active) return;
        if (response.status === AlipayLoginStatus.COMPLETE && response.account) {
          setSelectedAccountId(response.account.id);
          setAddOpen(false);
          setAddForm(EMPTY_ADD_FORM);
          setAlipayQR(null);
          await refreshAccountCollection();
          return;
        }
        setAlipayQR((current) => current && current.loginId === alipayQR.loginId
          ? { ...current, status: response.status, error: response.loginError }
          : current);
      } catch (err) {
        if (active) {
          setAlipayQR((current) => current && current.loginId === alipayQR.loginId
            ? { ...current, status: AlipayLoginStatus.FAILED, error: formatAPIError(err, "扫码登录失败") }
            : current);
        }
      } finally {
        polling = false;
      }
    };

    void poll();
    const timer = window.setInterval(() => void poll(), 1000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [addForm.channel, addOpen, alipayQR?.loginId, alipayQR?.status, refreshAccountCollection]);

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
      policyFetchGenRef.current += 1;
      policyOwnerAccountIdRef.current = "";
      setSnapshot(null);
      setPolicy(null);
      setPolicyLoading(false);
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
  }, [refreshSnapshot, selectedAccountId, selectedConnected]);

  useEffect(() => {
    if (!selectedAccountId) {
      return;
    }
    // Drop the previous account's editable policy immediately so a late getPolicy
    // cannot leave the wrong blob on screen (or get saved onto this account).
    policyOwnerAccountIdRef.current = "";
    setPolicy(null);
    setPolicyMessage("");
    void refreshPolicy(selectedAccountId);
  }, [refreshPolicy, selectedAccountId]);

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
    if (addForm.channel === Channel.ALIPAY) {
      if (!alipayQR || alipayQR.status === AlipayLoginStatus.EXPIRED || alipayQR.status === AlipayLoginStatus.FAILED) {
        await startAlipayLogin();
      }
      return;
    }
    if (accountQuota?.reached) {
      setError(`账号已满（${accountQuota.current}/${accountQuota.max}）`);
      return;
    }
    if (!addForm.username.trim() || !addForm.password) return;
    setBusyAction("create");
    setError("");
    try {
      const res = await accountClient.createAccount({
        name: addForm.name.trim(),
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

  async function startAlipayLogin() {
    setBusyAction("create");
    setError("");
    setAlipayQR(null);
    try {
      const response = await accountClient.startAlipayLogin({});
      setAlipayQR({
        loginId: response.loginId,
        content: response.qrContent,
        status: response.status,
        error: "",
      });
    } catch (err) {
      setError(formatAPIError(err, "获取支付宝二维码失败"));
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
      policyFetchGenRef.current += 1;
      policyOwnerAccountIdRef.current = "";
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
    const accountId = selectedAccount.id;
    if (policyOwnerAccountIdRef.current !== accountId) {
      setPolicyMessage("策略尚未与当前账号对齐，请等待加载完成后再保存");
      return;
    }
    setSavingPolicy(true);
    setPolicyMessage("");
    try {
      const res = await policyClient.setPolicy({ accountId, policy });
      if (policyOwnerAccountIdRef.current !== accountId) return;
      setPolicy(res.policy ?? policy);
      setPolicyMessage("");
      await refreshStatuses();
      if (policyOwnerAccountIdRef.current === accountId) {
        await refreshSnapshot(accountId);
      }
    } catch (err) {
      if (policyOwnerAccountIdRef.current === accountId) {
        setPolicyMessage(formatAPIError(err, "保存失败"));
      }
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
          <section
            className={cn(
              "min-h-0 min-w-0 w-full xl:flex xl:h-full xl:flex-col xl:overflow-hidden xl:pr-1",
              !selectedAccount && "hidden xl:block",
            )}
          >
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

      <Dialog
        open={addOpen}
        onOpenChange={(open) => {
          setAddOpen(open);
          if (!open) {
            setAddForm(EMPTY_ADD_FORM);
            setAlipayQR(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增账号</DialogTitle>
          </DialogHeader>
          <form className="space-y-4" onSubmit={createAccount}>
            <Field label="渠道">
              <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="渠道">
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
                  onClick={() => {
                    setAddForm((prev) => ({ ...prev, channel: Channel.IOS }));
                    setAlipayQR(null);
                  }}
                  disabled={creatingAccount}
                >
                  iOS
                </button>
                <button
                  type="button"
                  role="radio"
                  aria-checked={addForm.channel === Channel.ALIPAY}
                  className={cn(
                    "h-10 rounded-md border px-3 text-sm font-medium transition-colors",
                    addForm.channel === Channel.ALIPAY
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border/70 text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => {
                    setAddForm((prev) => ({ ...prev, channel: Channel.ALIPAY }));
                    setAlipayQR(null);
                  }}
                  disabled={creatingAccount}
                >
                  支付宝
                </button>
              </div>
            </Field>
            {addForm.channel === Channel.ALIPAY ? (
              <>
                <div className="rounded-md border border-border/60 bg-white/52 p-4 text-center dark:bg-white/5">
                  {alipayQR?.content ? (
                    <div className="space-y-3">
                      <div className="mx-auto w-fit rounded-md bg-white p-3 shadow-sm">
                        <QRCodeSVG value={alipayQR.content} size={208} level="M" />
                      </div>
                      <div className="text-sm font-medium">{alipayLoginStatusLabel(alipayQR.status)}</div>
                      {alipayQR.error && <div className="text-xs text-destructive">{alipayQR.error}</div>}
                      {(alipayQR.status === AlipayLoginStatus.EXPIRED || alipayQR.status === AlipayLoginStatus.FAILED) && (
                        <Button type="button" variant="outline" size="sm" onClick={() => void startAlipayLogin()} disabled={creatingAccount}>
                          <RefreshCw className="size-4" />
                          重新获取
                        </Button>
                      )}
                    </div>
                  ) : (
                    <div className="py-6 text-sm text-muted-foreground">点击下方按钮生成二维码，再使用支付宝扫码确认。</div>
                  )}
                </div>
              </>
            ) : (
              <>
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
              </>
            )}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setAddOpen(false);
                  setAddForm(EMPTY_ADD_FORM);
                  setAlipayQR(null);
                }}
                disabled={creatingAccount}
              >
                取消
              </Button>
              <Button
                type="submit"
                disabled={creatingAccount || (addForm.channel !== Channel.ALIPAY && accountQuota?.reached) || (addForm.channel === Channel.ALIPAY && Boolean(alipayQR) && alipayQR?.status !== AlipayLoginStatus.EXPIRED && alipayQR?.status !== AlipayLoginStatus.FAILED)}
              >
                {creatingAccount ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
                {creatingAccount ? "处理中" : addForm.channel === Channel.ALIPAY ? "获取二维码" : "新增"}
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
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    contentRef.current?.scrollTo({ top: 0 });
    window.scrollTo({ top: 0 });
  }, [account.id]);

  return (
    <div className="flex min-h-0 w-full min-w-0 max-w-full flex-col gap-3 sm:gap-4 xl:h-full xl:overflow-hidden">
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
      <div
        ref={contentRef}
        className={cn(
          "min-h-0",
          activeTab === "logs"
            ? "flex flex-1 xl:min-h-0 xl:overflow-hidden"
            : "dark-scrollbar xl:flex-1 xl:overflow-y-auto xl:pr-0.5",
        )}
      >
        {activeTab === "monitor" && <MonitorTab snapshot={snapshot} status={status} />}
        {activeTab === "logs" && <EventPanel events={events} />}
        {activeTab === "settings" && (
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
        )}
        {activeTab === "race" && <RaceTab snapshot={snapshot} policy={policy} />}
        {activeTab === "land" && <LandTab snapshot={snapshot} policy={policy} />}
        {activeTab === "warehouse" && <WarehouseTab snapshot={snapshot} />}
        {activeTab === "business" && <BusinessTab snapshot={snapshot} />}
      </div>
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
    <div className="dark-scrollbar sticky top-[3.25rem] z-20 flex shrink-0 gap-1 overflow-x-auto rounded-md border border-white/58 bg-white/62 p-1 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/72 sm:top-14 xl:static">
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
      <FmlRaceMonitorPanel
        race={snapshot?.fmlRace}
        showTakenTask={policy?.union?.race?.enabled ?? true}
        showPersonalScoreRank={policy?.union?.race?.showPersonalScoreRank ?? false}
      />
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
      <FmlLandMonitorPanel
        lands={snapshot?.fmlLands ?? []}
        plantableFlowers={snapshot?.plantableFlowers ?? []}
        observed={snapshot?.fmlLandsObserved ?? false}
        automationEnabled={policy?.automationEnabled ?? false}
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

function BusinessTab({ snapshot }: { snapshot: GetSnapshotResponse | null }) {
  return (
    <div className="space-y-3 sm:space-y-4">
      <BusinessStatisticsPanel statistics={snapshot?.businessStatistics} />
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
