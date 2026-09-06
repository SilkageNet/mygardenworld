"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import { exportPolicyJSON, importPolicyJSON } from "./policy-json";

export default function PolicyJSONDialog({ policy, disabled, onImport }: {
  policy: Policy;
  disabled: boolean;
  onImport: (policy: Policy) => void;
}) {
  const [mode, setMode] = useState<"import" | "export" | null>(null);
  const [text, setText] = useState("");
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<Policy | null>(null);
  function open(next: "import" | "export") {
    setMode(next);
    setText(next === "export" ? exportPolicyJSON(policy) : "");
    setMessage("");
    setPreview(null);
  }
  return (
    <>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" disabled={disabled} onClick={() => open("export")}>导出 JSON</Button>
        <Button variant="outline" size="sm" disabled={disabled} onClick={() => open("import")}>导入 JSON</Button>
      </div>
      <Dialog open={mode !== null} onOpenChange={(value) => { if (!value) setMode(null); }}>
        <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl">
          <DialogHeader><DialogTitle>{mode === "export" ? "导出账号配置" : "导入账号配置"}</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground">包含所有模块的完整配置，不包含账号密码或登录凭据。导入会替换全部模块设置，账号运行／暂停状态保持不变。</p>
          {mode === "export" && <p className="text-xs text-muted-foreground">导出当前编辑器中的配置，包含尚未保存的修改。</p>}
          <label htmlFor="policy-json" className="text-sm font-medium">配置 JSON</label>
          <textarea id="policy-json" className="dark-scrollbar h-64 w-full resize-y rounded-md border border-input bg-background p-3 font-mono text-xs focus-visible:outline-2 focus-visible:outline-ring" spellCheck={false} readOnly={mode === "export"} value={text}
            onChange={(event) => { setText(event.target.value); setPreview(null); setMessage(""); }} />
          {message && <p role="status" className="text-sm break-words">{message}</p>}
          {preview && <div className="space-y-1 rounded-md border p-3 text-sm">
            <p>完整配置校验通过，应用到编辑器后点击“保存”生效。</p>
            <p>自动挤号：{preview.basic?.displacedSessionReloginEnabled ? "开启" : "关闭"}；竞赛自动升级：{preview.union?.race?.upgradeTask ? "开启" : "关闭"}；单次元宝上限：{String(preview.union?.race?.maxSpendDiamond ?? 0)}</p>
          </div>}
          <DialogFooter>
            <Button variant="outline" onClick={() => setMode(null)}>取消</Button>
            {mode === "export" ? <Button onClick={async () => {
              try { await navigator.clipboard.writeText(text); setMessage("已复制完整配置 JSON"); }
              catch { setMessage("无法自动复制，请在文本框中全选并复制"); }
            }}>复制 JSON</Button> : <Button disabled={!text.trim() || disabled} onClick={() => {
              if (preview) { onImport(preview); setMode(null); return; }
              try { setPreview(importPolicyJSON(text, policy)); setMessage(""); }
              catch (error) { setMessage(error instanceof Error ? error.message : "JSON 配置无效"); }
            }}>{preview ? "应用到编辑器" : "校验配置"}</Button>}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
