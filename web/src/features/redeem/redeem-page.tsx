"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { createClient } from "@connectrpc/connect";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
  ChevronLeft,
  ChevronRight,
  ClipboardPaste,
  Clock3,
  Cloud,
  Database,
  Globe2,
  InfinityIcon,
  LoaderCircle,
  Pencil,
  RefreshCw,
  Send,
  Settings2,
  ShieldCheck,
  Trash2,
} from "lucide-react";

import { AdminService, RedeemSourceType, type RedeemSource } from "@/gen/mygardenworld/v1/admin_pb";
import { UserRole } from "@/gen/mygardenworld/v1/auth_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  RedeemExchangeService,
  RedeemBrowseFilter,
  RedeemSubmitDisposition,
  RedeemValidation,
  type RedeemCode,
} from "@/gen/mygardenworld/v1/redeem_pb";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ThemeToggle } from "@/components/theme-toggle";
import { formatAPIError, transport } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { cn } from "@/lib/utils";
import {
  DEFAULT_REDEEM_PAGE_SIZE,
  EXPIRY_PRESETS,
  REDEEM_PAGE_SIZES,
  redeemPageCount,
  redeemValidationLabel,
  resolveRedeemExpiry,
  type CustomUnit,
  type ExpiryMode,
} from "./redeem-utils";
import { RedeemExpiryDialog } from "./redeem-expiry-dialog";

const redeemClient = createClient(RedeemExchangeService, transport);
const adminClient = createClient(AdminService, transport);

export default function RedeemPage() {
  const { user, loading: authLoading } = useAuth();
  const [entries, setEntries] = useState<RedeemCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [showExpired, setShowExpired] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(DEFAULT_REDEEM_PAGE_SIZE);
  const [activeCount, setActiveCount] = useState(0);
  const [historicalCount, setHistoricalCount] = useState(0);
  const [editingEntry, setEditingEntry] = useState<RedeemCode>();
  const [code, setCode] = useState("");
  const [channels, setChannels] = useState<Channel[]>([Channel.IOS]);
  const [expiryMode, setExpiryMode] = useState<ExpiryMode>("30m");
  const [customAmount, setCustomAmount] = useState(15);
  const [customUnit, setCustomUnit] = useState<CustomUnit>("minutes");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const loadRequestId = useRef(0);

  const loadEntries = useCallback(async () => {
    const requestId = ++loadRequestId.current;
    setLoading(true);
    setError("");
    try {
      const response = await redeemClient.browseRedeemCodes({
        page,
        pageSize,
        filter: showExpired ? RedeemBrowseFilter.HISTORY : RedeemBrowseFilter.ACTIVE,
      });
      if (requestId !== loadRequestId.current) return;
      const activeTotal = Number(response.activeTotal);
      const historyTotal = Number(response.historyTotal);
      const selectedTotal = showExpired ? historyTotal : activeTotal;
      const lastPage = Math.max(0, Math.ceil(selectedTotal / pageSize) - 1);
      setActiveCount(activeTotal);
      setHistoricalCount(historyTotal);
      if (page > lastPage) {
        setEntries([]);
        setPage(lastPage);
        return;
      }
      setEntries(response.entries);
    } catch (err) {
      if (requestId !== loadRequestId.current) return;
      setError(formatAPIError(err, "兑换码加载失败"));
    } finally {
      if (requestId === loadRequestId.current) setLoading(false);
    }
  }, [page, pageSize, showExpired]);

  useEffect(() => {
    const saved = window.localStorage.getItem("redeem_expiry_mode") as ExpiryMode | null;
    if (saved && (saved === "permanent" || saved === "custom" || EXPIRY_PRESETS.some((item) => item.id === saved))) {
      setExpiryMode(saved);
    }
    void loadEntries();
    const timer = window.setInterval(() => void loadEntries(), 60_000);
    return () => {
      loadRequestId.current += 1;
      window.clearInterval(timer);
    };
  }, [loadEntries]);

  const selectedTotal = showExpired ? historicalCount : activeCount;
  const totalPages = redeemPageCount(selectedTotal, pageSize);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    setError("");
    const normalized = code.trim();
    if (!normalized) {
      setError("请输入兑换码");
      return;
    }
    if (channels.length === 0) {
      setError("请至少选择一个渠道");
      return;
    }
    let expiresAt: Date | null;
    try {
      expiresAt = resolveRedeemExpiry(expiryMode, customAmount, customUnit);
    } catch (err) {
      setError(err instanceof Error ? err.message : "过期时间不正确");
      return;
    }
    setSubmitting(true);
    try {
      const response = await redeemClient.submitRedeemCodes({
        entries: channels.map((channel) => ({
          code: normalized,
          channel,
          expiresAt: expiresAt ? timestampFromDate(expiresAt) : undefined,
          permanent: expiresAt === null,
          reportedValidation: RedeemValidation.PENDING,
          originInstanceId: "",
        })),
      });
      const rejected = response.results.filter((item) => item.disposition === RedeemSubmitDisposition.REJECTED);
      if (rejected.length > 0) throw new Error(rejected.map((item) => item.message).filter(Boolean).join("；") || "兑换码未被接受");
      const accepted = response.results.filter((item) => item.disposition === RedeemSubmitDisposition.ACCEPTED).length;
      const duplicate = response.results.length - accepted;
      setMessage(accepted > 0
        ? `已录入 ${accepted} 个渠道范围，正在使用现有账号验证${duplicate > 0 ? `；${duplicate} 个已存在` : ""}`
        : "兑换码已存在，验证状态已刷新");
      setCode("");
      window.localStorage.setItem("redeem_expiry_mode", expiryMode);
      setShowExpired(false);
      setPage(0);
      if (showExpired || page !== 0) {
        setLoading(true);
        setEntries([]);
      }
      if (!showExpired && page === 0) await loadEntries();
    } catch (err) {
      setError(formatAPIError(err, "兑换码录入失败"));
    } finally {
      setSubmitting(false);
    }
  }

  async function pasteCode() {
    setMessage("");
    setError("");
    try {
      if (!navigator.clipboard?.readText) throw new Error("clipboard unavailable");
      const pasted = (await navigator.clipboard.readText()).trim();
      if (!pasted) {
        setError("剪贴板中没有可用的兑换码");
        return;
      }
      if ([...pasted].length > 128) {
        setError("剪贴板内容超过 128 个字符，请确认后重试");
        return;
      }
      setCode(pasted);
    } catch {
      setError("无法读取剪贴板，请允许浏览器访问或手动输入");
    }
  }

  return (
    <main className="relative isolate h-dvh overflow-y-auto overflow-x-hidden px-3 py-3 text-foreground sm:px-6 sm:py-5 lg:px-8">
      <div className="pointer-events-none fixed inset-0 -z-10 bg-[radial-gradient(circle_at_15%_5%,rgba(255,244,178,0.35),transparent_28%),radial-gradient(circle_at_85%_15%,rgba(255,255,255,0.5),transparent_30%)]" />
      <div className="mx-auto max-w-6xl space-y-4">
        <header className="cloud-surface toy-shadow flex min-h-14 items-center justify-between gap-3 rounded-lg border border-white/65 bg-card/76 px-3 py-2 backdrop-blur-xl dark:border-white/10 dark:bg-card/84 sm:px-4">
          <Link href="/" className="flex min-w-0 items-center gap-2 rounded-md p-1 hover:bg-white/55 dark:hover:bg-white/8">
            <span className="relative size-9 shrink-0 overflow-hidden">
              <Image src="/brand/cloud-logo.svg" alt="小云朵" fill priority unoptimized sizes="2.25rem" className="object-contain" />
            </span>
            <span className="min-w-0">
              <span className="block truncate text-sm font-semibold">兑换码中心</span>
              <span className="hidden text-xs text-muted-foreground sm:block">公开录入、验证与节点共享</span>
            </span>
          </Link>
          <div className="flex items-center gap-1.5">
            <ThemeToggle />
            <Link href={user ? "/" : "/login"} className={buttonVariants({ variant: "outline", size: "sm" })}>
              <ChevronLeft className="size-3.5" />
              {authLoading ? "主页" : user ? "控制台" : "登录"}
            </Link>
          </div>
        </header>

        <section className="grid gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.35fr)]">
          <Card className="cloud-surface h-fit">
            <CardHeader className="border-b border-border/45 pb-3">
              <CardTitle className="flex items-center gap-2"><Send className="size-4 text-primary" />录入兑换码</CardTitle>
              <p className="text-sm leading-6 text-muted-foreground">提交后立即进入对应渠道账号的验证队列；只有成功或已经兑换的兑换码会继续传播。</p>
            </CardHeader>
            <CardContent>
              <form className="space-y-5" onSubmit={submit}>
                <div className="space-y-2">
                  <Label htmlFor="redeem-code">兑换码</Label>
                  <div className="flex gap-2">
                    <Input id="redeem-code" value={code} onChange={(event) => setCode(event.target.value)} placeholder="粘贴或输入兑换码" maxLength={128} autoComplete="off" className="min-w-0 flex-1" />
                    <Button type="button" variant="outline" className="shrink-0" onClick={() => void pasteCode()} aria-label="从剪贴板粘贴兑换码">
                      <ClipboardPaste className="size-4" />
                      粘贴
                    </Button>
                  </div>
                </div>

                <fieldset className="space-y-2">
                  <legend className="text-sm font-medium">适用渠道</legend>
                  <div className="grid grid-cols-2 gap-2">
                    {[Channel.IOS, Channel.ALIPAY].map((channel) => {
                      const selected = channels.includes(channel);
                      return (
                        <Button key={channel} type="button" variant={selected ? "default" : "outline"} aria-pressed={selected} onClick={() => setChannels((current) => selected ? current.filter((item) => item !== channel) : [...current, channel])}>
                          {channel === Channel.IOS ? "iOS" : "Alipay"}
                        </Button>
                      );
                    })}
                  </div>
                </fieldset>

                <fieldset className="space-y-2.5">
                  <legend className="text-sm font-medium">有效时间</legend>
                  <div className="grid grid-cols-4 gap-2 sm:grid-cols-7 lg:grid-cols-4 xl:grid-cols-7">
                    {EXPIRY_PRESETS.map((preset) => (
                      <Button key={preset.id} type="button" size="sm" variant={expiryMode === preset.id ? "default" : "outline"} aria-pressed={expiryMode === preset.id} className="px-1.5" onClick={() => setExpiryMode(preset.id)}>
                        {preset.label}
                      </Button>
                    ))}
                    <Button type="button" size="sm" variant={expiryMode === "permanent" ? "default" : "outline"} aria-pressed={expiryMode === "permanent"} className="px-1.5" onClick={() => setExpiryMode("permanent")}>
                      <InfinityIcon className="size-3.5" />永久
                    </Button>
                  </div>
                  <button type="button" className="text-sm font-medium text-ring hover:underline" onClick={() => setExpiryMode("custom")}>自定义时长</button>
                  {expiryMode === "custom" && (
                    <div className="grid grid-cols-[minmax(0,1fr)_8rem] gap-2 rounded-md border border-border/60 bg-white/42 p-2 dark:bg-white/5">
                      <Input type="number" min={1} max={525600} value={customAmount} onChange={(event) => setCustomAmount(Number(event.target.value))} aria-label="自定义有效时长" />
                      <select value={customUnit} onChange={(event) => setCustomUnit(event.target.value as CustomUnit)} className="h-9 rounded-md border border-input/85 bg-white/66 px-3 text-sm outline-none focus:border-ring focus:ring-3 focus:ring-ring/24 dark:bg-input/42" aria-label="有效时间单位">
                        <option value="minutes">分钟</option>
                        <option value="hours">小时</option>
                        <option value="days">天</option>
                      </select>
                    </div>
                  )}
                  <ExpiryPreview mode={expiryMode} customAmount={customAmount} customUnit={customUnit} />
                </fieldset>

                {error && <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>}
                {message && <div className="rounded-md border border-emerald-400/35 bg-emerald-50/75 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-200">{message}</div>}
                <Button type="submit" className="w-full" disabled={submitting}>
                  {submitting ? <LoaderCircle className="size-4 animate-spin" /> : <Send className="size-4" />}
                  {submitting ? "正在录入" : "录入并验证"}
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card className="cloud-surface min-h-[20rem] lg:min-h-[32rem]">
            <CardHeader className="border-b border-border/45 pb-3">
              <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2.5">
                <div className="flex items-center gap-2">
                  <span className="flex size-8 items-center justify-center rounded-md bg-sky-100 text-sky-600 dark:bg-sky-400/12 dark:text-sky-300"><Globe2 className="size-4" /></span>
                  <div><CardTitle>社区兑换码</CardTitle><p className="mt-0.5 text-xs text-muted-foreground">按最新收录排序</p></div>
                </div>
                <div className="flex w-full items-center gap-1.5 sm:w-auto">
                  <div className="grid min-w-0 flex-1 grid-cols-2 rounded-md bg-secondary/55 p-0.5 sm:flex-none" role="group" aria-label="兑换码状态筛选">
                    <Button type="button" size="sm" variant={showExpired ? "ghost" : "secondary"} className="min-w-0 shadow-none" aria-pressed={!showExpired} onClick={() => {
                      if (!showExpired) { void loadEntries(); return; }
                      setLoading(true); setEntries([]); setShowExpired(false); setPage(0);
                    }}>有效 <span className="tabular-nums text-muted-foreground">{activeCount}</span></Button>
                    <Button type="button" size="sm" variant={showExpired ? "secondary" : "ghost"} className="min-w-0 shadow-none" aria-pressed={showExpired} onClick={() => {
                      if (showExpired) { void loadEntries(); return; }
                      setLoading(true); setEntries([]); setShowExpired(true); setPage(0);
                    }}>历史 <span className="tabular-nums text-muted-foreground">{historicalCount}</span></Button>
                  </div>
                  <Button type="button" size="icon-sm" variant="ghost" onClick={() => void loadEntries()} disabled={loading} aria-label="刷新兑换码"><RefreshCw className={cn("size-4", loading && "animate-spin")} /></Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              {loading && entries.length === 0 ? (
                <EmptyState icon={<LoaderCircle className="size-5 animate-spin" />} title="正在同步兑换码" description="读取当前节点公开注册表" />
              ) : entries.length === 0 ? (
                <EmptyState icon={<Cloud className="size-5" />} title={showExpired ? "暂无历史兑换码" : "暂无有效兑换码"} description="提交后会在这里展示验证进度" />
              ) : (
                <div className="divide-y divide-border/45">
                  {entries.map((entry) => <RedeemCodeRow key={entry.fingerprint} entry={entry} onEdit={user?.role === UserRole.ADMIN ? () => setEditingEntry(entry) : undefined} />)}
                </div>
              )}
              {selectedTotal > 0 && (
                <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border/45 bg-secondary/20 px-3 py-2 text-xs text-muted-foreground">
                  <div className="flex items-center gap-2">
                    <span className="tabular-nums">共 {selectedTotal} 条</span>
                    <span className="h-3.5 w-px bg-border" aria-hidden="true" />
                    <span>每页</span>
                    <div className="flex rounded-md bg-background/65 p-0.5" role="group" aria-label="每页兑换码条数">
                      {REDEEM_PAGE_SIZES.map((size) => (
                        <Button
                          key={size}
                          type="button"
                          size="xs"
                          variant={pageSize === size ? "secondary" : "ghost"}
                          className="h-6 min-w-7 px-1.5 shadow-none"
                          aria-pressed={pageSize === size}
                          onClick={() => {
                            if (pageSize === size) return;
                            setLoading(true); setEntries([]); setPageSize(size); setPage(0);
                          }}
                        >
                          {size}
                        </Button>
                      ))}
                    </div>
                  </div>
                  <nav className="flex items-center gap-1" aria-label="兑换码分页">
                    <Button type="button" size="sm" variant="ghost" className="h-7 px-1.5" onClick={() => { setLoading(true); setEntries([]); setPage((current) => Math.max(0, current - 1)); }} disabled={loading || page === 0} aria-label="上一页"><ChevronLeft className="size-3.5" /><span className="hidden min-[420px]:inline">上一页</span></Button>
                    <span className="min-w-14 text-center tabular-nums text-foreground/80">{page + 1} / {totalPages}</span>
                    <Button type="button" size="sm" variant="ghost" className="h-7 px-1.5" onClick={() => { setLoading(true); setEntries([]); setPage((current) => Math.min(totalPages - 1, current + 1)); }} disabled={loading || page + 1 >= totalPages} aria-label="下一页"><span className="hidden min-[420px]:inline">下一页</span><ChevronRight className="size-3.5" /></Button>
                  </nav>
                </div>
              )}
            </CardContent>
          </Card>
        </section>

        {user?.role === UserRole.ADMIN && <SourceManager />}
        <RedeemExpiryDialog entry={editingEntry} onOpenChange={(open) => { if (!open) setEditingEntry(undefined); }} onSaved={loadEntries} />
      </div>
    </main>
  );
}

function ExpiryPreview({ mode, customAmount, customUnit }: { mode: ExpiryMode; customAmount: number; customUnit: CustomUnit }) {
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    setNow(new Date());
  }, [mode, customAmount, customUnit]);

  if (!now) {
    return <div className="flex items-start gap-1.5 text-xs leading-5 text-muted-foreground"><Clock3 className="mt-0.5 size-3.5 shrink-0" />按所选时长计算过期时间</div>;
  }
  let text = "";
  try {
    const expiry = resolveRedeemExpiry(mode, customAmount, customUnit, now);
    text = expiry ? `预计于 ${expiry.toLocaleString("zh-CN", { hour12: false })} 过期` : "不会按时间自动过期，仍会接受游戏返回的失效结果";
  } catch {
    text = "请输入大于零的有效时长";
  }
  return <div className="flex items-start gap-1.5 text-xs leading-5 text-muted-foreground"><Clock3 className="mt-0.5 size-3.5 shrink-0" />{text}</div>;
}

function RedeemCodeRow({ entry, onEdit }: { entry: RedeemCode; onEdit?: () => void }) {
  const expired = isRedeemExpired(entry);
  const communityVerified = Boolean(entry.communityVerifiedAt);
  const label = redeemValidationLabel(entry.validation, expired, communityVerified);
  const positive = entry.validation === RedeemValidation.SUCCESS || entry.validation === RedeemValidation.ALREADY_REDEEMED;
  const destructive = entry.validation === RedeemValidation.INVALID || expired;
  const expiryText = expired
    ? entry.validation === RedeemValidation.EXPIRED ? "游戏已判定过期" : "已过期"
    : entry.permanent ? "未知期限" : entry.expiresAt ? formatRemaining(timestampDate(entry.expiresAt)) : "待确认";
  const collectedAt = entry.firstSeenAt ? formatCompactTime(timestampDate(entry.firstSeenAt)) : "";
  return (
    <article className="flex min-w-0 items-start gap-2 px-3 py-2.5 sm:px-4">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-1.5">
          <code className="mr-0.5 break-all font-mono text-[0.82rem] font-semibold leading-5 text-foreground">{entry.code}</code>
          <Badge variant="outline" className="px-1.5 font-medium text-muted-foreground">{entry.channel === Channel.IOS ? "iOS" : "Alipay"}</Badge>
          <Badge variant={destructive ? "destructive" : positive ? "secondary" : "outline"}>{label}</Badge>
          {entry.expiryOverridden && <Badge variant="outline" className="font-medium">人工期限</Badge>}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[0.72rem] leading-4 text-muted-foreground">
          <span className="inline-flex items-center gap-1">{positive ? <ShieldCheck className="size-3 text-emerald-500" /> : <Clock3 className="size-3" />}{expiryText}</span>
          {collectedAt && <span>收录于 {collectedAt}</span>}
        </div>
        {entry.lastMessage && <p className="mt-1 truncate text-xs leading-4 text-muted-foreground/85" title={entry.lastMessage}>{entry.lastMessage}</p>}
      </div>
      {onEdit && (
        <Button type="button" size="icon-sm" variant="ghost" className="-mr-1 shrink-0" onClick={onEdit} aria-label={`修正兑换码 ${entry.code} 的有效期`} title="修正有效期">
          <Pencil className="size-3.5" />
        </Button>
      )}
    </article>
  );
}

function EmptyState({ icon, title, description }: { icon: ReactNode; title: string; description: string }) {
  return <div className="flex min-h-56 flex-col items-center justify-center px-6 text-center lg:min-h-72"><span className="mb-3 flex size-11 items-center justify-center rounded-full bg-secondary text-ring">{icon}</span><div className="font-medium">{title}</div><div className="mt-1 text-sm text-muted-foreground">{description}</div></div>;
}

function isRedeemExpired(entry: RedeemCode): boolean {
  return entry.validation === RedeemValidation.EXPIRED || Boolean(entry.expiresAt && timestampDate(entry.expiresAt).getTime() <= Date.now());
}

function formatRemaining(expiry: Date): string {
  const milliseconds = expiry.getTime() - Date.now();
  if (milliseconds <= 0) return "已过期";
  const minutes = Math.ceil(milliseconds / 60_000);
  if (minutes < 60) return `${minutes}分钟后过期`;
  const hours = Math.ceil(minutes / 60);
  if (hours < 48) return `${hours}小时后过期`;
  return `${Math.ceil(hours / 24)}天后过期`;
}

function formatCompactTime(value: Date): string {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(value);
}

type SourceForm = {
  id: bigint;
  name: string;
  type: RedeemSourceType;
  baseUrl: string;
  channel: Channel;
  parserConfigJson: string;
  enabled: boolean;
  pushEnabled: boolean;
  pollIntervalSeconds: number;
};

const EMPTY_SOURCE: SourceForm = {
  id: BigInt(0),
  name: "",
  type: RedeemSourceType.MYGARDENWORLD,
  baseUrl: "",
  channel: Channel.IOS,
  parserConfigJson: JSON.stringify({ type: "json_array", code_field: "code", permanent: true }, null, 2),
  enabled: true,
  pushEnabled: true,
  pollIntervalSeconds: 300,
};

function SourceStatistics({ source }: { source: RedeemSource }) {
  return (
    <div className="min-w-0 space-y-1.5">
      <div className="flex flex-wrap gap-1.5">
        <Badge variant="outline">收录 {source.observedCount.toString()}</Badge>
        <Badge variant="secondary">可采信 {source.trustedCount.toString()}</Badge>
        <Badge variant={source.invalidCount > BigInt(0) ? "destructive" : "outline"}>
          无效 {source.invalidCount.toString()}
        </Badge>
      </div>
      <div className="flex flex-wrap gap-x-3 gap-y-1 text-[11px] leading-5 text-muted-foreground">
        <span>成功 {source.successCount.toString()}</span>
        <span>已兑换 {source.alreadyRedeemedCount.toString()}</span>
        <span>已过期 {source.expiredCount.toString()}</span>
        <span>待验证 {source.pendingCount.toString()}</span>
      </div>
    </div>
  );
}

function SourceManager() {
  const [sources, setSources] = useState<RedeemSource[]>([]);
  const [form, setForm] = useState<SourceForm>(EMPTY_SOURCE);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const response = await adminClient.listRedeemSources({});
      setSources(response.sources);
    } catch (err) {
      setError(formatAPIError(err, "数据源加载失败"));
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy("save");
    setError("");
    try {
      await adminClient.upsertRedeemSource(form);
      setForm(EMPTY_SOURCE);
      setOpen(false);
      await load();
    } catch (err) {
      setError(formatAPIError(err, "数据源保存失败"));
    } finally {
      setBusy("");
    }
  }

  function edit(source: RedeemSource) {
    setForm({
      id: source.id, name: source.name, type: source.type, baseUrl: source.baseUrl,
      channel: source.channel || Channel.IOS, parserConfigJson: source.parserConfigJson,
      enabled: source.enabled, pushEnabled: source.pushEnabled,
      pollIntervalSeconds: source.pollIntervalSeconds,
    });
    setOpen(true);
  }

  return (
    <Card className="cloud-surface">
      <CardHeader className="border-b border-border/45 pb-3">
        <div className="flex items-center justify-between gap-3">
          <div><CardTitle className="flex items-center gap-2"><Database className="size-4 text-ring" />数据源管理</CardTitle><p className="mt-1 text-sm text-muted-foreground">MyGardenWorld 节点的每条兑换码自带渠道；自定义网页来源只读取，并统一归入所选渠道。</p></div>
          <Button type="button" size="sm" variant={open ? "secondary" : "outline"} onClick={() => { setOpen((value) => !value); setForm(EMPTY_SOURCE); }}><Settings2 className="size-4" />{open ? "收起" : "添加"}</Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>}
        {open && (
          <form onSubmit={save} className="grid gap-3 rounded-md border border-border/60 bg-white/42 p-3 dark:bg-white/5 md:grid-cols-2">
            <div className="space-y-1.5"><Label htmlFor="source-name">名称</Label><Input id="source-name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required /></div>
            <div className="space-y-1.5"><Label htmlFor="source-url">地址</Label><Input id="source-url" type="url" value={form.baseUrl} onChange={(event) => setForm({ ...form, baseUrl: event.target.value })} placeholder={form.type === RedeemSourceType.MYGARDENWORLD ? "https://gardend.example.com" : "https://example.com/codes.json"} required />{form.type === RedeemSourceType.MYGARDENWORLD && <p className="text-xs leading-5 text-muted-foreground">填写对方部署的站点根地址，无需附加 Connect 接口路径。</p>}</div>
            <div className="space-y-1.5"><Label htmlFor="source-type">类型</Label><select id="source-type" value={form.type} onChange={(event) => { const type = Number(event.target.value) as RedeemSourceType; setForm({ ...form, type, pushEnabled: type === RedeemSourceType.MYGARDENWORLD }); }} className="h-9 w-full rounded-md border border-input/85 bg-white/66 px-3 text-sm dark:bg-input/42"><option value={RedeemSourceType.MYGARDENWORLD}>MyGardenWorld 节点</option><option value={RedeemSourceType.CUSTOM_HTTP}>自定义网页来源</option></select></div>
            <div className="space-y-1.5"><Label htmlFor="source-interval">拉取间隔（秒）</Label><Input id="source-interval" type="number" min={60} value={form.pollIntervalSeconds} onChange={(event) => setForm({ ...form, pollIntervalSeconds: Number(event.target.value) })} /></div>
            {form.type === RedeemSourceType.CUSTOM_HTTP && <><div className="space-y-1.5"><Label htmlFor="source-channel">兑换渠道</Label><select id="source-channel" value={form.channel} onChange={(event) => setForm({ ...form, channel: Number(event.target.value) as Channel })} className="h-9 w-full rounded-md border border-input/85 bg-white/66 px-3 text-sm dark:bg-input/42"><option value={Channel.IOS}>iOS</option><option value={Channel.ALIPAY}>Alipay</option></select><p className="text-xs leading-5 text-muted-foreground">该来源解析出的所有兑换码都归入此渠道。</p></div><div className="space-y-1.5 md:col-span-2"><Label htmlFor="source-parser">解析配置</Label><textarea id="source-parser" value={form.parserConfigJson} onChange={(event) => setForm({ ...form, parserConfigJson: event.target.value })} rows={6} className="w-full rounded-md border border-input/85 bg-white/66 px-3 py-2 font-mono text-xs outline-none focus:border-ring focus:ring-3 focus:ring-ring/24 dark:bg-input/42" /><p className="text-xs leading-5 text-muted-foreground">必须明确一种期限规则：<code>permanent</code>、<code>expires_field</code> 或 <code>default_ttl_seconds</code>。来源未提供真实过期时间时建议使用 <code>permanent: true</code>，由游戏结果判定失效。</p></div></>}
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} />启用拉取</label>
            {form.type === RedeemSourceType.MYGARDENWORLD && <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.pushEnabled} onChange={(event) => setForm({ ...form, pushEnabled: event.target.checked })} />验证后回传</label>}
            <div className="flex justify-end gap-2 md:col-span-2"><Button type="button" variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button type="submit" disabled={busy === "save"}>{busy === "save" && <LoaderCircle className="size-4 animate-spin" />}保存数据源</Button></div>
          </form>
        )}
        {sources.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">尚未配置数据源，本节点仍可公开录入和验证。</div>
        ) : (
          <div className="grid gap-2 md:grid-cols-2">
            {sources.map((source) => (
              <div key={source.id.toString()} className="rounded-md border border-border/60 bg-white/42 p-3 dark:bg-white/5">
                <div className="flex items-start justify-between gap-2">
                  <button type="button" className="min-w-0 text-left" onClick={() => edit(source)}>
                    <div className="truncate text-sm font-semibold">{source.name}</div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">{source.baseUrl}</div>
                  </button>
                  <div className="flex shrink-0 gap-1">
                    <Badge variant={source.enabled ? "secondary" : "outline"}>{source.type === RedeemSourceType.MYGARDENWORLD ? "节点" : "网页"}</Badge>
                    <Badge variant="outline">{source.type === RedeemSourceType.MYGARDENWORLD ? "按兑换码" : source.channel === Channel.ALIPAY ? "Alipay" : "iOS"}</Badge>
                  </div>
                </div>
                <div className="mt-3 flex items-end justify-between gap-2">
                  <SourceStatistics source={source} />
                  <div className="flex shrink-0 gap-1">
                    <Button type="button" variant="ghost" size="icon-xs" aria-label="立即同步" disabled={busy === `sync:${source.id}`} onClick={async () => { setBusy(`sync:${source.id}`); try { await adminClient.syncRedeemSource({ id: source.id }); await load(); } catch (err) { setError(formatAPIError(err, "同步失败")); } finally { setBusy(""); } }}>{busy === `sync:${source.id}` ? <LoaderCircle className="animate-spin" /> : <RefreshCw />}</Button>
                    <Button type="button" variant="destructive" size="icon-xs" aria-label="删除数据源" onClick={async () => { if (!window.confirm(`删除数据源“${source.name}”？`)) return; try { await adminClient.deleteRedeemSource({ id: source.id }); await load(); } catch (err) { setError(formatAPIError(err, "删除失败")); } }}><Trash2 /></Button>
                  </div>
                </div>
                {source.lastError && <div className="mt-2 rounded bg-destructive/8 px-2 py-1 text-xs text-destructive">{source.lastError}</div>}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
