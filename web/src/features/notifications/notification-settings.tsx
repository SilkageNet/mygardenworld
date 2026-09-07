"use client";

import { useEffect, useRef, useState } from "react";
import { createClient } from "@connectrpc/connect";
import { Bell } from "lucide-react";
import { NotificationService, type UserNotificationsView } from "@/gen/mygardenworld/v1/notification_pb";
import { transport, formatAPIError } from "@/lib/api/client";
import { WorkspaceClient } from "@/lib/api/workspace-client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { notificationSettingsUpdate } from "./settings-model";

const commands = createClient(NotificationService, transport);
const statusLabels: Record<string, string> = { pending: "等待发送", sending: "发送中", sent: "已送达", failed: "发送失败", cancelled: "已取消" };

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
      await commands.saveNotificationSettings(notificationSettingsUpdate({ enabled, cooldown, endpoint, clearEndpoint }));
      setEndpoint(""); setClearEndpoint(false);
      showPage([BigInt(0)]);
      setNotice("已保存。启用或更换地址后仅处理新事件；更换地址、关闭通知会取消原有待发送记录。");
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

  const dirty = !!view?.settings && (enabled !== view.settings.enabled || cooldown !== String(view.settings.cooldownMinutes) || !!endpoint.trim() || clearEndpoint);
  const pageReady = view?.beforeId === pages.at(-1);

  return <div className="space-y-4 text-sm">
    <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/25 p-3">
      <div><label htmlFor="notifications-enabled" className="font-medium">Webhook 通知</label><p className="mt-1 text-xs text-muted-foreground">一个接收地址，覆盖你的全部游戏账号</p></div>
      <Switch id="notifications-enabled" checked={enabled} onCheckedChange={(value) => { setEnabled(value); if (value) setClearEndpoint(false); }} disabled={!view || busy} />
    </div>
    <div className="space-y-2">
      <label htmlFor="notification-endpoint" className="font-medium">接收地址</label>
      <Input id="notification-endpoint" type="password" autoComplete="off" placeholder={view?.settings?.hasEndpoint && !clearEndpoint ? "已保存加密地址，留空保持不变" : "https://example.com/webhook"} value={endpoint} onChange={(e) => { setEndpoint(e.target.value); setClearEndpoint(false); }} disabled={!view || busy} aria-describedby="notification-endpoint-help" />
      <p id="notification-endpoint-help" className="text-xs leading-relaxed text-muted-foreground">仅支持公网 HTTPS，发送通用 JSON。不会发送游戏凭据、原始响应或完整日志。企微、钉钉等专用格式需由接收端转换。</p>
      {view?.settings?.hasEndpoint && <Button variant="ghost" size="sm" disabled={busy} onClick={() => { setClearEndpoint(true); setEndpoint(""); setEnabled(false); }}>{clearEndpoint ? "保存后清除地址并关闭通知" : "清除已保存地址"}</Button>}
    </div>
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
