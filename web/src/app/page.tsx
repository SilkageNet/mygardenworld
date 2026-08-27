"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import type { WorkspaceHistoryItem, WorkspaceHistorySummary } from "@/gen/mygardenworld/v1/workspace_pb";
import { AccountHealth } from "@/lib/api/workspace-models";
import type { AccountStatus, Event, FeatureCapability } from "@/lib/api/workspace-models";
import AppShell from "@/components/app-shell";
import { formatAPIError, transport } from "@/lib/api/client";
import { WorkspaceClient, type WorkspaceConnectionState } from "@/lib/api/workspace-client";
import { useAuth } from "@/lib/auth/context";
import { cn } from "@/lib/utils";
import { accountNickname, accountConnected, isTransientConnectionMessage } from "@/components/dashboard/dashboard-utils";
import RedeemCodeDialog from "@/components/dashboard/redeem-code-dialog";
import {
  applyWorkspacePatch,
  EMPTY_ACCOUNT_VIEWS,
  mergeEvents,
  mergeHistoryItems,
  upsertAccount,
  withAccountStatus,
  workspaceStateToViews,
  type AccountViews,
} from "@/features/workspace/model";
import { AccountDetailView, SelectAccountPlaceholder, type DashboardTabId } from "@/features/account-workspace/account-detail";
import AccountListPanel, { type AccountQuota } from "@/features/account-workspace/account-list-panel";
import AddAccountDialog, { EMPTY_ADD_FORM, type AddAccountForm, type AlipayQRState } from "@/features/account-workspace/add-account-dialog";

const accountClient = createClient(AccountService, transport);
const policyClient = createClient(PolicyService, transport);

const accountKey = (id: bigint) => id.toString();

type HistoryFeed = {
  accountId: string;
  items: WorkspaceHistoryItem[];
  nextBeforeId: bigint;
  hasMore: boolean;
  loading: boolean;
  paged: boolean;
};

const EMPTY_HISTORY_FEED: HistoryFeed = {
  accountId: "",
  items: [],
  nextBeforeId: BigInt(0),
  hasMore: false,
  loading: false,
  paged: false,
};

function historyFeedFromSummary(accountId: string, summary?: WorkspaceHistorySummary): HistoryFeed {
  return {
    accountId,
    items: summary?.recentOperations ?? [],
    nextBeforeId: summary?.nextBeforeId ?? BigInt(0),
    hasMore: summary?.hasMore ?? false,
    loading: false,
    paged: false,
  };
}

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
  const [historyFeed, setHistoryFeed] = useState<HistoryFeed>(EMPTY_HISTORY_FEED);
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
  const [redeemOpen, setRedeemOpen] = useState(false);
  const [dashboardTab, setDashboardTab] = useState<DashboardTabId>("basic");
  const didAutoSelectAccount = useRef(false);
  const workspaceClientRef = useRef<WorkspaceClient | null>(null);
  const selectedAccountIdRef = useRef("");
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());
  const accountsLoadedRef = useRef(false);
  const policyOwnerAccountIdRef = useRef("");

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
      },
      onStatuses: (batch) => applyStatuses(batch.accounts),
      onSnapshot: (snapshot) => {
        const state = snapshot.state;
        if (!state || accountKey(state.accountId) !== selectedAccountIdRef.current) return;
        setViews(workspaceStateToViews(state));
        setPolicy(state.policy ?? create(PolicySchema));
        setHistoryFeed(historyFeedFromSummary(accountKey(state.accountId), state.history));
        policyOwnerAccountIdRef.current = accountKey(state.accountId);
        setPolicyLoading(false);
        setPolicyMessage("");
        if (state.accountStatus) {
          setStatuses((current) => withAccountStatus(current, state.accountStatus!));
        }
        setEvents((current) => mergeEvents(current, snapshot.logs));
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
        if (patch.history) {
          setHistoryFeed((current) => {
            if (current.accountId !== accountKey(patch.accountId) || !current.paged) {
              return historyFeedFromSummary(accountKey(patch.accountId), patch.history);
            }
            return {
              ...current,
              items: mergeHistoryItems(current.items, patch.history!.recentOperations),
            };
          });
        }
      },
      onLogs: (batch) => {
        if (accountKey(batch.accountId) !== selectedAccountIdRef.current) return;
        setEvents((current) => mergeEvents(current, batch.events));
      },
      onHistory: (page) => {
        if (accountKey(page.accountId) !== selectedAccountIdRef.current) return;
        setHistoryFeed((current) => ({
          accountId: accountKey(page.accountId),
          items: mergeHistoryItems(current.accountId === accountKey(page.accountId) ? current.items : [], page.items),
          nextBeforeId: page.nextBeforeId,
          hasMore: page.hasMore,
          loading: false,
          paged: true,
        }));
      },
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
        setHistoryFeed((current) => current.loading ? { ...current, loading: false } : current);
        if (workspaceError.message) setError(workspaceError.message);
      },
    });
    workspaceClientRef.current = client;
    client.start(selectedAccountIdRef.current);
    return () => {
      workspaceClientRef.current = null;
      client.stop();
    };
  }, [applyStatuses, refreshAccounts]);

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
    setEvents([]);
    setHistoryFeed(EMPTY_HISTORY_FEED);
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

  function loadMoreHistory() {
    if (!selectedAccountId || historyFeed.loading || !historyFeed.hasMore) return;
    const sent = workspaceClientRef.current?.loadHistory(selectedAccountId, historyFeed.nextBeforeId, 50) ?? false;
    if (sent) {
      setHistoryFeed((current) => ({ ...current, loading: true }));
    } else {
      setError("状态通道尚未连接，暂时无法加载更多历史");
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
                historyItems={historyFeed.items}
                historyHasMore={historyFeed.hasMore}
                historyLoading={historyFeed.loading}
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
                onLoadMoreHistory={loadMoreHistory}
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
            workspaceClientRef.current?.resync();
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
