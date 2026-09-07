"use client";

import { useEffect, useRef, useState } from "react";
import { createClient } from "@connectrpc/connect";
import { Bell } from "lucide-react";
import { NotificationProvider, NotificationService, type UserNotificationsView } from "@/gen/mygardenworld/v1/notification_pb";
import { transport, formatAPIError } from "@/lib/api/client";
import { WorkspaceClient } from "@/lib/api/workspace-client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { notificationProviders, notificationSettingsUpdate, supportsSigning } from "./settings-model";
import { WebhookHelp } from "./webhook-help";

const commands = createClient(NotificationService, transport);
const statusLabels: Record<string, string> = { pending: "等待发送", sending: "发送中", sent: "接收端已确认", failed: "发送失败", cancelled: "已取消" };

export function NotificationSettings() {
  const [open, setOpen] = useState(false);
  return <>
    <Button variant="ghost" size="icon-sm" onClick={() => setOpen(true)} aria-label="个人通知设置"><Bell className="size-4" /></Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[88dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>个人通知</DialogTitle>
          <DialogDescription>仅通知你名下的游戏账号。管理员也不会收到其他用户的事件。</DialogDescription>
        </DialogHeader>
        {open && <NotificationForm />}
      </DialogContent>
    </Dialog>
  </>;
}

function NotificationForm() {
  const clientRef = useRef<WorkspaceClient | null>(null);
  const beforeRef = useRef(BigInt(0));
  const initialized = useRef(false);
  const [view, setView] = useState<UserNotificationsView>();
  const [enabled, setEnabled] = useState(false);
  const [endpoint, setEndpoint] = useState("");
  const [clearEndpoint, setClearEndpoint] = useState(false);
  const [provider, setProvider] = useState(NotificationProvider.CUSTOM);
  const [signingSecret, setSigningSecret] = useState("");
  const [clearSigningSecret, setClearSigningSecret] = useState(false);
  const [cooldown, setCooldown] = useState("30");
  const [pages, setPages] = useState<bigint[]>([BigInt(0)]);
  const [busy, setBusy] = useState(false);
  const [connected, setConnected] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const client = new WorkspaceClient({
      onConnectionState: (state) => { if (active) setConnected(state === "open"); },
      onReady: () => client.loadNotifications(beforeRef.current),
      onNotifications: (next) => {
        if (!active || next.beforeId !== beforeRef.current) return;
        setView(next);
        if (!initialized.current && next.settings) {
          initialized.current = true;
          setEnabled(next.settings.enabled);
          setProvider(next.settings.provider);
          setCooldown(String(next.settings.cooldownMinutes));
        }
      },
      onError: (err) => { if (active) setError(err.message); },
    });
    clientRef.current = client;
    client.start();
    const timer = window.setInterval(() => client.loadNotifications(beforeRef.current), 5_000);
    return () => { active = false; window.clearInterval(timer); client.stop(); clientRef.current = null; };
  }, []);

  function showPage(nextPages: bigint[]) {
    beforeRef.current = nextPages.at(-1) ?? BigInt(0);
    setPages(nextPages);
    clientRef.current?.loadNotifications(beforeRef.current);
  }

  async function save() {
    setBusy(true); setError(""); setNotice("");
    try {
      await commands.saveNotificationSettings(notificationSettingsUpdate({ enabled, cooldown, endpoint, clearEndpoint, provider, savedProvider: view!.settings!.provider, signingSecret, clearSigningSecret }));
      setEndpoint(""); setClearEndpoint(false);
      setSigningSecret(""); setClearSigningSecret(false);
      showPage([BigInt(0)]);
      setNotice("已保存。启用或更换渠道、地址、密钥后仅处理新事件；旧的待发送记录会取消。请发送测试确认配置。");
    } catch (err) { setError(formatAPIError(err)); } finally { setBusy(false); }
  }

  async function test() {
    setBusy(true); setError(""); setNotice("");
    try {
      await commands.testNotification({});
      showPage([BigInt(0)]);
      setNotice("测试已加入队列，请查看下方投递结果。每分钟可测试一次。");
    } catch (err) { setError(formatAPIError(err)); } finally { setBusy(false); }
  }

  const dirty = !!view?.settings && (enabled !== view.settings.enabled || provider !== view.settings.provider || cooldown !== String(view.settings.cooldownMinutes) || !!endpoint.trim() || clearEndpoint || !!signingSecret.trim() || clearSigningSecret);
  const sameProvider = provider === view?.settings?.provider;
  const keepingSecret = sameProvider && !endpoint.trim() && !clearEndpoint && !clearSigningSecret && view?.settings?.hasSigningSecret;
  const pageReady = view?.beforeId === pages.at(-1);

  return <div className="space-y-4 text-sm">
    <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/25 p-3">
      <div><label htmlFor="notifications-enabled" className="font-medium">启用个人通知</label><p className="mt-1 text-xs text-muted-foreground">一个接收渠道，覆盖你的全部游戏账号</p></div>
      <Switch id="notifications-enabled" checked={enabled} onCheckedChange={(value) => { setEnabled(value); if (value) setClearEndpoint(false); }} disabled={!view || busy} />
    </div>
    <div className="space-y-2">
      <label htmlFor="notification-provider" className="font-medium">推送渠道</label>
      <select id="notification-provider" className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50" disabled={!view || busy} value={provider} onChange={(e) => { setProvider(Number(e.target.value)); setEndpoint(""); setSigningSecret(""); setClearEndpoint(false); setClearSigningSecret(false); }}>
        {notificationProviders.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
      </select>
      {view?.settings && !sameProvider && <p className="text-xs text-muted-foreground">切换渠道需重新填写地址和密钥，不会沿用其他渠道的凭据。</p>}
    </div>
    <div className="space-y-2">
      <label htmlFor="notification-endpoint" className="font-medium">接收地址</label>
      <Input id="notification-endpoint" type="password" autoComplete="off" placeholder={sameProvider && view?.settings?.hasEndpoint && !clearEndpoint ? "已保存加密地址，留空保持不变" : notificationProviders.find((item) => item.value === provider)?.placeholder} value={endpoint} onChange={(e) => { setEndpoint(e.target.value); setClearEndpoint(false); }} disabled={!view || busy} aria-describedby="notification-endpoint-help" />
      <p id="notification-endpoint-help" className="text-xs leading-relaxed text-muted-foreground">{provider === NotificationProvider.CUSTOM ? "接收端需支持下方说明中的通用 JSON 格式。" : "从所选平台的群机器人设置中复制原始 Webhook 地址，系统会自动转换消息格式；同一用户的机器人消息至少间隔 4 秒发送。"} 地址加密保存，不会回显；仅支持公网 HTTPS。</p>
      {view?.settings?.hasEndpoint && <Button variant="ghost" size="sm" disabled={busy} onClick={() => { setClearEndpoint(true); setEndpoint(""); setEnabled(false); }}>{clearEndpoint ? "保存后清除地址并关闭通知" : "清除已保存地址"}</Button>}
    </div>
    {supportsSigning(provider) && <div className="space-y-2">
      <label htmlFor="notification-signing-secret" className="font-medium">加签密钥 <span className="font-normal text-muted-foreground">（按机器人安全设置填写）</span></label>
      <Input id="notification-signing-secret" type="password" autoComplete="off" maxLength={1024} value={signingSecret} disabled={!view || busy || clearEndpoint} placeholder={keepingSecret ? "已加密保存，留空保持不变" : "机器人启用签名校验时必填"} onChange={(e) => { setSigningSecret(e.target.value); setClearSigningSecret(false); }} aria-describedby="notification-signing-help" />
      <p id="notification-signing-help" className="text-xs leading-relaxed text-muted-foreground">更换地址时请重新填写对应密钥，旧密钥会清除。若使用关键词校验，请在机器人中添加关键词“小云朵”；若配置 IP 白名单，请放行部署服务器的出口 IP。</p>
      {sameProvider && view?.settings?.hasSigningSecret && <Button variant="ghost" size="sm" disabled={busy || clearEndpoint} onClick={() => { setClearSigningSecret(true); setSigningSecret(""); }}>{clearSigningSecret ? "保存后清除加签密钥" : "清除已保存密钥"}</Button>}
    </div>}
    {provider === NotificationProvider.CUSTOM && <WebhookHelp example={view?.customPayloadExample ?? ""} />}
    <div className="flex items-center justify-between gap-3">
      <label htmlFor="notification-cooldown">同一账号同类异常冷却</label>
      <div className="flex items-center gap-2"><Input id="notification-cooldown" type="number" min={1} max={1440} step={1} className="w-24" value={cooldown} onChange={(e) => setCooldown(e.target.value)} disabled={!view || busy} /><span className="text-muted-foreground">分钟</span></div>
    </div>
    <p className="rounded-lg bg-muted/35 p-3 text-xs leading-relaxed text-muted-foreground">通知范围：请求保护、会话失效、礼仪分保护和珍珠雇佣锁定。首个异常立即通知，重复异常按冷却汇总；请求恢复、会话重建单独通知。普通操作失败和正常等待不推送。</p>
    {error && <p role="alert" className="text-xs text-destructive">{error}</p>}
    {notice && <p role="status" className="text-xs text-muted-foreground">{notice}</p>}
    <div className="flex items-center justify-between gap-2">
      <Button variant="outline" size="sm" disabled={!connected || busy || !view?.settings?.enabled || dirty} onClick={test}>发送测试</Button>
      <Button size="sm" disabled={!connected || !view || busy || !dirty} onClick={save}>{busy ? "处理中…" : "保存设置"}</Button>
    </div>
    <section className="space-y-2 border-t pt-3" aria-label="通知投递记录">
      <div className="flex items-center justify-between"><h3 className="font-medium">投递记录</h3><span className="text-xs text-muted-foreground">保留 7 天 · 每页 5 条</span></div>
      {!view || !pageReady ? <p className="py-4 text-center text-muted-foreground">加载中…</p> : view.deliveries.length === 0 ? <p className="py-4 text-center text-muted-foreground">暂无通知，可保存设置后发送测试</p> : <ul className="divide-y rounded-lg border px-3">
        {view.deliveries.map((item) => <li key={String(item.id)} className="space-y-1 py-2.5">
          <div className="flex items-start justify-between gap-3"><p className="min-w-0 break-words text-xs">{item.title}</p><span className={`shrink-0 text-xs ${item.status === "failed" ? "text-destructive" : "text-muted-foreground"}`}>{statusLabels[item.status] ?? item.status}</span></div>
          <p className="text-[11px] text-muted-foreground">{new Date(Number(item.createdMs)).toLocaleString()} · 尝试 {item.attempts} 次</p>
          {item.lastError && <p className="text-xs text-muted-foreground">{item.lastError}</p>}
        </li>)}
      </ul>}
      <div className="flex items-center justify-end gap-2">
        {!connected && <span role="status" className="mr-auto text-xs text-muted-foreground">正在连接…</span>}
        <Button variant="outline" size="sm" disabled={pages.length === 1 || !connected || !pageReady} onClick={() => showPage(pages.slice(0, -1))}>上一页</Button>
        <span className="text-xs text-muted-foreground">第 {pages.length} 页</span>
        <Button variant="outline" size="sm" disabled={!view?.hasMore || !connected || !pageReady} onClick={() => showPage([...pages, view!.nextBeforeId])}>下一页</Button>
      </div>
    </section>
  </div>;
}
