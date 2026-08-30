"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { useRouter } from "next/navigation";
import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import {
  AccountRedeemAttemptFilter,
  WorkspaceLogPageKind,
  type AccountRedeemAttemptPage,
  type WorkspaceLogPage,
} from "@/gen/mygardenworld/v1/workspace_pb";
import { AccountHealth } from "@/lib/api/workspace-models";
import type { AccountStatus, Event, FeatureCapability } from "@/lib/api/workspace-models";
import AppShell from "@/components/app-shell";
import { formatAPIError, transport } from "@/lib/api/client";
import { WorkspaceClient, type WorkspaceConnectionState } from "@/lib/api/workspace-client";
import { useAuth } from "@/lib/auth/context";
import { cn } from "@/lib/utils";
import { accountNickname, accountConnected, isTransientConnectionMessage } from "@/components/dashboard/dashboard-utils";
import {
  applyWorkspacePatch,
  EMPTY_ACCOUNT_VIEWS,
  EVENT_LIMIT,
  mergeEvents,
  upsertAccount,
  withAccountStatus,
  workspaceStateToViews,
  type AccountViews,
} from "@/features/workspace/model";
import {
  applyRedeemAttemptPage,
  emptyRedeemAttemptFeed,
  loadingRedeemAttemptFeed,
  REDEEM_ATTEMPT_PAGE_SIZE,
  type RedeemAttemptLoadingMode,
} from "@/features/workspace/basic/redeem-attempts-model";
import { AccountDetailView, SelectAccountPlaceholder, type DashboardTabId } from "@/features/account-workspace/account-detail";
import AccountListPanel, { type AccountQuota } from "@/features/account-workspace/account-list-panel";
import AddAccountDialog, { EMPTY_ADD_FORM, type AddAccountForm, type AlipayQRState } from "@/features/account-workspace/add-account-dialog";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);

const accountKey = (id: bigint) => id.toString();

type LogFeed = {
  accountId: string;
  events: Event[];
  nextBeforeId: bigint;
  hasMore: boolean;
  loading: boolean;
};

const EMPTY_LOG_FEED: LogFeed = {
  accountId: "",
  events: [],
  nextBeforeId: BigInt(0),
  hasMore: false,
  loading: false,
};

export default function HomePage() {
  const [serverVersion, setServerVersion] = useState("");

  return (
    <AppShell version={serverVersion}>
      <DashboardContent onServerVersion={setServerVersion} />
    </AppShell>
  );
}

function DashboardContent({ onServerVersion }: { onServerVersion: (version: string) => void }) {
  const { user } = useAuth();
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [statuses, setStatuses] = useState<Map<string, AccountStatus>>(new Map());
  const [featureCapabilities, setFeatureCapabilities] = useState<FeatureCapability[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [views, setViews] = useState<AccountViews>(EMPTY_ACCOUNT_VIEWS);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [logFeed, setLogFeed] = useState<LogFeed>(EMPTY_LOG_FEED);
  const [redeemFeed, setRedeemFeed] = useState(() => emptyRedeemAttemptFeed());
  const [workspaceConnection, setWorkspaceConnection] = useState<WorkspaceConnectionState>("connecting");
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
  const [dashboardTab, setDashboardTab] = useState<DashboardTabId>("basic");
  const didAutoSelectAccount = useRef(false);
  const workspaceClientRef = useRef<WorkspaceClient | null>(null);
  const selectedAccountIdRef = useRef("");
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());
  const accountsLoadedRef = useRef(false);
  const policyOwnerAccountIdRef = useRef("");
  const logFeedsRef = useRef<Map<string, LogFeed>>(new Map());
  const redeemFeedRef = useRef(emptyRedeemAttemptFeed());

  const selectedAccount = useMemo(
    () => accounts.find((account) => accountKey(account.id) === selectedAccountId) ?? null,
    [accounts, selectedAccountId],
  );
  const selectedStatus = selectedAccountId ? statuses.get(selectedAccountId) : undefined;
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

  useEffect(() => {
    selectedAccountIdRef.current = selectedAccountId;
  }, [selectedAccountId]);

  const refreshAccounts = useCallback(async () => {
    const accountRes = await accountClient.listAccounts({});
    setAccounts(accountRes.accounts);
    accountsLoadedRef.current = true;
  }, []);

  const applyStatuses = useCallback((incoming: AccountStatus[]) => {
    const nextStatuses = new Map<string, AccountStatus>();
    for (const status of incoming) {
      nextStatuses.set(accountKey(status.accountId), status);
    }
    setStatuses(nextStatuses);
    setError((current) => (isTransientConnectionMessage(current) ? "" : current));
  }, []);

  const applyLogPage = useCallback((page?: WorkspaceLogPage) => {
    if (!page) return;
    const accountId = accountKey(page.accountId);
    const previous = logFeedsRef.current.get(accountId) ?? { ...EMPTY_LOG_FEED, accountId };
    const replace = page.gapDetected || page.kind === WorkspaceLogPageKind.RECENT;
    const events = mergeEvents(replace ? [] : previous.events, page.events);
    const paged = page.kind === WorkspaceLogPageKind.RECENT || page.kind === WorkspaceLogPageKind.BEFORE;
    const next: LogFeed = {
      accountId,
      events,
      nextBeforeId: paged && page.nextBeforeId > BigInt(0) ? page.nextBeforeId : previous.nextBeforeId,
      hasMore: paged ? page.hasMoreBefore && events.length < EVENT_LIMIT : previous.hasMore,
      loading: false,
    };
    logFeedsRef.current.set(accountId, next);
    if (accountId === selectedAccountIdRef.current) setLogFeed(next);
    if (page.gapDetected && accountId === selectedAccountIdRef.current) {
      setError("部分旧日志已超过服务端保留期，已重新加载当前可用窗口");
    }
  }, []);

  const requestRedeemAttemptPage = useCallback((
    accountId: string,
    filter: AccountRedeemAttemptFilter,
    beforeId: bigint,
    mode: Exclude<RedeemAttemptLoadingMode, "">,
  ) => {
    const sent = workspaceClientRef.current?.loadRedeemAttempts(
      accountId,
      beforeId,
      REDEEM_ATTEMPT_PAGE_SIZE,
      filter,
    ) ?? false;
    if (!sent) return false;
    setRedeemFeed((current) => {
      const next = loadingRedeemAttemptFeed(current, accountId, filter, mode);
      redeemFeedRef.current = next;
      return next;
    });
    return true;
  }, []);

  const applyRedeemPage = useCallback((page: AccountRedeemAttemptPage) => {
    setRedeemFeed((current) => {
      const next = applyRedeemAttemptPage(current, page);
      redeemFeedRef.current = next;
      return next;
    });
  }, []);

  const refreshAccountCollection = useCallback(async () => {
    await refreshAccounts();
    workspaceClientRef.current?.resync();
  }, [refreshAccounts]);

  const refreshDashboardStatus = useCallback(async () => {
    if (!accountsLoadedRef.current) await refreshAccounts();
    workspaceClientRef.current?.resync();
  }, [refreshAccounts]);

  const initializeWorkspace = useCallback(async () => {
    setError("");
    try {
      await refreshAccounts();
    } catch (err) {
      setError(formatAPIError(err, "刷新失败"));
    } finally {
      setLoading(false);
    }
  }, [refreshAccounts]);

  useEffect(() => {
    void initializeWorkspace();
  }, [initializeWorkspace]);

  useEffect(() => {
    const client = new WorkspaceClient({
      onConnectionState: setWorkspaceConnection,
      onReady: (ready) => {
        applyStatuses(ready.accounts);
        setFeatureCapabilities(ready.featureCapabilities);
        onServerVersion(ready.serverVersion || "dev");
      },
      onStatuses: (batch) => applyStatuses(batch.accounts),
      onSnapshot: (snapshot) => {
        const state = snapshot.state;
        if (!state || accountKey(state.accountId) !== selectedAccountIdRef.current) return;
        setViews(workspaceStateToViews(state));
        setPolicy(state.policy ?? create(PolicySchema));
        policyOwnerAccountIdRef.current = accountKey(state.accountId);
        setPolicyLoading(false);
        setPolicyMessage("");
        if (state.accountStatus) {
          setStatuses((current) => withAccountStatus(current, state.accountStatus!));
        }
        applyLogPage(snapshot.logs);
        const accountId = accountKey(state.accountId);
        const currentRedeemFeed = redeemFeedRef.current;
        const filter = currentRedeemFeed.accountId === accountId
          ? currentRedeemFeed.filter
          : AccountRedeemAttemptFilter.ALL;
        requestRedeemAttemptPage(accountId, filter, BigInt(0), "replace");
        setViewsLoading(false);
      },
      onPatch: (patch) => {
        if (accountKey(patch.accountId) !== selectedAccountIdRef.current) return;
        setViews((current) => applyWorkspacePatch(current, patch));
        if (patch.policy) {
          setPolicy(patch.policy);
          policyOwnerAccountIdRef.current = accountKey(patch.accountId);
        }
        if (patch.accountStatus) {
          setStatuses((current) => withAccountStatus(current, patch.accountStatus!));
        }
      },
      onLogs: applyLogPage,
      onRedeemAttempts: applyRedeemPage,
      onAlipayLogin: (progress) => {
        setAlipayQR((current) => current && current.loginId === progress.loginId
          ? { ...current, status: progress.status, error: progress.loginError }
          : current);
        if (progress.status === AlipayLoginStatus.COMPLETE && progress.account) {
          setAccounts((current) => upsertAccount(current, progress.account!));
          setSelectedAccountId(accountKey(progress.account.id));
          setAddOpen(false);
          setAddForm(EMPTY_ADD_FORM);
          setAlipayQR(null);
          void refreshAccounts();
        }
      },
      onError: (workspaceError) => {
        setLogFeed((current) => current.loading ? { ...current, loading: false } : current);
        setRedeemFeed((current) => {
          if (!current.loadingMode) return current;
          const next = { ...current, loadingMode: "" as const };
          redeemFeedRef.current = next;
          return next;
        });
        // Access-token rotation is an expected short reconnect: the socket
        // closes with 4401, refreshes through the HttpOnly cookie, then opens
        // again with the new access token. Connection state already presents
        // that transition, so do not leave a persistent red global error.
        if (workspaceError.code === "authentication_expired" && workspaceError.retryable) return;
        if (workspaceError.message) setError(workspaceError.message);
      },
    });
    workspaceClientRef.current = client;
    client.start(selectedAccountIdRef.current);
    return () => {
      workspaceClientRef.current = null;
      client.stop();
    };
  }, [applyLogPage, applyRedeemPage, applyStatuses, onServerVersion, refreshAccounts, requestRedeemAttemptPage]);

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
    setDashboardTab("basic");
  }, [selectedAccountId]);

  useEffect(() => {
    policyOwnerAccountIdRef.current = "";
    setViews(EMPTY_ACCOUNT_VIEWS);
    setPolicy(null);
    setPolicyMessage("");
    setLogFeed(selectedAccountId
      ? (logFeedsRef.current.get(selectedAccountId) ?? { ...EMPTY_LOG_FEED, accountId: selectedAccountId })
      : EMPTY_LOG_FEED);
    const nextRedeemFeed = emptyRedeemAttemptFeed(selectedAccountId);
    redeemFeedRef.current = nextRedeemFeed;
    setRedeemFeed(nextRedeemFeed);
    if (!selectedAccountId) {
      setPolicyLoading(false);
      setViewsLoading(false);
      return;
    }
    setPolicyLoading(true);
    setViewsLoading(true);
    workspaceClientRef.current?.selectAccount(selectedAccountId);
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
      workspaceClientRef.current?.resync();
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
      // Optimistic flip so the list button/badge update before the pushed patch.
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
      if (accountId === selectedAccountId) workspaceClientRef.current?.resync();
    } catch (err) {
      setError(formatAPIError(err, online ? "暂停失败" : "启动失败"));
      workspaceClientRef.current?.resync();
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
      if (accountId === selectedAccountId) workspaceClientRef.current?.resync();
    } catch (err) {
      setError(formatAPIError(err, "停止失败"));
      workspaceClientRef.current?.resync();
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
        } catch (err) {
          failures.push(
            `${accountNickname(account)}: ${formatAPIError(err, wantOnline ? "启动失败" : "暂停失败")}`,
          );
        }
      }

      workspaceClientRef.current?.resync();
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
      workspaceClientRef.current?.watchAlipayLogin(response.loginId);
    } catch (err) {
      setError(formatAPIError(err, "获取 Alipay 二维码失败"));
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
      policyOwnerAccountIdRef.current = "";
      const nextAccounts = accounts.filter((account) => account.id !== selectedAccount.id);
      setSelectedAccountId(nextAccounts[0] ? accountKey(nextAccounts[0].id) : "");
      setViews(EMPTY_ACCOUNT_VIEWS);
      setPolicy(null);
      setStatuses((current) => {
        const next = new Map(current);
        next.delete(accountKey(selectedAccount.id));
        return next;
      });
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
      workspaceClientRef.current?.resync();
    } catch (err) {
      if (policyOwnerAccountIdRef.current === accountId) {
        setPolicyMessage(formatAPIError(err, "保存失败"));
      }
    } finally {
      setSavingPolicy(false);
    }
  }

  function loadMoreLogs() {
    if (!selectedAccountId || logFeed.loading || !logFeed.hasMore) return;
    const sent = workspaceClientRef.current?.loadLogs(selectedAccountId, logFeed.nextBeforeId, 200) ?? false;
    if (sent) {
      setLogFeed((current) => {
        const next = { ...current, loading: true };
        logFeedsRef.current.set(current.accountId, next);
        return next;
      });
    } else {
      setError("状态通道尚未连接，暂时无法加载更早日志");
    }
  }

  function changeRedeemAttemptFilter(filter: AccountRedeemAttemptFilter) {
    if (!selectedAccountId) return;
    const current = redeemFeedRef.current;
    if (current.filter === filter && current.loadingMode) return;
    if (!requestRedeemAttemptPage(selectedAccountId, filter, BigInt(0), "replace")) {
      setError("状态通道尚未连接，暂时无法加载兑换记录");
    }
  }

  function loadMoreRedeemAttempts() {
    const current = redeemFeedRef.current;
    if (!selectedAccountId || current.loadingMode || !current.hasMore || current.nextBeforeId <= BigInt(0)) return;
    if (!requestRedeemAttemptPage(selectedAccountId, current.filter, current.nextBeforeId, "append")) {
      setError("状态通道尚未连接，暂时无法加载更早的兑换记录");
    }
  }

  return (
    <div className="relative z-10 min-h-0 xl:h-full">
      {error && (
        <div className="mb-4 rounded-md border border-destructive/25 bg-white/72 px-3 py-2 text-sm text-destructive shadow-sm backdrop-blur-xl dark:bg-destructive/12">
          {error}
        </div>
      )}
      {!error && workspaceConnection !== "open" && (
        <div className="mb-4 rounded-md border border-amber-400/30 bg-amber-50/75 px-3 py-2 text-sm text-amber-800 shadow-sm backdrop-blur-xl dark:bg-amber-400/10 dark:text-amber-200">
          状态通道正在{workspaceConnection === "connecting" ? "连接" : "重连"}，写操作仍可继续使用。
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
            onRedeem={() => router.push("/redeem")}
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
                redeemFeed={redeemFeed}
                events={logFeed.events}
                logsHasMore={logFeed.hasMore}
                logsLoading={logFeed.loading}
                policy={policy}
                policyLoading={policyLoading}
                savingPolicy={savingPolicy}
                policyMessage={policyMessage}
                onBack={() => setSelectedAccountId("")}
                onTabChange={setDashboardTab}
                onRefresh={() => {
                  setViewsLoading(true);
                  workspaceClientRef.current?.resync();
                }}
                onAction={runAccountAction}
                onDelete={() => void deleteSelectedAccount()}
                onPolicyChange={setPolicy}
                onPolicySave={() => void savePolicy()}
                onLoadMoreLogs={loadMoreLogs}
                onRedeemFilterChange={changeRedeemAttemptFilter}
                onLoadMoreRedeemAttempts={loadMoreRedeemAttempts}
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
      />

    </div>
  );
}
