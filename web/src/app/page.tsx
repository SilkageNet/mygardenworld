"use client";

import { useEffect, useLayoutEffect, useRef, useState, useCallback, useMemo, type FormEvent, type ReactNode, type RefObject } from "react";
import { createClient } from "@connectrpc/connect";
import { AccountService } from "@/gen/mygardenworld/v1/account_service_pb";
import { QueryService } from "@/gen/mygardenworld/v1/query_service_pb";
import { AutomationService } from "@/gen/mygardenworld/v1/automation_service_pb";
import { PolicyService } from "@/gen/mygardenworld/v1/policy_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { AccountStatus, Event, GetSnapshotResponse, LandView, PendingTaskView, RequirementView } from "@/gen/mygardenworld/v1/query_service_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { transport } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  ArrowDownToLine,
  Coins,
  ChevronDown,
  Droplets,
  Flower2,
  Gem,
  Check,
  LockKeyhole,
  LayoutList,
  ListChecks,
  LogIn,
  LogOut,
  MapIcon,
  Package,
  Play,
  Plus,
  RefreshCw,
  Shovel,
  SlidersHorizontal,
  Sprout,
  Square,
  TrendingUp,
  Trash2,
  Wifi,
  WifiOff,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { allFlowers, itemCategory, itemIconPath, itemInfo, itemName, type FlowerInfo, type ItemInfo } from "@/lib/game/catalog";
import AppShell from "@/components/app-shell";

const accountClient = createClient(AccountService, transport);
const queryClient = createClient(QueryService, transport);
const automationClient = createClient(AutomationService, transport);
const policyClient = createClient(PolicyService, transport);
const MAX_EVENT_ROWS = 300;
const SNAPSHOT_REFRESH_DELAY_MS = 350;
const FLOWER_OPTIONS = allFlowers();
const SNAPSHOT_REFRESH_EVENT_KINDS = new Set([
  "resource_changed",
  "inventory_changed",
  "land_changed",
  "land_unlock",
  "operation_ack",
  "task_recv",
  "task_daily",
  "road_grow",
  "random_event",
  "story_unlock",
  "order_finish",
  "order_customer",
  "flower_art",
  "cultivate_recv",
  "cultivate_new",
  "flower_upgrade",
  "waterwheel",
  "free_water",
]);
const SNAPSHOT_REFRESH_EVENT_CATEGORIES = new Set(["task", "order", "cultivation", "reward"]);

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
  const [selectedSnapshot, setSelectedSnapshot] = useState<GetSnapshotResponse | null>(null);
  const [events, setEvents] = useState<Event[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [policyAccountId, setPolicyAccountId] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [policyMessage, setPolicyMessage] = useState("");
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [busyAccountId, setBusyAccountId] = useState("");
  const [workspaceBusy, setWorkspaceBusy] = useState("");
  const [error, setError] = useState("");
  const logViewportRef = useRef<HTMLDivElement>(null);
  const autoScrollLogRef = useRef(true);
  const accountsRef = useRef<Account[]>([]);
  const statusesRef = useRef<Map<string, AccountStatus>>(new Map());
  const selectedAccountIdRef = useRef("");
  const didInitialFetchRef = useRef(false);
  const snapshotRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setError("");
      const [accRes, statusRes] = await Promise.all([
        accountClient.listAccounts({}),
        queryClient.getStatus({}),
      ]);
      setAccounts(accRes.accounts);
      const statusMap = new Map<string, AccountStatus>();
      for (const status of statusRes.accounts) {
        statusMap.set(status.accountId, status);
      }
      setStatuses(statusMap);
    } catch (err) {
      setError(err instanceof Error ? err.message : "刷新数据失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshStatus = useCallback(async () => {
    try {
      const statusRes = await queryClient.getStatus({});
      const statusMap = new Map<string, AccountStatus>();
      for (const status of statusRes.accounts) {
        statusMap.set(status.accountId, status);
      }
      setStatuses(statusMap);
    } catch {
      // The event stream is the primary live signal; keep the current view if a background refresh fails.
    }
  }, []);

  const refreshSelectedSnapshot = useCallback(async (options?: {
    accountId?: string;
    showLoading?: boolean;
    clearOnError?: boolean;
    skipConnectedCheck?: boolean;
  }) => {
    const accountId = options?.accountId ?? selectedAccountId;
    if (!accountId) {
      setSelectedSnapshot(null);
      return;
    }
    if (!options?.skipConnectedCheck) {
      const account = accountsRef.current.find((item) => item.id === accountId);
      const status = statusesRef.current.get(accountId);
      const connected = status?.connected ?? account?.connected ?? false;
      if (!connected) {
        setSelectedSnapshot(null);
        setSnapshotLoading(false);
        return;
      }
    }
    if (options?.showLoading) {
      setSnapshotLoading(true);
    }
    try {
      const snapshot = await queryClient.getSnapshot({ accountId });
      setSelectedSnapshot(snapshot);
    } catch {
      if (options?.clearOnError) {
        setSelectedSnapshot(null);
      }
    } finally {
      if (options?.showLoading) {
        setSnapshotLoading(false);
      }
    }
  }, [selectedAccountId]);

  const refreshPolicy = useCallback(async (accountId = selectedAccountId) => {
    if (!accountId) {
      setPolicy(null);
      setPolicyAccountId("");
      return;
    }
    setPolicy(null);
    setPolicyAccountId(accountId);
    try {
      const res = await policyClient.getPolicy({ accountId });
      setPolicy(res.policy ?? null);
      setPolicyAccountId(accountId);
    } catch {
      setPolicy(null);
      setPolicyAccountId(accountId);
    }
  }, [selectedAccountId]);

  const scheduleSnapshotRefresh = useCallback((accountId: string) => {
    if (!accountId || accountId !== selectedAccountIdRef.current) return;
    if (snapshotRefreshTimerRef.current) return;
    snapshotRefreshTimerRef.current = setTimeout(() => {
      snapshotRefreshTimerRef.current = null;
      if (accountId !== selectedAccountIdRef.current) return;
      refreshSelectedSnapshot({ accountId, skipConnectedCheck: true });
      refreshStatus();
    }, SNAPSHOT_REFRESH_DELAY_MS);
  }, [refreshSelectedSnapshot, refreshStatus]);

  const applyEventToLocalState = useCallback((event: Event) => {
    setStatuses((prev) => {
      const current = prev.get(event.accountId);
      if (!current) return prev;
      const next = new Map(prev);
      next.set(event.accountId, { ...current, lastEventAt: event.ts });
      return next;
    });

    if (event.kind === "session" || event.kind === "ws_disconnected" || event.kind === "session_expired") {
      setStatuses((prev) => {
        const current = prev.get(event.accountId);
        if (!current) return prev;
        const next = new Map(prev);
        next.set(event.accountId, {
          ...current,
          connected: event.kind === "session",
        });
        return next;
      });
    }

    const payload = parseEventPayload(event.payloadJson);
    if (event.kind === "policy_changed") {
      const enabled = booleanField(payload, "automation_enabled", undefined);
      if (enabled !== undefined) {
        setStatuses((prev) => {
          const current = prev.get(event.accountId);
          if (!current) return prev;
          const next = new Map(prev);
          next.set(event.accountId, { ...current, automationEnabled: enabled });
          return next;
        });
        if (event.accountId === selectedAccountIdRef.current) {
          setPolicy((prev) => prev ? { ...prev, automationEnabled: enabled } : prev);
        }
      }
    }

    if (event.accountId !== selectedAccountIdRef.current) return;
    if (!payload) return;

    if (event.kind === "resource_changed") {
      setSelectedSnapshot((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          gold: numberField(payload, "gold", prev.gold),
          waterDrops: numberField(payload, "water_drops", prev.waterDrops),
          waterDropsTotal: numberField(payload, "water_drops_total", prev.waterDropsTotal),
          waterDropsNextMs: bigintField(payload, "water_drops_next_ms", prev.waterDropsNextMs),
          level: numberField(payload, "level", prev.level),
          experience: numberField(payload, "experience", prev.experience),
          diamondsFree: numberField(payload, "diamonds_free", prev.diamondsFree),
          diamondsPaid: numberField(payload, "diamonds_paid", prev.diamondsPaid),
        };
      });
      return;
    }

    if (event.kind === "inventory_changed") {
      setSelectedSnapshot((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          inventory: inventoryField(payload, "inventory", prev.inventory),
        };
      });
      return;
    }

    if (event.kind === "land_changed") {
      const changes = arrayField(payload, "changes");
      if (changes.length > 0) {
        setSelectedSnapshot((prev) => {
          if (!prev) return prev;
          const lands = [...prev.lands];
          const byId = new Map<number, number>();
          lands.forEach((land, index) => byId.set(land.landId, index));
          let touched = false;
          for (const change of changes) {
            const landId = numberField(change, "landId", 0);
            const after = recordField(change, "after");
            const index = byId.get(landId);
            if (!landId || !after || index === undefined) continue;
            lands[index] = applyLandAfter(lands[index], after);
            touched = true;
          }
          return touched ? { ...prev, lands } : prev;
        });
        return;
      }
      const landId = numberField(payload, "landId", 0);
      const after = recordField(payload, "after");
      if (!landId || !after) return;
      setSelectedSnapshot((prev) => {
        if (!prev) return prev;
        const index = prev.lands.findIndex((land) => land.landId === landId);
        if (index < 0) return prev;
        const current = prev.lands[index];
        const lands = [...prev.lands];
        lands[index] = applyLandAfter(current, after);
        return { ...prev, lands };
      });
    }
  }, []);

  useEffect(() => {
    accountsRef.current = accounts;
  }, [accounts]);

  useEffect(() => {
    statusesRef.current = statuses;
  }, [statuses]);

  useEffect(() => {
    selectedAccountIdRef.current = selectedAccountId;
    setPolicy(null);
    setPolicyAccountId(selectedAccountId);
    setPolicyMessage("");
  }, [selectedAccountId]);

  useEffect(() => {
    if (didInitialFetchRef.current) return;
    didInitialFetchRef.current = true;
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    if (accounts.length === 0) {
      setSelectedAccountId("");
      return;
    }
    if (!accounts.some((account) => account.id === selectedAccountId)) {
      setSelectedAccountId(accounts[0].id);
    }
  }, [accounts, selectedAccountId]);

  useEffect(() => {
    refreshSelectedSnapshot({ showLoading: true, clearOnError: true });
  }, [selectedAccountId, refreshSelectedSnapshot]);

  useEffect(() => {
    refreshPolicy();
  }, [refreshPolicy]);

  useEffect(() => {
    const abort = new AbortController();
    let stopped = false;

    (async () => {
      let retryMs = 1000;
      while (!stopped) {
        try {
          setStreaming(true);
          const stream = queryClient.streamEvents({}, { signal: abort.signal });
          for await (const event of stream) {
            retryMs = 1000;
            setEvents((prev) => [...prev.slice(-(MAX_EVENT_ROWS * 3 - 1)), event]);
            applyEventToLocalState(event);
            if (eventNeedsSnapshotRefresh(event)) {
              scheduleSnapshotRefresh(event.accountId);
            }
          }
        } catch {
          // Page navigation, auth refresh, daemon restart, and transient network loss all land here.
        } finally {
          if (!stopped) {
            setStreaming(false);
          }
        }
        if (!stopped) {
          await sleep(retryMs);
          retryMs = Math.min(retryMs * 2, 10000);
        }
      }
    })();

    return () => {
      stopped = true;
      abort.abort();
    };
  }, [applyEventToLocalState, scheduleSnapshotRefresh]);

  useEffect(() => {
    return () => {
      if (snapshotRefreshTimerRef.current) {
        clearTimeout(snapshotRefreshTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!selectedAccountId) {
      return;
    }
    autoScrollLogRef.current = true;
  }, [selectedAccountId]);

  useEffect(() => {
    const viewport = logViewportRef.current;
    if (!viewport || !autoScrollLogRef.current) return;
    const frame = requestAnimationFrame(() => {
      viewport.scrollTop = viewport.scrollHeight;
    });
    return () => cancelAnimationFrame(frame);
  }, [events.length, selectedAccountId]);

  const handleLogScroll = useCallback(() => {
    const viewport = logViewportRef.current;
    if (!viewport) return;
    autoScrollLogRef.current = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight < 32;
  }, []);

  const scrollLogToBottom = useCallback(() => {
    const viewport = logViewportRef.current;
    if (!viewport) return;
    autoScrollLogRef.current = true;
    requestAnimationFrame(() => {
      viewport.scrollTo({ top: viewport.scrollHeight, behavior: "smooth" });
    });
  }, []);

  const clearSelectedLogs = useCallback(() => {
    if (!selectedAccountId) return;
    autoScrollLogRef.current = true;
    setEvents((prev) => prev.filter((event) => event.accountId !== selectedAccountId));
  }, [selectedAccountId]);

  async function loginAccount(accountId: string, accountName: string) {
    setBusyAccountId(accountId);
    setError("");
    try {
      await accountClient.loginAccount({ id: accountId, name: accountName });
      setSelectedAccountId(accountId);
      await fetchData();
      await Promise.all([
        refreshSelectedSnapshot({ accountId, showLoading: true, clearOnError: true, skipConnectedCheck: true }),
        refreshPolicy(accountId),
      ]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录账号失败");
    } finally {
      setBusyAccountId("");
    }
  }

  async function logoutAccount(accountId: string, accountName: string) {
    setBusyAccountId(accountId);
    setError("");
    try {
      await accountClient.logoutAccount({ id: accountId, name: accountName });
      await fetchData();
      if (accountId === selectedAccountId) {
        setSelectedSnapshot(null);
        await refreshPolicy();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "退出登录失败");
    } finally {
      setBusyAccountId("");
    }
  }

  async function runSelectedAction(action: "start" | "stop") {
    if (!selectedAccount) return;
    setWorkspaceBusy(action);
    setError("");
    try {
      if (action === "start") {
        await automationClient.start({ accountId: selectedAccount.id, accountName: selectedAccount.name });
      } else {
        await automationClient.stop({ accountId: selectedAccount.id, accountName: selectedAccount.name });
      }
      await Promise.all([
        fetchData(),
        refreshSelectedSnapshot({ showLoading: true, clearOnError: true, skipConnectedCheck: action === "start" }),
        refreshPolicy(),
      ]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    } finally {
      setWorkspaceBusy("");
    }
  }

  async function saveSelectedPolicy() {
    if (!policy || !selectedAccount) return;
    if (policyAccountId !== selectedAccount.id) {
      setPolicyMessage("策略仍在加载，请稍后再保存");
      return;
    }
    setSavingPolicy(true);
    setPolicyMessage("");
    try {
      await policyClient.setPolicy({ accountId: selectedAccount.id, policy });
      setPolicyMessage("已保存");
      await Promise.all([refreshStatus(), refreshPolicy()]);
      setTimeout(() => setPolicyMessage(""), 1800);
    } catch (err) {
      setPolicyMessage(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSavingPolicy(false);
    }
  }

  const maxAccounts = user?.maxAccounts ?? 5;
  const onlineCount = Array.from(statuses.values()).filter((status) => status.connected).length;
  const offlineCount = Math.max(accounts.length - onlineCount, 0);
  const selectedAccount = accounts.find((account) => account.id === selectedAccountId) ?? null;
  const selectedStatus = selectedAccount ? statuses.get(selectedAccount.id) : undefined;
  const selectedPolicy = policyAccountId === selectedAccountId ? policy : null;
  const selectedEvents = selectedAccountId
    ? events.filter((event) => event.accountId === selectedAccountId).slice(-MAX_EVENT_ROWS)
    : [];

  if (loading) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="flex min-h-full flex-col gap-4 xl:h-full xl:min-h-0 xl:overflow-hidden">
      {error && (
        <div className="shrink-0 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      {accounts.length > 0 ? (
        <div className="grid gap-4 xl:min-h-0 xl:flex-1 xl:grid-cols-[280px_minmax(0,1fr)] xl:overflow-hidden">
          <Card className="bg-card/95 shadow-sm shadow-black/5 xl:min-h-0">
            <CardHeader className="space-y-3 border-b border-border/70 pb-3">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <CardTitle className="flex items-center gap-2 text-base">
                    <LayoutList className="size-4 text-primary" />
                    账号
                  </CardTitle>
                </div>
                <div className="flex items-center gap-1.5">
                  <Button variant="outline" size="icon-sm" onClick={fetchData} aria-label="刷新账号列表">
                    <RefreshCw className="size-3.5" />
                  </Button>
                  <Button size="icon-sm" onClick={() => setShowAddDialog(true)} disabled={accounts.length >= maxAccounts} aria-label="添加账号">
                    <Plus className="size-3.5" />
                  </Button>
                </div>
              </div>
              <div className="grid grid-cols-3 overflow-hidden rounded-md border border-border/70 bg-muted/20 text-center">
                <HeaderMetric label="配额" value={`${accounts.length}/${maxAccounts}`} />
                <HeaderMetric label="在线" value={onlineCount} />
                <HeaderMetric label="未登录" value={offlineCount} />
              </div>
            </CardHeader>
            <CardContent className="space-y-1.5 p-2 xl:dark-scrollbar xl:min-h-0 xl:flex-1 xl:overflow-y-auto">
              {accounts.map((account) => (
                <AccountRow
                  key={account.id}
                  account={account}
                  status={statuses.get(account.id)}
                  selected={account.id === selectedAccountId}
                  busy={busyAccountId === account.id}
                  onSelect={() => setSelectedAccountId(account.id)}
                  onLogin={() => loginAccount(account.id, account.name)}
                  onLogout={() => logoutAccount(account.id, account.name)}
                />
              ))}
            </CardContent>
          </Card>

          <AccountWorkspace
            account={selectedAccount}
            status={selectedStatus}
            snapshot={selectedSnapshot}
            policy={selectedPolicy}
            events={selectedEvents}
            streaming={streaming}
            logViewportRef={logViewportRef}
            onLogScroll={handleLogScroll}
            onClearLog={clearSelectedLogs}
            onScrollLogToBottom={scrollLogToBottom}
            loading={snapshotLoading}
            busy={selectedAccount ? busyAccountId === selectedAccount.id || !!workspaceBusy : false}
            onRefresh={refreshSelectedSnapshot}
            onLogin={() => selectedAccount && loginAccount(selectedAccount.id, selectedAccount.name)}
            onLogout={() => selectedAccount && logoutAccount(selectedAccount.id, selectedAccount.name)}
            onStart={() => runSelectedAction("start")}
            onStop={() => runSelectedAction("stop")}
            onOpenSettings={() => setSettingsOpen(true)}
          />
        </div>
      ) : (
        <Card className="border-dashed bg-card/70 xl:flex-1">
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <CatalogIcon itemId={23006} className="mb-4 size-14 rounded-md bg-muted p-2" fallback={<Sprout className="size-6 text-muted-foreground" />} />
            <p className="font-medium">还没有游戏账号</p>
            <p className="mt-1 text-sm text-muted-foreground">添加账号后就能在首页切换查看状态。</p>
            <Button className="mt-5" onClick={() => setShowAddDialog(true)}>
              <Plus className="size-4" />
              添加第一个账号
            </Button>
          </CardContent>
        </Card>
      )}

      <AddAccountDialog
        open={showAddDialog}
        onOpenChange={setShowAddDialog}
        onSuccess={fetchData}
      />
      <SettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        accountName={selectedAccount?.name ?? ""}
        policy={selectedPolicy}
        setPolicy={setPolicy}
        saving={savingPolicy}
        message={policyMessage}
        onSave={saveSelectedPolicy}
      />
    </div>
  );
}

function HeaderMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-16 border-r border-border/70 px-3 py-1.5 last:border-r-0">
      <div className="text-[10px] text-muted-foreground">{label}</div>
      <div className="text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function parseEventPayload(payloadJson: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(payloadJson || "{}");
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as Record<string, unknown> : null;
  } catch {
    return null;
  }
}

function recordField(source: Record<string, unknown>, key: string): Record<string, unknown> | null {
  const value = source[key];
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function arrayField(source: Record<string, unknown>, key: string): Record<string, unknown>[] {
  const value = source[key];
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item));
}

function numberField(source: Record<string, unknown>, key: string, fallback: number): number {
  const value = source[key];
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return fallback;
}

function numberFieldAny(source: Record<string, unknown>, keys: string[], fallback: number): number {
  for (const key of keys) {
    const value = numberField(source, key, Number.NaN);
    if (Number.isFinite(value)) return value;
  }
  return fallback;
}

function booleanField(source: Record<string, unknown> | null, key: string, fallback: boolean | undefined): boolean | undefined {
  if (!source) return fallback;
  const value = source[key];
  if (typeof value === "boolean") return value;
  if (typeof value === "string") {
    if (value === "true") return true;
    if (value === "false") return false;
  }
  return fallback;
}

function bigintField(source: Record<string, unknown>, key: string, fallback: bigint): bigint {
  const value = source[key];
  if (typeof value === "bigint") return value;
  if (typeof value === "number" && Number.isFinite(value)) return BigInt(Math.trunc(value));
  if (typeof value === "string" && value.trim() !== "") {
    try {
      return BigInt(value);
    } catch {
      return fallback;
    }
  }
  return fallback;
}

function bigintFieldAny(source: Record<string, unknown>, keys: string[], fallback: bigint): bigint {
  for (const key of keys) {
    const value = bigintField(source, key, fallback);
    if (value !== fallback || source[key] !== undefined) return value;
  }
  return fallback;
}

function applyLandAfter(current: LandView, after: Record<string, unknown>): LandView {
  const next = {
    ...current,
    flowerId: numberFieldAny(after, ["flowerId", "flower_id"], current.flowerId),
    state: numberField(after, "state", current.state),
    lvl: numberField(after, "lvl", current.lvl),
    harvestCnt: numberFieldAny(after, ["harvestCnt", "harvest_cnt"], current.harvestCnt),
    nextTimeMs: bigintFieldAny(after, ["nextTime", "next_time_ms"], current.nextTimeMs),
    plantTimeMs: bigintFieldAny(after, ["plantTime", "plant_time_ms"], current.plantTimeMs),
    observed: true,
    landStatus: "opened",
  };
  return {
    ...next,
    ...landRecommendation(next),
  };
}

function landRecommendation(land: LandView): Pick<LandView, "recommendation" | "reason"> {
  if (!land.observed) {
    return { recommendation: "unknown", reason: "no observed primary state" };
  }
  if (land.flowerId <= 0) {
    return { recommendation: "plant", reason: "land is empty" };
  }
  if (land.state === 3) {
    return { recommendation: "harvest", reason: "state=3 (initial bloom ready)" };
  }
  if (land.state === 2) {
    const nextTime = Number(land.nextTimeMs);
    if (nextTime > 0 && nextTime <= Date.now()) {
      return { recommendation: "harvest", reason: `state=2, nextTime(${nextTime}) elapsed` };
    }
    return { recommendation: "wait", reason: `state=2 regrowing; nextTime=${nextTime}` };
  }
  if (land.state === 1) {
    return { recommendation: "water", reason: "state=1, awaiting first water" };
  }
  return { recommendation: "wait", reason: `state=${land.state} not actionable` };
}

function inventoryField(source: Record<string, unknown>, key: string, fallback: GetSnapshotResponse["inventory"]): GetSnapshotResponse["inventory"] {
  const value = source[key];
  if (!value || typeof value !== "object" || Array.isArray(value)) return fallback;
  const next: Record<number, number> = {};
  for (const [rawId, rawCount] of Object.entries(value)) {
    const id = Number(rawId);
    const count = typeof rawCount === "number" ? rawCount : typeof rawCount === "string" ? Number(rawCount) : NaN;
    if (Number.isFinite(id) && Number.isFinite(count)) {
      next[id] = count;
    }
  }
  return next;
}

function eventNeedsSnapshotRefresh(event: Event): boolean {
  return SNAPSHOT_REFRESH_EVENT_KINDS.has(event.kind) || SNAPSHOT_REFRESH_EVENT_CATEGORIES.has(event.category);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function AccountRow({
  account,
  status,
  selected,
  busy,
  onSelect,
  onLogin,
  onLogout,
}: {
  account: Account;
  status?: AccountStatus;
  selected: boolean;
  busy: boolean;
  onSelect: () => void;
  onLogin: () => void;
  onLogout: () => void;
}) {
  const connected = status?.connected ?? account.connected;
  const username = account.username && account.username !== account.name ? account.username : "";

  return (
    <div className={cn("flex items-center gap-2 rounded-md border border-border/70 bg-card/70 p-1.5 transition-colors hover:border-primary/25 hover:bg-muted/20", selected && "border-primary/45 bg-primary/8 ring-1 ring-primary/20")}>
      <button type="button" className="flex min-w-0 flex-1 items-center gap-2 text-left" onClick={onSelect}>
        <AccountAvatar account={account} connected={connected} className="size-8 rounded-md" />
        <div className="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-1.5">
          <div className="grid min-w-0 gap-0.5">
            <span className="min-w-0 truncate text-sm font-semibold leading-4">{account.name}</span>
            {username && <div className="truncate text-[11px] leading-3 text-muted-foreground">{username}</div>}
          </div>
          <StatusDot connected={connected} />
        </div>
      </button>

      {connected ? (
        <Button variant="outline" size="sm" className="h-8 shrink-0 px-2.5" disabled={busy} onClick={onLogout}>
          <LogOut className="size-3.5" />
          {busy ? "退出中" : "退出"}
        </Button>
      ) : (
        <Button size="sm" className="h-8 shrink-0 px-2.5" disabled={busy} onClick={onLogin}>
          <LogIn className="size-3.5" />
          {busy ? "登录中" : "登录"}
        </Button>
      )}
    </div>
  );
}

const ACCOUNT_AVATAR_PALETTE = [
  ["#16a34a", "#bbf7d0", "#052e16"],
  ["#0891b2", "#cffafe", "#083344"],
  ["#ca8a04", "#fef3c7", "#422006"],
  ["#db2777", "#fce7f3", "#500724"],
  ["#7c3aed", "#ede9fe", "#2e1065"],
  ["#0f766e", "#ccfbf1", "#042f2e"],
  ["#ea580c", "#ffedd5", "#431407"],
  ["#2563eb", "#dbeafe", "#172554"],
];

function AccountAvatar({ account, connected, className }: { account: Account; connected: boolean; className?: string }) {
  const aid = account.aid.toString();
  const seed = aid !== "0" ? aid : `${account.id}:${account.name}:${account.username}`;
  const hash = hashString(seed);
  const [solid, soft, text] = ACCOUNT_AVATAR_PALETTE[hash % ACCOUNT_AVATAR_PALETTE.length];
  const initial = (account.name || account.username || "?").trim().slice(0, 1).toUpperCase();

  return (
    <div
      className={cn("relative grid shrink-0 place-items-center overflow-hidden ring-1 ring-border/70", className)}
      style={{
        background: `radial-gradient(circle at 30% 24%, ${soft} 0, ${soft} 28%, transparent 29%), linear-gradient(135deg, ${solid}, ${soft})`,
        color: text,
      }}
      aria-hidden="true"
    >
      <span className="text-sm font-semibold leading-none">{initial}</span>
      <span className={cn("absolute bottom-0.5 right-0.5 size-2 rounded-full ring-1 ring-background", connected ? "bg-primary" : "bg-muted-foreground/45")} />
    </div>
  );
}

function hashString(value: string): number {
  let hash = 2166136261;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function AccountWorkspace({
  account,
  status,
  snapshot,
  policy,
  events,
  streaming,
  logViewportRef,
  onLogScroll,
  onClearLog,
  onScrollLogToBottom,
  loading,
  busy,
  onRefresh,
  onLogin,
  onLogout,
  onStart,
  onStop,
  onOpenSettings,
}: {
  account: Account | null;
  status?: AccountStatus;
  snapshot: GetSnapshotResponse | null;
  policy: Policy | null;
  events: Event[];
  streaming: boolean;
  logViewportRef: RefObject<HTMLDivElement | null>;
  onLogScroll: () => void;
  onClearLog: () => void;
  onScrollLogToBottom: () => void;
  loading: boolean;
  busy: boolean;
  onRefresh: () => void;
  onLogin: () => void;
  onLogout: () => void;
  onStart: () => void;
  onStop: () => void;
  onOpenSettings: () => void;
}) {
  if (!account) {
    return (
      <Card className="bg-card/95 xl:min-h-0">
        <CardContent className="flex min-h-64 items-center justify-center text-sm text-muted-foreground xl:h-full xl:min-h-0">
          选择一个账号
        </CardContent>
      </Card>
    );
  }

  const connected = status?.connected ?? account.connected;
  const automationEnabled = policy?.automationEnabled ?? status?.automationEnabled ?? false;

  return (
    <Card className="bg-card/95 shadow-sm shadow-black/5 xl:min-h-0">
      <CardHeader className="border-b border-border/70 pb-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <AccountAvatar account={account} connected={connected} className="size-11 rounded-md" />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <CardTitle className="truncate text-lg">{account.name}</CardTitle>
                <StatusBadge connected={connected} />
              </div>
              <p className="mt-1 truncate text-xs text-muted-foreground">
                {account.username || "未记录用户名"}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {connected ? (
              <Button variant="outline" size="sm" disabled={busy} onClick={onLogout}>
                <LogOut className="size-3.5" />
                退出登录
              </Button>
            ) : (
              <Button size="sm" disabled={busy} onClick={onLogin}>
                <LogIn className="size-3.5" />
                登录
              </Button>
            )}
            {automationEnabled ? (
              <Button variant="outline" size="sm" disabled={busy} onClick={onStop}>
                <Square className="size-3.5" />
                停止
              </Button>
            ) : (
              <Button size="sm" disabled={busy} onClick={onStart}>
                <Play className="size-3.5" />
                启动
              </Button>
            )}
            <Button variant="outline" size="sm" disabled={loading} onClick={onRefresh}>
              <RefreshCw className={cn("size-3.5", loading && "animate-spin")} />
              刷新
            </Button>
            <Button variant="outline" size="sm" onClick={onOpenSettings}>
              <SlidersHorizontal className="size-3.5" />
              设置
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="p-4 xl:min-h-0 xl:flex-1 xl:overflow-hidden">
        {loading ? (
          <div className="grid gap-3">
            <div className="h-20 animate-pulse rounded-md bg-muted/35" />
            <div className="h-72 animate-pulse rounded-md bg-muted/25" />
          </div>
        ) : snapshot ? (
          <SnapshotOverview
            snapshot={snapshot}
            events={events}
            streaming={streaming}
            logScrollKey={account.id}
            logViewportRef={logViewportRef}
            onLogScroll={onLogScroll}
            onClearLog={onClearLog}
            onScrollLogToBottom={onScrollLogToBottom}
          />
        ) : (
          <div className="flex min-h-72 flex-col items-center justify-center rounded-md border border-dashed border-border/70 bg-muted/15 text-center xl:h-full xl:min-h-0">
            <WifiOff className="mb-3 size-7 text-muted-foreground" />
            <p className="font-medium">暂无运行快照</p>
            <p className="mt-1 max-w-sm text-sm text-muted-foreground">账号登录并启动 runner 后，田地、库存和资源会直接显示在这里。</p>
            {!connected && (
              <Button className="mt-5" onClick={onLogin} disabled={busy}>
                <LogIn className="size-4" />
                登录账号
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function SnapshotOverview({
  snapshot,
  events,
  streaming,
  logScrollKey,
  logViewportRef,
  onLogScroll,
  onClearLog,
  onScrollLogToBottom,
}: {
  snapshot: GetSnapshotResponse;
  events: Event[];
  streaming: boolean;
  logScrollKey: string;
  logViewportRef: RefObject<HTMLDivElement | null>;
  onLogScroll: () => void;
  onClearLog: () => void;
  onScrollLogToBottom: () => void;
}) {
  const landTiles = [...snapshot.lands]
    .sort((a, b) => a.landId - b.landId)
    .map((land) => {
      const landState = landStatus(land);
      return {
        land,
        landState,
        plantingState: plantingStatus(land, landState),
      } satisfies LandTile;
    });
  const opened = landTiles.filter((tile) => tile.landState === "opened").length;
  const wasteland = landTiles.filter((tile) => tile.landState === "wasteland").length;
  const locked = landTiles.filter((tile) => tile.landState === "locked").length;
  const empty = landTiles.filter((tile) => tile.plantingState === "empty").length;
  const needsWater = landTiles.filter((tile) => tile.plantingState === "needs_water").length;
  const growing = landTiles.filter((tile) => tile.plantingState === "growing").length;
  const ready = landTiles.filter((tile) => tile.plantingState === "ready").length;
  const inventory = inventoryEntries(snapshot.inventory);

  return (
    <div className="flex flex-col gap-4 xl:h-full xl:min-h-0 xl:overflow-hidden">
      <div className="grid shrink-0 gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <ResourceChip icon={<Coins className="size-4" />} label={itemName(11)} value={formatCount(snapshot.gold)} tone="amber" />
        <ResourceChip
          icon={<Droplets className="size-4" />}
          label={itemName(7)}
          value={snapshot.waterDropsTotal > 0 ? `${formatCount(snapshot.waterDrops)}/${formatCount(snapshot.waterDropsTotal)}` : formatCount(snapshot.waterDrops)}
          tone="cyan"
        />
        <ResourceChip icon={<Gem className="size-4" />} label={itemName(1)} value={formatCount(snapshot.diamondsFree + snapshot.diamondsPaid)} tone="rose" />
        <ResourceChip icon={<TrendingUp className="size-4" />} label="等级" value={`Lv.${snapshot.level} · ${formatCount(snapshot.experience)} EXP`} tone="green" />
      </div>

      <TaskPanel tasks={snapshot.pendingTasks ?? []} />

      <div className="grid gap-4 xl:h-[340px] xl:shrink-0 xl:grid-cols-[320px_minmax(0,1fr)] xl:overflow-hidden 2xl:h-[360px] 2xl:grid-cols-[340px_minmax(0,1fr)]">
        <InventoryPanel inventory={inventory} />

        <section className="flex flex-col rounded-lg border border-border/70 bg-card/55 p-3 shadow-sm shadow-black/5 xl:min-h-0">
          <div className="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2 font-medium">
              <MapIcon className="size-4 text-primary" />
              田地
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-foreground/65">土地</span>
                <LandLegend tone="opened" label={`已开垦 ${opened}`} />
                <LandLegend tone="wasteland" label={`未开垦 ${wasteland}`} />
                <LandLegend tone="locked" label={`未开放 ${locked}`} />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-foreground/65">种植</span>
                <PlantingLegend tone="empty" label={`空地 ${empty}`} />
                <PlantingLegend tone="needs_water" label={`待浇水 ${needsWater}`} />
                <PlantingLegend tone="growing" label={`生长中 ${growing}`} />
                <PlantingLegend tone="ready" label={`可收获 ${ready}`} />
              </div>
            </div>
          </div>
          {landTiles.length > 0 ? (
            <div className="rounded-lg border border-border/60 bg-muted/10 p-2 xl:min-h-0 xl:flex-1 xl:overflow-hidden">
              <div className="grid grid-cols-4 gap-2 sm:grid-cols-8 xl:h-full xl:grid-cols-16 xl:auto-rows-fr">
                {landTiles.map((tile) => <LandCell key={tile.land.landId} tile={tile} />)}
              </div>
            </div>
          ) : (
            <div className="flex min-h-32 items-center justify-center rounded-md border border-dashed border-border/70 text-sm text-muted-foreground xl:min-h-0 xl:flex-1">
              暂无田地数据
            </div>
          )}
        </section>
      </div>

      <EventLog
        events={events}
        streaming={streaming}
        scrollKey={logScrollKey}
        logViewportRef={logViewportRef}
        onScroll={onLogScroll}
        onClear={onClearLog}
        onScrollToBottom={onScrollLogToBottom}
      />
    </div>
  );
}

type LandTile = {
  land: LandView;
  landState: "opened" | "wasteland" | "locked";
  plantingState: "unavailable" | "empty" | "needs_water" | "growing" | "ready" | "unknown";
};

type InventoryEntry = {
  id: number;
  name: string;
  count: number;
  item?: ItemInfo;
  category: string;
};

type InventoryGroup = {
  category: string;
  entries: InventoryEntry[];
};

function TaskPanel({ tasks }: { tasks: PendingTaskView[] }) {
  const groups = useMemo(() => groupPendingTasks(tasks), [tasks]);
  const plantMissing = useMemo(() => {
    const byItem = new Map<number, { itemId: number; itemName: string; missing: number }>();
    for (const task of tasks) {
      for (const req of task.requirements) {
        if (!req.plantingRelevant || req.missing <= 0) continue;
        const current = byItem.get(req.itemId);
        if (current) {
          current.missing += req.missing;
        } else {
          byItem.set(req.itemId, { itemId: req.itemId, itemName: req.itemName, missing: req.missing });
        }
      }
    }
    return Array.from(byItem.values()).sort((a, b) => b.missing - a.missing || a.itemId - b.itemId);
  }, [tasks]);

  return (
    <section className="shrink-0 rounded-lg border border-border/70 bg-card/55 p-3 shadow-sm shadow-black/5">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 font-medium">
          <ListChecks className="size-4 text-primary" />
          自动待办
        </div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {plantMissing.length > 0 && (
            <span className="inline-flex items-center gap-1 rounded border border-primary/20 bg-primary/10 px-2 py-1 text-primary">
              <Sprout className="size-3" />
              {plantMissing.length} 种鲜花缺口
            </span>
          )}
          <span>{tasks.length} 项</span>
        </div>
      </div>

      {groups.length > 0 ? (
        <div className="grid gap-2 lg:grid-cols-2 2xl:grid-cols-4">
          {groups.map((group) => (
            <div key={group.category} className="flex max-h-48 min-w-0 flex-col rounded-md border border-border/60 bg-background/45 p-2">
              <div className="mb-2 flex shrink-0 items-center justify-between gap-2">
                <span className="truncate text-xs font-medium">{group.category}</span>
                <span className="text-[10px] text-muted-foreground">{group.tasks.length}</span>
              </div>
              {group.tasks.length > 0 ? (
                <div className="dark-scrollbar grid gap-1.5 overflow-y-auto pr-1">
                  {group.tasks.map((task) => (
                    <TaskRow key={`${task.category}:${task.id}`} task={task} />
                  ))}
                </div>
              ) : (
                <div className="flex min-h-20 items-center justify-center rounded-md border border-dashed border-border/70 px-3 text-center text-xs text-muted-foreground">
                  {emptyTaskGroupText(group.category)}
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="flex min-h-20 items-center justify-center rounded-md border border-dashed border-border/70 text-sm text-muted-foreground">
          暂无任务数据
        </div>
      )}
    </section>
  );
}

function TaskRow({ task }: { task: PendingTaskView }) {
  const title = taskDisplayTitle(task);
  return (
    <div className="rounded-md border border-border/50 bg-card/55 p-2">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate text-xs font-medium">{title}</div>
          {task.target > 0 && (
            <div className="mt-0.5 text-[10px] tabular-nums text-muted-foreground">
              {formatCount(task.finished)}/{formatCount(task.target)}
            </div>
          )}
        </div>
        <TaskStatusBadge status={task.status} />
      </div>
      {task.requirements.length > 0 && (
        <div className={cn("mt-2 grid gap-1", task.category === "居民订单" && "grid-cols-[repeat(auto-fit,minmax(0,1fr))]")}>
          {task.requirements.map((req) => (
            <RequirementRow key={`${task.category}:${task.id}:${req.itemId}`} req={req} />
          ))}
        </div>
      )}
    </div>
  );
}

const dailyTaskTitleTemplates: Record<string, string> = {
  "40001": "消耗${value}个剧情星星",
  "30060001": "完成${value}次居民订单",
  "30140001": "浇水${value}块田地",
  "30150001": "在花架出售${value}件花艺品",
  "30160001": "完成${value}次顾客订单",
  "30170001": "在材料商店购买${value}次道具",
  "30180001": "完成${value}次宫廷特供",
  "30230001": "采集珍珠雇佣${value}次",
  "30240001": "摘取好友${value}朵花",
  "30250001": "观看${value}次视频",
  "30520001": "和小动物互动${value}次",
};

function taskDisplayTitle(task: PendingTaskView): string {
  const title = task.title || "";
  const fallbackTitle = `${task.category} #${task.id}`;
  if (task.category !== "日常任务") {
    return title || fallbackTitle;
  }
  const match = title.match(/^日常任务 #(\d+)$/);
  const taskID = match?.[1] || task.id;
  const template = dailyTaskTitleTemplates[taskID];
  if (!template) {
    return title || fallbackTitle;
  }
  return template.replace("${value}", formatCount(task.target));
}

function RequirementRow({ req }: { req: RequirementView }) {
  return (
    <div className="grid grid-cols-[26px_minmax(0,1fr)_auto] items-center gap-1.5 rounded border border-border/45 bg-background/55 px-1.5 py-1">
      <CatalogIcon itemId={req.itemId} className="size-6 rounded bg-muted/45 p-0.5" fallback={<Package className="size-3.5 text-muted-foreground" />} />
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-1">
          <span className="truncate text-[11px] font-medium">{req.itemName || itemName(req.itemId)}</span>
          {req.plantingRelevant && <Sprout className="size-3 shrink-0 text-primary" />}
        </div>
        <div className="text-[10px] tabular-nums text-muted-foreground">
          {formatCount(req.owned)}/{formatCount(req.required)}
        </div>
      </div>
      <div className={cn("text-[11px] font-semibold tabular-nums", req.missing > 0 ? "text-rose-600 dark:text-rose-300" : "text-primary")}>
        {req.missing > 0 ? `缺 ${formatCount(req.missing)}` : "可交"}
      </div>
    </div>
  );
}

function TaskStatusBadge({ status }: { status: string }) {
  const label = status === "ready" ? "可提交" : status === "missing" ? "缺材料" : "进行中";
  return (
    <span
      className={cn(
        "shrink-0 rounded border px-1.5 py-0.5 text-[10px]",
        status === "ready" && "border-primary/25 bg-primary/10 text-primary",
        status === "missing" && "border-rose-400/25 bg-rose-400/10 text-rose-700 dark:text-rose-200",
        status !== "ready" && status !== "missing" && "border-border/70 bg-muted/30 text-muted-foreground"
      )}
    >
      {label}
    </span>
  );
}

function InventoryPanel({ inventory }: { inventory: InventoryEntry[] }) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [keyword, setKeyword] = useState("");
  const filteredInventory = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return inventory;
    return inventory.filter((entry) => {
      return String(entry.id).includes(q) || entry.name.toLowerCase().includes(q) || entry.category.toLowerCase().includes(q);
    });
  }, [inventory, keyword]);
  const groups = useMemo(() => groupInventoryEntries(filteredInventory), [filteredInventory]);

  return (
    <section className="flex max-h-80 flex-col rounded-lg border border-border/70 bg-card/55 p-3 shadow-sm shadow-black/5 xl:max-h-none xl:min-h-0">
      <div className="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 font-medium">
          <Package className="size-4 text-primary" />
          库存
        </div>
        <div className="text-xs text-muted-foreground">{groups.length} 类 · {filteredInventory.length} 项</div>
      </div>
      <Input
        className="mb-2 h-8 shrink-0 text-xs"
        placeholder="搜索库存名称、ID 或分类"
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
      />
      {inventory.length > 0 ? (
        <div className="dark-scrollbar min-h-0 overflow-y-auto pr-1 xl:flex-1">
          {filteredInventory.length > 0 ? (
            <div className="grid gap-2">
              {groups.map((group) => {
                const isCollapsed = collapsed[group.category] ?? false;
                return (
                  <div key={group.category} className="overflow-hidden rounded-lg border border-border/70 bg-card/50">
                    <button
                      type="button"
                      className="flex w-full items-center justify-between gap-2 bg-muted/20 px-2.5 py-2 text-left transition-colors hover:bg-muted/35"
                      onClick={() => setCollapsed((prev) => ({ ...prev, [group.category]: !isCollapsed }))}
                    >
                      <span className="min-w-0 truncate text-xs font-medium">{group.category}</span>
                      <span className="flex items-center gap-1.5 text-[10px] text-muted-foreground">
                        {group.entries.length}
                        <ChevronDown className={cn("size-3.5 transition-transform", isCollapsed && "-rotate-90")} />
                      </span>
                    </button>
                    {!isCollapsed && (
                      <div className="grid gap-1.5 p-1.5">
                        {group.entries.map((entry) => (
                          <InventoryItemRow key={entry.id} entry={entry} />
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="flex h-full min-h-28 items-center justify-center rounded-md border border-dashed border-border/70 text-xs text-muted-foreground">
              没有匹配的库存
            </div>
          )}
        </div>
      ) : (
        <div className="flex min-h-32 items-center justify-center rounded-md border border-dashed border-border/70 text-sm text-muted-foreground xl:min-h-0 xl:flex-1">
          暂无库存数据
        </div>
      )}
    </section>
  );
}

function InventoryItemRow({ entry }: { entry: InventoryEntry }) {
  return (
    <div className="grid grid-cols-[34px_minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-border/60 bg-background/55 px-2 py-1.5">
      <CatalogIcon itemId={entry.id} className="size-8 rounded-md bg-muted/45 p-1" fallback={<Package className="size-4 text-muted-foreground" />} />
      <div className="min-w-0">
        <div className="truncate text-sm font-medium">{entry.name}</div>
        <div className="text-[11px] text-muted-foreground">#{entry.id}</div>
      </div>
      <div className="text-sm font-semibold tabular-nums">{formatCount(entry.count)}</div>
    </div>
  );
}

function landStatus(land: LandView): LandTile["landState"] {
  if (land.landStatus === "locked" || land.landStatus === "wasteland" || land.landStatus === "opened") {
    return land.landStatus;
  }
  return land.observed ? "opened" : "locked";
}

function plantingStatus(land: LandView, landState: LandTile["landState"]): LandTile["plantingState"] {
  if (landState !== "opened") return "unavailable";
  if (land.flowerId <= 0) return "empty";
  if (land.state === 1) return "needs_water";
  if (land.state === 3) return "ready";
  if (land.state === 2) {
    const nextTime = Number(land.nextTimeMs);
    return nextTime > 0 && nextTime <= Date.now() ? "ready" : "growing";
  }
  if (land.recommendation === "harvest") return "ready";
  if (land.recommendation === "water") return "needs_water";
  return "unknown";
}

function landStateLabel(state: LandTile["landState"]) {
  return state === "locked" ? "未开放" : state === "wasteland" ? "未开垦" : "已开垦";
}

function plantingStateLabel(state: LandTile["plantingState"]) {
  const labels = {
    unavailable: "不可种植",
    empty: "空地",
    needs_water: "待浇水",
    growing: "生长中",
    ready: "可收获",
    unknown: "未知",
  };
  return labels[state];
}

function LandLegend({ tone, label }: { tone: LandTile["landState"]; label: string }) {
  const toneClass = {
    opened: "bg-primary",
    wasteland: "bg-amber-400",
    locked: "bg-muted-foreground/45",
  }[tone];
  return (
    <span className="inline-flex items-center gap-1">
      <span className={cn("size-1.5 rounded-full", toneClass)} />
      {label}
    </span>
  );
}

function PlantingLegend({ tone, label }: { tone: Exclude<LandTile["plantingState"], "unavailable" | "unknown">; label: string }) {
  const toneClass = {
    empty: "bg-muted-foreground/45",
    needs_water: "bg-cyan-500",
    growing: "bg-amber-500",
    ready: "bg-emerald-500",
  }[tone];
  return (
    <span className="inline-flex items-center gap-1">
      <span className={cn("size-1.5 rounded-full", toneClass)} />
      {label}
    </span>
  );
}

function LandCell({ tile }: { tile: LandTile }) {
  const { land, landState, plantingState } = tile;
  const planted = land.flowerId > 0;
  const landLabel = landStateLabel(landState);
  const plantingLabel = plantingStateLabel(plantingState);
  const flower = planted ? itemName(land.flowerId) : "";
  const titleParts = [`${land.landId}`, `土地：${landLabel}`];
  if (land.openLevel) titleParts.push(`开放 Lv.${land.openLevel}`);
  if (land.unlockCost.length >= 2) titleParts.push(`开垦消耗 ${itemName(land.unlockCost[0]) || `#${land.unlockCost[0]}`} x${land.unlockCost[1]}`);
  if (landState === "opened") titleParts.push(`种植：${plantingLabel}`);
  if (flower) titleParts.push(`作物：${flower}`);

  return (
    <div
      className={cn(
        "group/land relative flex min-h-[72px] flex-col overflow-hidden rounded-lg border px-1.5 py-1.5 text-[10px] tabular-nums transition-colors duration-150 hover:border-primary/45 hover:bg-primary/5 xl:h-full xl:min-h-0",
        landState === "opened" && "border-primary/25 bg-emerald-50/70 text-emerald-950 dark:bg-emerald-950/20 dark:text-emerald-50",
        landState === "wasteland" && "border-amber-400/35 bg-amber-50/70 text-amber-900 dark:bg-amber-950/20 dark:text-amber-100",
        landState === "locked" && "border-border/60 bg-background/45 text-muted-foreground/70"
      )}
      title={titleParts.join(" · ")}
      aria-label={titleParts.join(" · ")}
    >
      <div className="flex shrink-0 items-center justify-between gap-1 leading-none">
        <span className="font-medium text-foreground/75">{land.landId}</span>
        <span
          className={cn(
            "size-1.5 rounded-full transition-transform group-hover/land:scale-125",
            landState === "opened" && "bg-primary",
            landState === "wasteland" && "bg-amber-400",
            landState === "locked" && "bg-muted-foreground/45"
          )}
        />
      </div>
      <div className="flex min-h-0 flex-1 items-center justify-center">
        {planted && land ? (
          <CatalogIcon itemId={land.flowerId} className="size-8 object-contain drop-shadow-sm transition-transform group-hover/land:scale-110" fallback={<Flower2 className="size-5" />} />
        ) : landState === "locked" ? (
          <LockKeyhole className="size-4 opacity-75" />
        ) : landState === "wasteland" ? (
          <Shovel className="size-5 opacity-85" />
        ) : (
          <Sprout className="size-5 opacity-85" />
        )}
      </div>
      <div className="grid shrink-0 gap-0.5 text-center leading-3">
        <span className={cn("block truncate text-[9px] transition-colors group-hover/land:text-foreground", planted ? "font-medium text-foreground/80" : "text-muted-foreground")}>
          {planted ? flower : landLabel}
        </span>
        {landState === "opened" && (
          <span
            className={cn(
              "mx-auto max-w-full rounded-full border px-1.5 py-px text-[8px] leading-none",
              plantingState === "empty" && "border-border/70 bg-background/70 text-muted-foreground",
              plantingState === "needs_water" && "border-cyan-400/35 bg-cyan-400/15 text-cyan-700 dark:text-cyan-200",
              plantingState === "growing" && "border-amber-400/35 bg-amber-400/15 text-amber-700 dark:text-amber-200",
              plantingState === "ready" && "border-emerald-500/35 bg-emerald-500/15 text-emerald-700 dark:text-emerald-200",
              plantingState === "unknown" && "border-border/70 bg-muted/40 text-muted-foreground"
            )}
          >
            {plantingLabel}
          </span>
        )}
      </div>
    </div>
  );
}

function EventLog({
  events,
  streaming,
  scrollKey,
  logViewportRef,
  onScroll,
  onClear,
  onScrollToBottom,
}: {
  events: Event[];
  streaming: boolean;
  scrollKey: string;
  logViewportRef: RefObject<HTMLDivElement | null>;
  onScroll: () => void;
  onClear: () => void;
  onScrollToBottom: () => void;
}) {
  const [category, setCategory] = useState("all");
  const filteredEvents = events.filter((event) => eventMatchesLogFilter(event, category));

  useLayoutEffect(() => {
    const viewport = logViewportRef.current;
    if (!viewport) return;
    viewport.scrollTop = viewport.scrollHeight;
  }, [scrollKey, category, logViewportRef]);

  return (
    <section className="flex h-80 flex-col rounded-lg border border-border/70 bg-card/55 p-3 shadow-sm shadow-black/5 xl:h-auto xl:min-h-0 xl:flex-1">
      <div className="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2 font-medium">
          <ListChecks className="size-4 text-primary" />
          日志
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex overflow-hidden rounded-md border border-border/70 bg-muted/20 p-0.5">
            {LOG_FILTERS.map((filter) => (
              <button
                key={filter.value}
                type="button"
                className={cn(
                  "rounded px-2 py-1 text-[10px] leading-none text-muted-foreground transition-colors hover:bg-background/80 hover:text-foreground",
                  category === filter.value && "bg-background text-foreground shadow-sm"
                )}
                onClick={() => setCategory(filter.value)}
              >
                {filter.label}
              </button>
            ))}
          </div>
          <Badge variant="secondary" className="text-[10px]">
            <span className={cn("mr-1 size-1.5 rounded-full", streaming ? "animate-pulse bg-emerald-400" : "bg-muted-foreground")} />
            {streaming ? "实时" : "断开"}
          </Badge>
          <Button variant="outline" size="icon-sm" onClick={onScrollToBottom} disabled={events.length === 0} aria-label="滚动到日志底部" title="滚动到底部">
            <ArrowDownToLine className="size-3.5" />
          </Button>
          <Button variant="outline" size="icon-sm" onClick={onClear} disabled={events.length === 0} aria-label="清空当前账号日志" title="清空日志">
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      </div>
      <div ref={logViewportRef} onScroll={onScroll} className="dark-scrollbar min-h-0 flex-1 overflow-y-auto rounded-md border border-border/70 bg-background/70 p-2 font-mono text-[11px]">
        {events.length === 0 ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">等待事件...</div>
        ) : filteredEvents.length === 0 ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">当前分类暂无日志</div>
        ) : filteredEvents.map((event, index) => (
          <div key={`${event.kind}-${index}`} className="grid grid-cols-[54px_112px_minmax(0,1fr)] gap-1.5 rounded px-1 leading-5 hover:bg-muted/45">
            <span className="text-muted-foreground">{event.ts ? new Date(Number(event.ts.seconds) * 1000).toLocaleTimeString("zh-CN", { hour12: false }) : "--:--"}</span>
            <span className={cn("truncate", eventColor(event))}>[{event.label || eventLabel(event.kind)}]</span>
            <span className="break-all text-foreground/80">{event.message}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

const LOG_FILTERS = [
  { value: "all", label: "全部" },
  { value: "error", label: "错误" },
  { value: "operation", label: "操作" },
  { value: "land", label: "田地" },
  { value: "resource", label: "资源" },
  { value: "reward", label: "奖励" },
  { value: "task", label: "任务" },
  { value: "order", label: "订单" },
  { value: "cultivation", label: "培育" },
] as const;

function eventMatchesLogFilter(event: Event, category: string): boolean {
  if (category === "all") return true;
  if (category === "error") return event.level === "error";
  return event.category === category;
}

const PLANT_MODE_OPTIONS = [
  { value: "auto", label: "自动", description: "任务和订单缺花优先，否则选择价值高的花。" },
  { value: "high_value", label: "高价值", description: "始终优先选择金币收益更高的花。" },
  { value: "low_stock", label: "低库存", description: "优先补库存最低的花，保留旧策略。" },
  { value: "selected", label: "自选", description: "只从你勾选的花里选择种植。" },
] as const;

function SettingsDialog({
  open,
  onOpenChange,
  accountName,
  policy,
  setPolicy,
  saving,
  message,
  onSave,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  accountName: string;
  policy: Policy | null;
  setPolicy: (policy: Policy) => void;
  saving: boolean;
  message: string;
  onSave: () => void;
}) {
  const plantMode = policy?.plant?.mode || "auto";
  const [flowerQuery, setFlowerQuery] = useState("");
  const selectedFlowerIds = useMemo(() => new Set(policy?.plant?.allowedFlowerIds ?? []), [policy?.plant?.allowedFlowerIds]);
  const visibleFlowers = useMemo(() => {
    const keyword = flowerQuery.trim().toLowerCase();
    if (!keyword) return FLOWER_OPTIONS;
    return FLOWER_OPTIONS.filter((flower) => {
      const id = String(flower.id);
      const name = itemName(flower.id).toLowerCase();
      return id.includes(keyword) || name.includes(keyword);
    });
  }, [flowerQuery]);
  const updatePlant = (patch: Partial<NonNullable<Policy["plant"]>>) => {
    if (!policy) return;
    setPolicy({ ...policy, plant: { ...policy.plant!, ...patch } });
  };
  const updateWater = (patch: Partial<NonNullable<Policy["water"]>>) => {
    if (!policy) return;
    setPolicy({ ...policy, water: { ...policy.water!, ...patch } });
  };
  const toggleFlower = (flowerId: number) => {
    const ids = new Set(policy?.plant?.allowedFlowerIds ?? []);
    if (ids.has(flowerId)) {
      ids.delete(flowerId);
    } else {
      ids.add(flowerId);
    }
    updatePlant({ allowedFlowerIds: Array.from(ids).sort((a, b) => a - b) });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-hidden sm:max-w-6xl">
        <DialogHeader>
          <DialogTitle>策略设置</DialogTitle>
          <DialogDescription>{accountName ? `调整「${accountName}」的自动化策略` : "选择账号后调整策略"}</DialogDescription>
        </DialogHeader>
        {!policy ? (
          <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">加载策略中...</div>
        ) : (
          <>
            <div className="dark-scrollbar max-h-[66vh] overflow-y-auto pr-1">
              <div className="grid gap-4">
                <div className="grid gap-4 lg:grid-cols-[1fr_2fr]">
                  <Card size="sm">
                    <CardHeader className="pb-2"><CardTitle className="text-sm">运行</CardTitle></CardHeader>
                    <CardContent className="grid gap-3">
                    <Row label="决策间隔">
                      <Input type="number" className="h-8 w-24 text-xs" value={policy.decisionIntervalSeconds || 4} onChange={(e) => setPolicy({ ...policy, decisionIntervalSeconds: parseFloat(e.target.value) })} />
                    </Row>
                    </CardContent>
                  </Card>

                  <Card size="sm">
                    <CardHeader className="pb-2"><CardTitle className="text-sm">收获</CardTitle></CardHeader>
                    <CardContent className="grid gap-2 sm:grid-cols-2">
                      <Row label="自动收获"><Switch checked={policy.harvest?.enabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, harvest: { ...policy.harvest!, enabled: v } })} /></Row>
                      <Row label="一键收获"><Switch checked={policy.harvest?.preferOneKey ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, harvest: { ...policy.harvest!, preferOneKey: v } })} /></Row>
                    </CardContent>
                  </Card>
                </div>

                <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
                  <Card size="sm" className="min-w-0">
                    <CardHeader className="pb-2"><CardTitle className="text-sm">种植</CardTitle></CardHeader>
                    <CardContent className="space-y-3">
                      <div className="grid gap-2 sm:grid-cols-2">
                        <Row label="自动种植"><Switch checked={policy.plant?.enabled ?? true} onCheckedChange={(v: boolean) => updatePlant({ enabled: v })} /></Row>
                        <Row label="批量上限"><Input type="number" className="h-8 w-20 text-xs" value={policy.plant?.maxBatch ?? 8} onChange={(e) => updatePlant({ maxBatch: parseNumber(e.target.value, 8) })} /></Row>
                      </div>

                      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                        {PLANT_MODE_OPTIONS.map((option) => (
                          <button
                            key={option.value}
                            type="button"
                            onClick={() => updatePlant({ mode: option.value })}
                            className={cn(
                              "rounded-lg border border-border/70 bg-muted/15 p-3 text-left transition-all hover:border-primary/35 hover:bg-primary/5",
                              plantMode === option.value && "border-primary/55 bg-primary/10 shadow-sm ring-1 ring-primary/20"
                            )}
                          >
                            <div className="mb-1 flex items-center justify-between gap-2">
                              <span className="text-sm font-medium">{option.label}</span>
                              {plantMode === option.value && <Check className="size-4 text-primary" />}
                            </div>
                            <p className="text-xs leading-5 text-muted-foreground">{option.description}</p>
                          </button>
                        ))}
                      </div>

                      {plantMode === "selected" && (
                        <div className="rounded-lg border border-border/70 bg-muted/10 p-3">
                          <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <div className="text-sm font-medium">选择种子</div>
                              <div className="text-xs text-muted-foreground">已选择 {selectedFlowerIds.size} 种</div>
                            </div>
                            <div className="flex gap-2">
                              <Input className="h-8 w-full text-xs sm:w-48" placeholder="搜索名称或 ID" value={flowerQuery} onChange={(e) => setFlowerQuery(e.target.value)} />
                              <Button type="button" size="sm" variant="outline" className="h-8 shrink-0" onClick={() => updatePlant({ allowedFlowerIds: [] })}>清空</Button>
                            </div>
                          </div>
                          <div className="dark-scrollbar max-h-64 overflow-y-auto pr-1">
                            <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                              {visibleFlowers.map((flower) => (
                                <FlowerOptionButton
                                  key={flower.id}
                                  flower={flower}
                                  selected={selectedFlowerIds.has(flower.id)}
                                  onToggle={() => toggleFlower(flower.id)}
                                />
                              ))}
                            </div>
                          </div>
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  <Card size="sm">
                    <CardHeader className="pb-2"><CardTitle className="text-sm">浇水</CardTitle></CardHeader>
                    <CardContent className="space-y-2">
                      <Row label="自动浇水"><Switch checked={policy.water?.enabled ?? true} onCheckedChange={(v: boolean) => updateWater({ ...policy.water!, enabled: v })} /></Row>
                      <Row label="保留水滴"><Input type="number" min={1} className="h-8 w-24 text-xs" value={policy.water?.minDrops || 5} onChange={(e) => updateWater({ ...policy.water!, minDrops: parseNumberAtLeast(e.target.value, 5, 1) })} /></Row>
                      <Row label="批量上限"><Input type="number" className="h-8 w-24 text-xs" value={policy.water?.maxBatch ?? 8} onChange={(e) => updateWater({ ...policy.water!, maxBatch: parseNumber(e.target.value, 8) })} /></Row>
                    </CardContent>
                  </Card>
                </div>

                <Card size="sm">
                  <CardHeader className="pb-2"><CardTitle className="text-sm">辅助操作</CardTitle></CardHeader>
                  <CardContent className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                    <Row label="自动开垦"><Switch checked={policy.misc?.landUnlockEnabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, landUnlockEnabled: v } })} /></Row>
                    <Row label="领取任务奖励"><Switch checked={policy.misc?.taskRecvEnabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, taskRecvEnabled: v } })} /></Row>
                    <Row label="解锁剧情"><Switch checked={policy.misc?.storyUnlockEnabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, storyUnlockEnabled: v } })} /></Row>
                    <Row label="完成订单"><Switch checked={policy.misc?.orderEnabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, orderEnabled: v } })} /></Row>
                    <Row label="领取水资源"><Switch checked={policy.misc?.waterwheelEnabled ?? true} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, waterwheelEnabled: v } })} /></Row>
                    <Row label="自动培育"><Switch checked={policy.misc?.cultivateEnabled ?? false} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, cultivateEnabled: v } })} /></Row>
                    <Row label="自动升级"><Switch checked={policy.misc?.flowerUpgradeEnabled ?? false} onCheckedChange={(v: boolean) => setPolicy({ ...policy, misc: { ...policy.misc!, flowerUpgradeEnabled: v } })} /></Row>
                  </CardContent>
                </Card>
              </div>
            </div>
            <DialogFooter className="items-center gap-3 border-t border-border/70 pt-4">
              {message && <span className="mr-auto text-xs text-muted-foreground">{message}</span>}
              <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>关闭</Button>
              <Button type="button" disabled={saving} onClick={onSave}>{saving ? "保存中..." : "保存策略"}</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function parseNumber(value: string, fallback: number): number {
	const n = parseInt(value, 10);
	return Number.isFinite(n) ? n : fallback;
}

function parseNumberAtLeast(value: string, fallback: number, min: number): number {
	const n = parseNumber(value, fallback);
	return n < min ? min : n;
}

function FlowerOptionButton({ flower, selected, onToggle }: { flower: FlowerInfo; selected: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className={cn(
        "grid grid-cols-[36px_minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-border/70 bg-card/70 px-2 py-2 text-left transition-all hover:border-primary/35 hover:bg-primary/5",
        selected && "border-primary/55 bg-primary/10 ring-1 ring-primary/20"
      )}
    >
      <CatalogIcon itemId={flower.id} className="size-9 rounded-md bg-muted/45 p-1" fallback={<Flower2 className="size-4 text-muted-foreground" />} />
      <span className="min-w-0">
        <span className="block truncate text-xs font-medium">{itemName(flower.id)}</span>
        <span className="block truncate text-[10px] text-muted-foreground">#{flower.id} · {formatCount(flower.gold || 0)} 金币</span>
      </span>
      <span
        className={cn(
          "flex size-5 items-center justify-center rounded-full border border-border/70 text-[10px]",
          selected ? "border-primary bg-primary text-primary-foreground" : "bg-background text-transparent"
        )}
      >
        <Check className="size-3" />
      </span>
    </button>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/60 bg-muted/20 px-3 py-1.5">
      <Label className="text-xs">{label}</Label>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function AddAccountDialog({
  open,
  onOpenChange,
  onSuccess,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}) {
  const [name, setName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [channel, setChannel] = useState(1);
  const [loginNow, setLoginNow] = useState(true);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim() || !username.trim() || !password) {
      setError("请填写所有字段");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const res = await accountClient.createAccount({
        name: name.trim(),
        username: username.trim(),
        password,
        channel,
        loginNow,
      });
      if (res.loginError) {
        setError(`账号已创建，但登录失败：${res.loginError}`);
        onSuccess();
      } else {
        onOpenChange(false);
        setName("");
        setUsername("");
        setPassword("");
        onSuccess();
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>添加游戏账号</DialogTitle>
          <DialogDescription>输入登录账号信息</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="acc-name">账号别名</Label>
            <Input
              id="acc-name"
              placeholder="例如：主号"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="acc-username">游戏用户名</Label>
            <Input
              id="acc-username"
              placeholder="babigame 登录用户名"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="acc-password">游戏密码</Label>
            <Input
              id="acc-password"
              type="password"
              placeholder="登录密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>
          <div className="space-y-2">
            <Label>渠道</Label>
            <div className="grid grid-cols-2 gap-2">
              <Button type="button" variant={channel === 1 ? "default" : "outline"} onClick={() => setChannel(1)}>
                iOS
              </Button>
              <Button type="button" variant="outline" disabled>
                Android 待支持
              </Button>
            </div>
          </div>
          <div className="flex items-center justify-between rounded-md border border-border/70 bg-muted/25 px-3 py-2">
            <Label htmlFor="login-now">立即登录并连接</Label>
            <Switch id="login-now" checked={loginNow} onCheckedChange={(v: boolean) => setLoginNow(v)} />
          </div>
          {error && (
            <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}
          <DialogFooter className="flex-col-reverse sm:flex-row">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? "创建中..." : "创建账号"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function CatalogIcon({ itemId, className, fallback }: { itemId: number; className?: string; fallback: ReactNode }) {
  const src = itemIconPath(itemId);
  if (!src) return <div className={cn("flex items-center justify-center", className)}>{fallback}</div>;
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={itemName(itemId)} className={cn("object-contain", className)} loading="lazy" />
  );
}

function StatusBadge({ connected }: { connected: boolean }) {
  if (connected) {
    return (
      <Badge className="w-fit border-primary/20 bg-primary/10 text-primary">
        <Wifi className="size-3" />
        在线
      </Badge>
    );
  }
  return (
    <Badge variant="secondary" className="w-fit">
      <WifiOff className="size-3" />
      离线
    </Badge>
  );
}

function StatusDot({ connected }: { connected: boolean }) {
  return (
    <span className={cn("inline-flex h-4 shrink-0 items-center gap-1 rounded-full px-1.5 text-[10px] leading-none", connected ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground")}>
      <span className={cn("size-1.5 rounded-full", connected ? "bg-primary" : "bg-muted-foreground/60")} />
      {connected ? "在线" : "离线"}
    </span>
  );
}

function ResourceChip({
  icon,
  label,
  value,
  tone,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  tone: "amber" | "cyan" | "green" | "rose" | "slate";
}) {
  const toneClass = {
    amber: "border-amber-400/25 bg-amber-400/10 text-amber-700 dark:text-amber-200",
    cyan: "border-cyan-400/25 bg-cyan-400/10 text-cyan-700 dark:text-cyan-200",
    green: "border-primary/25 bg-primary/10 text-primary",
    rose: "border-rose-400/25 bg-rose-400/10 text-rose-700 dark:text-rose-200",
    slate: "border-border/70 bg-muted/25 text-muted-foreground",
  }[tone];

  return (
    <div className={cn("grid grid-cols-[28px_minmax(0,1fr)] items-center gap-2 rounded-md border px-2 py-2", toneClass)}>
      <span className="flex size-7 items-center justify-center rounded bg-background/60">
        {icon}
      </span>
      <div className="min-w-0">
        <div className="truncate text-[11px] opacity-80">{label}</div>
        <div className="truncate text-sm font-semibold tabular-nums text-foreground">{value}</div>
      </div>
    </div>
  );
}

function inventoryEntries(inventory: Record<string, number> | Record<number, number>): InventoryEntry[] {
  return Object.entries(inventory)
    .map(([rawId, count]) => {
      const id = Number(rawId);
      const item = itemInfo(id);
      return {
        id,
        name: itemName(id),
        count,
        item,
        category: itemCategory(item),
      };
    })
    .filter((entry) => Number.isFinite(entry.id) && entry.count > 0)
    .sort((a, b) => {
      const categoryDiff = inventoryCategoryRank(a.category) - inventoryCategoryRank(b.category);
      if (categoryDiff !== 0) return categoryDiff;
      if (b.count !== a.count) return b.count - a.count;
      return a.id - b.id;
    });
}

function groupInventoryEntries(entries: InventoryEntry[]): InventoryGroup[] {
  const groups = new Map<string, InventoryEntry[]>();
  for (const entry of entries) {
    const group = groups.get(entry.category) ?? [];
    group.push(entry);
    groups.set(entry.category, group);
  }
  return Array.from(groups, ([category, groupEntries]) => ({ category, entries: groupEntries }))
    .sort((a, b) => inventoryCategoryRank(a.category) - inventoryCategoryRank(b.category));
}

function groupPendingTasks(tasks: PendingTaskView[]): { category: string; tasks: PendingTaskView[] }[] {
  const rank = new Map([
    ["主线任务", 0],
    ["日常任务", 1],
    ["居民订单", 2],
    ["顾客订单", 3],
    ["地图事件", 4],
  ]);
  const groups = new Map<string, PendingTaskView[]>();
  for (const task of tasks) {
    const category = task.category || "其他任务";
    const group = groups.get(category) ?? [];
    group.push(task);
    groups.set(category, group);
  }
  for (const category of ["居民订单", "顾客订单"]) {
    if (!groups.has(category)) {
      groups.set(category, []);
    }
  }
  return Array.from(groups, ([category, groupTasks]) => ({
    category,
    tasks: [...groupTasks].sort((a, b) => taskStatusRank(a.status) - taskStatusRank(b.status) || a.id.localeCompare(b.id, "zh-CN", { numeric: true })),
  })).sort((a, b) => (rank.get(a.category) ?? 99) - (rank.get(b.category) ?? 99) || a.category.localeCompare(b.category, "zh-CN"));
}

function emptyTaskGroupText(category: string): string {
  switch (category) {
    case "居民订单":
      return "暂无居民订单";
    case "顾客订单":
      return "暂无顾客订单";
    default:
      return "暂无待办";
  }
}

function taskStatusRank(status: string): number {
  if (status === "missing") return 0;
  if (status === "progress") return 1;
  if (status === "ready") return 2;
  return 3;
}

function inventoryCategoryRank(category: string): number {
  switch (category) {
    case "核心资源":
      return 0;
    case "货币资源":
      return 1;
    case "鲜花":
      return 2;
    case "花朵精华":
      return 3;
    case "培育材料":
      return 4;
    case "可用道具":
      return 5;
    case "其他库存":
      return 6;
    default:
      return 99;
  }
}

function formatCount(value: number): string {
  return new Intl.NumberFormat("zh-CN").format(value);
}

function eventColor(event: Event): string {
  if (event.level === "error") return "text-red-600 dark:text-red-300";
  if (event.level === "warn") return "text-yellow-700 dark:text-yellow-300";
  switch (event.category) {
    case "resource":
      return "text-emerald-600 dark:text-emerald-300";
    case "operation":
      return "text-sky-600 dark:text-sky-300";
    case "land":
      return "text-lime-700 dark:text-lime-300";
    case "reward":
      return "text-cyan-600 dark:text-cyan-300";
    case "cultivation":
      return "text-rose-600 dark:text-rose-300";
    case "order":
      return "text-orange-600 dark:text-orange-300";
    case "task":
      return "text-yellow-700 dark:text-yellow-300";
    case "session":
      return "text-muted-foreground";
    default:
      return legacyKindColor(event.kind);
  }
}

function legacyKindColor(kind: string): string {
  switch (kind) {
    case "waterwheel":
    case "free_water":
      return "text-cyan-600 dark:text-cyan-300";
    case "cultivate_recv":
    case "cultivate_new":
    case "flower_upgrade":
      return "text-rose-600 dark:text-rose-300";
    case "order_finish":
    case "order_customer":
      return "text-orange-600 dark:text-orange-300";
    case "land_unlock":
    case "task_daily":
    case "task_recv":
      return "text-yellow-700 dark:text-yellow-300";
    case "resource_changed":
      return "text-emerald-600 dark:text-emerald-300";
    default:
      return "text-muted-foreground";
  }
}

function eventLabel(kind: string): string {
  switch (kind) {
    case "resource_changed":
      return "资源";
    case "land_changed":
      return "田地";
    case "session":
      return "连接";
    case "session_expired":
      return "过期";
    case "ws_disconnected":
      return "断开";
    default:
      return kind;
  }
}

function DashboardSkeleton() {
  return (
    <div className="flex min-h-full flex-col gap-4 xl:h-full xl:min-h-0 xl:overflow-hidden">
      <div className="h-20 animate-pulse rounded-lg bg-muted/35" />
      <div className="grid gap-4 xl:min-h-0 xl:flex-1 xl:grid-cols-[280px_minmax(0,1fr)] xl:overflow-hidden">
        <div className="min-h-64 animate-pulse rounded-lg bg-muted/30 xl:min-h-0" />
        <div className="min-h-96 animate-pulse rounded-lg bg-muted/25 xl:min-h-0" />
      </div>
    </div>
  );
}
