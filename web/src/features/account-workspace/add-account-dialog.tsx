import type { FormEvent } from "react";
import { Check, Loader2, Plus, RefreshCw } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { alipayLoginStatusLabel } from "@/components/dashboard/dashboard-utils";
import { Field } from "@/features/workspace/shared/workspace-ui";
import { Button } from "@/components/ui/button";
import { ContentReveal } from "@/components/effects/content-reveal";
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
}) {
  const alipay = form.channel === Channel.ALIPAY;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[min(600px,calc(100dvh-2rem))] max-h-[600px] flex-col overflow-hidden">
        <DialogHeader><DialogTitle>新增账号</DialogTitle></DialogHeader>
        <form className="flex min-h-0 flex-1 flex-col gap-4" onSubmit={onSubmit}>
          <LoginProgress alipay={alipay} qr={qr} creating={creating} />
          <Field label="渠道">
            <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="渠道">
              {[
                { channel: Channel.IOS, label: "iOS" },
                { channel: Channel.ALIPAY, label: "Alipay" },
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
            <ContentReveal key="alipay" className="flex min-h-0 flex-1 items-center justify-center rounded-md border border-border/60 bg-white/52 p-4 text-center dark:bg-white/5">
              {qr?.content ? (
                <div className="space-y-2">
                  <div className="mx-auto w-fit rounded-md bg-white p-2.5 shadow-sm"><QRCodeSVG value={qr.content} size={176} level="M" /></div>
                  <div className="text-sm font-medium">{alipayLoginStatusLabel(qr.status)}</div>
                  {qr.error && <div className="text-xs text-destructive">{qr.error}</div>}
                  {(qr.status === AlipayLoginStatus.EXPIRED || qr.status === AlipayLoginStatus.FAILED) && (
                    <Button type="submit" variant="outline" size="sm" disabled={creating}>
                      {creating ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}刷新二维码
                    </Button>
                  )}
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="text-sm text-muted-foreground">使用 Alipay 扫码后将自动获取游戏账号。</div>
                  <Button type="submit" disabled={creating}>
                    {creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
                    {creating ? "获取中" : "获取二维码"}
                  </Button>
                </div>
              )}
            </ContentReveal>
          ) : (
            <ContentReveal key="ios" className="min-h-0 flex-1 space-y-4">
              <Field label="账号">
                <Input value={form.username} onChange={(event) => onFormChange({ ...form, username: event.target.value })} autoComplete="username" disabled={creating} />
              </Field>
              <Field label="密码">
                <Input type="password" value={form.password} onChange={(event) => onFormChange({ ...form, password: event.target.value })} autoComplete="current-password" disabled={creating} />
              </Field>
            </ContentReveal>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={creating}>取消</Button>
            {!alipay && (
              <Button type="submit" disabled={creating || quota?.reached}>
                {creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
                {creating ? "处理中" : "新增"}
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function LoginProgress({ alipay, qr, creating }: { alipay: boolean; qr: AlipayQRState | null; creating: boolean }) {
  const failed = alipay && (qr?.status === AlipayLoginStatus.EXPIRED || qr?.status === AlipayLoginStatus.FAILED);
  const connecting = creating || (alipay && (qr?.status === AlipayLoginStatus.PROCESSING || qr?.status === AlipayLoginStatus.COMPLETE));
  const activeStep = connecting ? 3 : 2;
  const steps = ["选择渠道", alipay ? "扫码授权" : "账号验证", "连接账号"];

  return (
    <ol className="grid grid-cols-3 gap-1 rounded-md border border-border/55 bg-white/38 p-1.5 dark:bg-white/4" aria-label="新增账号进度">
      {steps.map((label, index) => {
        const step = index + 1;
        const complete = step < activeStep;
        const current = step === activeStep;
        const error = failed && step === 2;
        return (
          <li
            key={label}
            aria-current={current ? "step" : undefined}
            className={cn(
              "flex min-w-0 items-center justify-center gap-1.5 rounded px-1.5 py-1.5 text-[11px] font-medium transition-colors sm:text-xs",
              current && "bg-white/82 text-foreground shadow-sm dark:bg-white/9",
              complete && "text-primary",
              !complete && !current && "text-muted-foreground/70",
              error && "bg-destructive/8 text-destructive",
            )}
          >
            <span
              className={cn(
                "flex size-5 shrink-0 items-center justify-center rounded-full border text-[10px] tabular-nums",
                complete && "border-primary/35 bg-primary/12 text-primary",
                current && "border-primary bg-primary text-primary-foreground",
                !complete && !current && "border-border/70 bg-background/45",
                error && "border-destructive/45 bg-destructive/10 text-destructive",
              )}
            >
              {complete ? <Check className="size-3" /> : step}
            </span>
            <span className="truncate">{label}</span>
          </li>
        );
      })}
    </ol>
  );
}
