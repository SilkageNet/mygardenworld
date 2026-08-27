import type { FormEvent } from "react";
import { Loader2, Plus, RefreshCw } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_service_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { alipayLoginStatusLabel } from "@/components/dashboard/dashboard-utils";
import { Field } from "@/components/dashboard/monitor-panels";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { AccountQuota } from "./account-list-panel";

export type AddAccountForm = {
  channel: Channel;
  username: string;
  password: string;
};

export type AlipayQRState = {
  loginId: string;
  content: string;
  status: AlipayLoginStatus;
  error: string;
};

export const EMPTY_ADD_FORM: AddAccountForm = {
  channel: Channel.IOS,
  username: "",
  password: "",
};

export default function AddAccountDialog({
  open,
  form,
  qr,
  quota,
  creating,
  onOpenChange,
  onFormChange,
  onClearQR,
  onSubmit,
  onStartAlipay,
}: {
  open: boolean;
  form: AddAccountForm;
  qr: AlipayQRState | null;
  quota: AccountQuota | null;
  creating: boolean;
  onOpenChange: (open: boolean) => void;
  onFormChange: (form: AddAccountForm) => void;
  onClearQR: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onStartAlipay: () => void;
}) {
  const alipay = form.channel === Channel.ALIPAY;
  const qrActive = Boolean(qr) && qr?.status !== AlipayLoginStatus.EXPIRED && qr?.status !== AlipayLoginStatus.FAILED;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader><DialogTitle>新增账号</DialogTitle></DialogHeader>
        <form className="space-y-4" onSubmit={onSubmit}>
          <Field label="渠道">
            <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="渠道">
              {[
                { channel: Channel.IOS, label: "iOS" },
                { channel: Channel.ALIPAY, label: "支付宝" },
              ].map((option) => (
                <button
                  key={option.channel}
                  type="button"
                  role="radio"
                  aria-checked={form.channel === option.channel}
                  className={cn(
                    "h-10 rounded-md border px-3 text-sm font-medium transition-colors",
                    form.channel === option.channel
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border/70 text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => {
                    onFormChange({ ...form, channel: option.channel });
                    onClearQR();
                  }}
                  disabled={creating}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </Field>
          {alipay ? (
            <div className="rounded-md border border-border/60 bg-white/52 p-4 text-center dark:bg-white/5">
              {qr?.content ? (
                <div className="space-y-3">
                  <div className="mx-auto w-fit rounded-md bg-white p-3 shadow-sm"><QRCodeSVG value={qr.content} size={208} level="M" /></div>
                  <div className="text-sm font-medium">{alipayLoginStatusLabel(qr.status)}</div>
                  {qr.error && <div className="text-xs text-destructive">{qr.error}</div>}
                  {(qr.status === AlipayLoginStatus.EXPIRED || qr.status === AlipayLoginStatus.FAILED) && (
                    <Button type="button" variant="outline" size="sm" onClick={onStartAlipay} disabled={creating}>
                      <RefreshCw className="size-4" />重新获取
                    </Button>
                  )}
                </div>
              ) : (
                <div className="py-6 text-sm text-muted-foreground">点击下方按钮生成二维码，再使用支付宝扫码确认。</div>
              )}
            </div>
          ) : (
            <>
              <Field label="账号">
                <Input value={form.username} onChange={(event) => onFormChange({ ...form, username: event.target.value })} autoComplete="username" disabled={creating} />
              </Field>
              <Field label="密码">
                <Input type="password" value={form.password} onChange={(event) => onFormChange({ ...form, password: event.target.value })} autoComplete="current-password" disabled={creating} />
              </Field>
            </>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={creating}>取消</Button>
            <Button type="submit" disabled={creating || (!alipay && quota?.reached) || (alipay && qrActive)}>
              {creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
              {creating ? "处理中" : alipay ? "获取二维码" : "新增"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
