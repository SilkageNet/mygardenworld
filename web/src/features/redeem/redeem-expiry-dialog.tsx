"use client";

import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import { Clock3, InfinityIcon, LoaderCircle, RotateCcw } from "lucide-react";
import { useEffect, useState } from "react";

import { AdminService, RedeemExpiryOverrideMode } from "@/gen/mygardenworld/v1/admin_pb";
import type { RedeemCode } from "@/gen/mygardenworld/v1/redeem_pb";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { formatAPIError, transport } from "@/lib/api/client";
import { EXPIRY_PRESETS, resolveRedeemExpiry, type CustomUnit, type ExpiryMode } from "./redeem-utils";

const adminClient = createClient(AdminService, transport);

export function RedeemExpiryDialog({ entry, onOpenChange, onSaved }: {
  entry?: RedeemCode;
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void> | void;
}) {
  const [mode, setMode] = useState<ExpiryMode>("custom");
  const [customAmount, setCustomAmount] = useState(15);
  const [customUnit, setCustomUnit] = useState<CustomUnit>("minutes");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!entry) return;
    setError("");
    if (entry.expiresAt) {
      const minutes = Math.max(1, Math.ceil((timestampDate(entry.expiresAt).getTime() - Date.now()) / 60_000));
      setMode("custom");
      setCustomAmount(minutes);
      setCustomUnit("minutes");
    } else {
      // A permanent entry is the common correction case; make a finite
      // 15-minute override the first deliberate choice rather than silently
      // preselecting another permanent value.
      setMode("custom");
      setCustomAmount(15);
      setCustomUnit("minutes");
    }
  }, [entry]);

  async function save() {
    if (!entry) return;
    setSaving(true);
    setError("");
    try {
      const expiry = resolveRedeemExpiry(mode, customAmount, customUnit);
      await adminClient.updateRedeemCodeExpiry({
        fingerprint: entry.fingerprint,
        mode: expiry ? RedeemExpiryOverrideMode.FINITE : RedeemExpiryOverrideMode.PERMANENT,
        expiresAt: expiry ? timestampFromDate(expiry) : undefined,
      });
      await onSaved();
      onOpenChange(false);
    } catch (err) {
      setError(formatAPIError(err, "有效期修正失败"));
    } finally {
      setSaving(false);
    }
  }

  async function restoreSource() {
    if (!entry) return;
    setSaving(true);
    setError("");
    try {
      await adminClient.updateRedeemCodeExpiry({
        fingerprint: entry.fingerprint,
        mode: RedeemExpiryOverrideMode.SOURCE,
      });
      await onSaved();
      onOpenChange(false);
    } catch (err) {
      setError(formatAPIError(err, "恢复数据源有效期失败"));
    } finally {
      setSaving(false);
    }
  }

  const currentExpiry = entry
    ? entry.permanent ? "当前为未知期限" : entry.expiresAt ? `当前至 ${timestampDate(entry.expiresAt).toLocaleString("zh-CN", { hour12: false })}` : "当前期限待确认"
    : "";

  return (
    <Dialog open={Boolean(entry)} onOpenChange={(open) => { if (!saving) onOpenChange(open); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>修正兑换码有效期</DialogTitle>
          <DialogDescription>管理员修正仅改变当前节点采用的期限；兑换码文本与渠道仍保持原始身份。</DialogDescription>
        </DialogHeader>
        {entry && (
          <div className="space-y-4">
            <div className="rounded-md border border-border/60 bg-secondary/35 px-3 py-3">
              <code className="break-all font-mono text-sm font-semibold">{entry.code}</code>
              <div className="mt-1 text-xs text-muted-foreground">{currentExpiry}{entry.expiryOverridden ? " · 已人工校正" : " · 来自数据源"}</div>
            </div>
            <fieldset className="space-y-2.5">
              <legend className="text-sm font-medium">新的有效时间</legend>
              <div className="grid grid-cols-4 gap-2 sm:grid-cols-7">
                {EXPIRY_PRESETS.map((preset) => (
                  <Button key={preset.id} type="button" size="sm" variant={mode === preset.id ? "default" : "outline"} aria-pressed={mode === preset.id} className="px-1.5" onClick={() => setMode(preset.id)}>
                    {preset.label}
                  </Button>
                ))}
                <Button type="button" size="sm" variant={mode === "permanent" ? "default" : "outline"} aria-pressed={mode === "permanent"} className="px-1.5" onClick={() => setMode("permanent")}>
                  <InfinityIcon className="size-3.5" />永久
                </Button>
              </div>
              <button type="button" className="text-sm font-medium text-ring hover:underline" onClick={() => setMode("custom")}>自定义时长</button>
              {mode === "custom" && (
                <div className="grid grid-cols-[minmax(0,1fr)_8rem] gap-2 rounded-md border border-border/60 bg-white/42 p-2 dark:bg-white/5">
                  <Input type="number" min={1} max={525600} value={customAmount} onChange={(event) => setCustomAmount(Number(event.target.value))} aria-label="修正后的有效时长" />
                  <select value={customUnit} onChange={(event) => setCustomUnit(event.target.value as CustomUnit)} className="h-9 rounded-md border border-input/85 bg-white/66 px-3 text-sm outline-none focus:border-ring focus:ring-3 focus:ring-ring/24 dark:bg-input/42" aria-label="有效时间单位">
                    <option value="minutes">分钟</option>
                    <option value="hours">小时</option>
                    <option value="days">天</option>
                  </select>
                </div>
              )}
              <div className="flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
                <Clock3 className="mt-0.5 size-3.5 shrink-0" />保存后，后续数据源同步不会覆盖这次人工修正。
              </div>
            </fieldset>
            {error && <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>}
          </div>
        )}
        <DialogFooter className="min-[380px]:justify-between">
          <div>
            {entry?.expiryOverridden && (
              <Button type="button" variant="ghost" onClick={() => void restoreSource()} disabled={saving}>
                <RotateCcw className="size-4" />恢复数据源期限
              </Button>
            )}
          </div>
          <div className="flex flex-col-reverse gap-2 min-[380px]:flex-row">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>取消</Button>
            <Button type="button" onClick={() => void save()} disabled={!entry || saving}>
              {saving && <LoaderCircle className="size-4 animate-spin" />}
              {saving ? "保存中" : "保存修正"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
