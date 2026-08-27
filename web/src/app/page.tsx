"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { AccountService, AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import { QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import { AccountHealth } from "@/lib/api/query-models";
import type { AccountStatus, Event, FeatureCapability } from "@/lib/api/query-models";
import AppShell from "@/components/app-shell";
import { formatAPIError, transport } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { cn } from "@/lib/utils";
import { accountNickname, accountConnected, isRunnerNotStartedError, isTransientConnectionMessage, waitForAbortableDelay } from "@/components/dashboard/dashboard-utils";
import RedeemCodeDialog from "@/components/dashboard/redeem-code-dialog";
import { EMPTY_ACCOUNT_VIEWS, type AccountViews } from "@/features/account-workspace/model";
import { AccountDetailView, SelectAccountPlaceholder, type DashboardTabId } from "@/features/account-workspace/account-detail";
import AccountListPanel, { type AccountQuota } from "@/features/account-workspace/account-list-panel";
import AddAccountDialog, { EMPTY_ADD_FORM, type AddAccountForm, type AlipayQRState } from "@/features/account-workspace/add-account-dialog";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);
const queryClient = createClient(QueryService, transport);

const EVENT_LIMIT = 500;
const EVENT_RECONNECT_INITIAL_MS = 1000;
const EVENT_RECONNECT_MAX_MS = 15000;
const STATUS_POLL_MS = 5000;
const accountKey = (id: bigint) => id.toString();

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
  const [views, setViews] = useState<AccountViews>(EMPTY_ACCOUNT_VIEWS);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [viewsLoading, setViewsLoading] = useState(false);
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
  const [dashboardTab, setDashboardTab] = useState<DashboardTabId>("overview");
  const didAutoSelectAccount = useRef(false);
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());
  const accountsLoadedRef = useRef(false);
  const capabilitiesLoadedRef = useRef(false);
  const viewFetchGenRef = useRef(0);
  const viewFetchAbortRef = useRef<AbortController | null>(null);
  // Bumped on each getPolicy so a slow response from account A cannot overwrite
  // the panel after the user has switched to account B (and then save A's policy
  // onto B).
  const policyFetchGenRef = useRef(0);
  const policyOwnerAccountIdRef = useRef("");

  const selectedAccount = useMemo(
    () => accounts.find((account) => accountKey(account.id) === selectedAccountId) ?? null,
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

  useEffect(() => () => {
    viewFetchAbortRef.current?.abort();
  }, []);

  const refreshAccounts = useCallback(async () => {
    const accountRes = await accountClient.listAccounts({});
    setAccounts(accountRes.accounts);
    accountsLoadedRef.current = true;
  }, []);

  const refreshStatuses = useCallback(async () => {
    const needsCapabilities = !capabilitiesLoadedRef.current;
    const [statusRes, capabilitiesRes] = await Promise.all([
      queryClient.getStatus({}),
      needsCapabilities ? queryClient.getFeatureCapabilities({}) : Promise.resolve(null),
    ]);
    const nextStatuses = new Map<string, AccountStatus>();
    for (const status of statusRes.accounts) {
      nextStatuses.set(accountKey(status.accountId), status);
    }
    setStatuses(nextStatuses);
    if (capabilitiesRes) {
      setFeatureCapabilities(capabilitiesRes.featureCapabilities);
      capabilitiesLoadedRef.current = capabilitiesRes.featureCapabilities.length > 0;
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

  const canReadViews = useCallback((accountId: string) => {
    const account = accountsRef.current.find((item) => accountKey(item.id) === accountId);
    if (!account) return false;
    return accountConnected(account, statusesRef.current.get(accountId));
  }, []);

  const refreshViews = useCallback(async (accountId: string, showLoading = false, options?: { force?: boolean; }) => {
    const fetchGen = ++viewFetchGenRef.current;
    viewFetchAbortRef.current?.abort();
    viewFetchAbortRef.current = null;
    if (!accountId) {
      setViews(EMPTY_ACCOUNT_VIEWS);
      return;
    }
    if (!options?.force && !canReadViews(accountId)) {
      setViews(EMPTY_ACCOUNT_VIEWS);
      setViewsLoading(false);
      setError((current) => (isRunnerNotStartedError(current) ? "" : current));
      return;
    }
    if (showLoading) {
      setViewsLoading(true);
    }
    const controller = new AbortController();
    viewFetchAbortRef.current = controller;
    try {
      const [overview, garden, orders, union, activities, assets] = await Promise.all([
        queryClient.getOverview({ accountId: BigInt(accountId) }, { signal: controller.signal }),
        queryClient.getGarden({ accountId: BigInt(accountId) }, { signal: controller.signal }),
        queryClient.getOrders({ accountId: BigInt(accountId) }, { signal: controller.signal }),
        queryClient.getUnion({ accountId: BigInt(accountId) }, { signal: controller.signal }),
        queryClient.getActivities({ accountId: BigInt(accountId) }, { signal: controller.signal }),
        queryClient.getAssets({ accountId: BigInt(accountId) }, { signal: controller.signal }),
      ]);
      if (fetchGen !== viewFetchGenRef.current) return;
      setViews({
        overview: overview.overview ?? null,
        garden: garden.garden ?? null,
        orders: orders.orders ?? null,
        union: union.union ?? null,
        activities: activities.activities ?? null,
        assets: assets.assets ?? null,
      });
    } catch (err) {
      if (controller.signal.aborted) return;
      if (fetchGen !== viewFetchGenRef.current) return;
      setViews(EMPTY_ACCOUNT_VIEWS);
      if (!isRunnerNotStartedError(err)) {
        setError(formatAPIError(err, "读取账号视图失败"));
      } else {
        setError((current) => (isRunnerNotStartedError(current) ? "" : current));
      }
    } finally {
      if (viewFetchAbortRef.current === controller) {
        viewFetchAbortRef.current = null;
      }
      if (fetchGen === viewFetchGenRef.current) {
        setViewsLoading(false);
      }
    }
  }, [canReadViews]);

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
      const res = await policyClient.getPolicy({ accountId: BigInt(accountId) });
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
          setSelectedAccountId(accountKey(response.account.id));
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
    if (selectedAccountId && !accounts.some((account) => accountKey(account.id) === selectedAccountId)) {
      setSelectedAccountId(accountKey(accounts[0].id));
      didAutoSelectAccount.current = true;
      return;
    }
    if (!selectedAccountId && !didAutoSelectAccount.current) {
      setSelectedAccountId(accountKey(accounts[0].id));
      didAutoSelectAccount.current = true;
    }
  }, [accounts, selectedAccountId]);

  useEffect(() => {
    setDashboardTab("overview");
  }, [selectedAccountId]);

  useEffect(() => {
    if (!selectedAccountId) {
      viewFetchAbortRef.current?.abort();
      viewFetchAbortRef.current = null;
      policyFetchGenRef.current += 1;
      policyOwnerAccountIdRef.current = "";
      viewFetchGenRef.current += 1;
      setViews(EMPTY_ACCOUNT_VIEWS);
      setPolicy(null);
      setPolicyLoading(false);
      setEvents([]);
      return;
    }
    if (selectedConnected) {
      void refreshViews(selectedAccountId, true);
    } else {
      viewFetchAbortRef.current?.abort();
      viewFetchAbortRef.current = null;
      viewFetchGenRef.current += 1;
      setViews(EMPTY_ACCOUNT_VIEWS);
      setViewsLoading(false);
      setError((current) => (isRunnerNotStartedError(current) ? "" : current));
    }
  }, [refreshViews, selectedAccountId, selectedConnected]);

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
        void refreshViews(selectedAccountId).catch(() => undefined);
      }
    }, STATUS_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refreshViews, refreshStatuses, selectedAccountId]);

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
          for await (const response of queryClient.streamEvents(
            { accountId: BigInt(selectedAccountId), replayLimit: EVENT_LIMIT, afterEventId: lastEventId },
            { signal: controller.signal },
          )) {
            if (!active || controller.signal.aborted) return;
            const event = response.event;
            if (!event) continue;
            if (event.id > BigInt(0)) {
              if (event.id <= lastEventId) continue;
              lastEventId = event.id;
            }
            receivedEvent = true;
            retryDelayMs = EVENT_RECONNECT_INITIAL_MS;
            setError((current) => (isTransientConnectionMessage(current) ? "" : current));
            // Stream replay and batch operations can emit hundreds of events.
            // The bounded poll below refreshes views without multiplying each
            // event into six concurrent domain requests.
            setEvents((prev) => [event, ...prev].slice(0, EVENT_LIMIT));
          }
        } catch (err) {
          if (!active || controller.signal.aborted) return;
          const streamError = formatAPIError(err, "事件流中断");
          if (!isTransientConnectionMessage(streamError)) {
            setError((current) => (current && !isTransientConnectionMessage(current) ? current : streamError));
          }
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
  }, [selectedAccountId]);

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
        ? await accountClient.connectAccount({ id: selectedAccount.id })
        : await accountClient.disconnectAccount({ id: selectedAccount.id });
      updateCachedAccount(response.account);
      await refreshStatuses();
      await refreshViews(accountKey(selectedAccount.id), action === "login", { force: action === "login" });
      await refreshPolicy(accountKey(selectedAccount.id));
    } catch (err) {
      setError(formatAPIError(err, "操作失败"));
    } finally {
      setBusyAction("");
    }
  }

  async function runAutomationToggle(accountId: string) {
    if (busyBulkAutomation) return;
    const account = accountsRef.current.find((item) => accountKey(item.id) === accountId);
    const status = statusesRef.current.get(accountId);
    const online = account ? accountConnected(account, status) : Boolean(status?.connected);
    setBusyAutomationAccountId(accountId);
    setError("");
    try {
      const response = online
        ? await accountClient.disconnectAccount({ id: BigInt(accountId) })
        : await accountClient.connectAccount({ id: BigInt(accountId) });
      // Optimistic flip so the list button/badge update before the next poll.
      setStatuses((prev) => {
        const next = new Map(prev);
        const current = next.get(accountId);
        if (current) {
          next.set(accountId, {
            ...current,
            connected: !online,
            automationEnabled: !online,
            health: online ? AccountHealth.OFFLINE : AccountHealth.ONLINE,
          });
        }
        return next;
      });
      setAccounts((prev) =>
        prev.map((item) => (
          accountKey(item.id) === accountId ? (response.account ?? { ...item, connected: !online }) : item
        )),
      );
      await refreshStatuses();
      if (accountId === selectedAccountId) {
        await refreshPolicy(accountId);
        await refreshViews(accountId, !online, { force: !online });
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
      const response = await accountClient.disconnectAccount({ id: BigInt(accountId) });
      setStatuses((prev) => {
        const next = new Map(prev);
        const current = next.get(accountId);
        if (current) {
          next.set(accountId, {
            ...current,
            connected: false,
            automationEnabled: false,
            health: AccountHealth.OFFLINE,
            lastError: "",
          });
        }
        return next;
      });
      setAccounts((prev) =>
        prev.map((item) => (
          accountKey(item.id) === accountId ? (response.account ?? { ...item, connected: false }) : item
        )),
      );
      await refreshStatuses();
      if (accountId === selectedAccountId) {
        await refreshPolicy(accountId);
        await refreshViews(accountId, false);
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
      const online = accountConnected(account, statusesRef.current.get(accountKey(account.id)));
      return online !== wantOnline;
    });
    if (targets.length === 0) return;

    setBusyBulkAutomation(action);
    setError("");
    const failures: string[] = [];
    let selectedTouched = false;

    try {
      for (const account of targets) {
        setBusyAutomationAccountId(accountKey(account.id));
        try {
          const response = wantOnline
            ? await accountClient.connectAccount({ id: account.id })
            : await accountClient.disconnectAccount({ id: account.id });
          setStatuses((prev) => {
            const next = new Map(prev);
            const key = accountKey(account.id);
            const current = next.get(key);
            if (current) {
              next.set(key, {
                ...current,
                connected: wantOnline,
                automationEnabled: wantOnline,
                health: wantOnline ? AccountHealth.ONLINE : AccountHealth.OFFLINE,
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
          if (accountKey(account.id) === selectedAccountId) selectedTouched = true;
        } catch (err) {
          failures.push(
            `${accountNickname(account)}: ${formatAPIError(err, wantOnline ? "启动失败" : "暂停失败")}`,
          );
        }
      }

      await refreshStatuses();
      if (selectedTouched && selectedAccountId) {
        await refreshPolicy(selectedAccountId);
        await refreshViews(selectedAccountId, wantOnline, { force: wantOnline });
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
        username: addForm.username.trim(),
        password: addForm.password,
        channel: addForm.channel,
      });
      setAddOpen(false);
      setAddForm(EMPTY_ADD_FORM);
      await refreshAccountCollection();
      if (res.account?.id) {
        setSelectedAccountId(accountKey(res.account.id));
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
      setSelectedAccountId(nextAccounts[0] ? accountKey(nextAccounts[0].id) : "");
      viewFetchGenRef.current += 1;
      setViews(EMPTY_ACCOUNT_VIEWS);
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
    const accountId = accountKey(selectedAccount.id);
    if (policyOwnerAccountIdRef.current !== accountId) {
      setPolicyMessage("策略尚未与当前账号对齐，请等待加载完成后再保存");
      return;
    }
    setSavingPolicy(true);
    setPolicyMessage("");
    try {
      const res = await policyClient.setPolicy({ accountId: selectedAccount.id, policy });
      if (policyOwnerAccountIdRef.current !== accountId) return;
      setPolicy(res.policy ?? policy);
      setPolicyMessage("");
      await refreshStatuses();
      if (policyOwnerAccountIdRef.current === accountId) {
        await refreshViews(accountId);
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
            onRedeem={() => setRedeemOpen(true)}
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
                views={views}
                viewsLoading={viewsLoading}
                busyAction={busyAction}
                activeTab={dashboardTab}
                events={events}
                policy={policy}
                policyLoading={policyLoading}
                savingPolicy={savingPolicy}
                policyMessage={policyMessage}
                onBack={() => setSelectedAccountId("")}
                onTabChange={setDashboardTab}
                onRefresh={() => void refreshViews(accountKey(selectedAccount.id), true)}
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

      <AddAccountDialog
        open={addOpen}
        form={addForm}
        qr={alipayQR}
        quota={accountQuota}
        creating={creatingAccount}
        onOpenChange={(open) => {
          setAddOpen(open);
          if (!open) {
            setAddForm(EMPTY_ADD_FORM);
            setAlipayQR(null);
          }
        }}
        onFormChange={setAddForm}
        onClearQR={() => setAlipayQR(null)}
        onSubmit={createAccount}
        onStartAlipay={() => void startAlipayLogin()}
      />

      {redeemOpen && (
        <RedeemCodeDialog
          accounts={accounts}
          preferredChannel={selectedAccount?.channel}
          onOpenChange={setRedeemOpen}
          onRedeem={async (code, accountIds) => {
            setError("");
            const response = await accountClient.redeemCode({ code, accountIds });
            await refreshStatuses().catch(() => undefined);
            return {
              results: response.results,
              successCount: response.successCount,
              failureCount: response.failureCount,
            };
          }}
        />
      )}
    </div>
  );
}
