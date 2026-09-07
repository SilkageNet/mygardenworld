"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";

export function WebhookHelp({ example }: { example: string }) {
  const [copyStatus, setCopyStatus] = useState("");
  async function copy() {
    try {
      await navigator.clipboard.writeText(example);
      setCopyStatus("已复制 JSON 示例");
    } catch {
      setCopyStatus("无法自动复制，请选中下方示例手动复制");
    }
  }
  return <details className="min-w-0 rounded-lg border bg-muted/20 p-3 text-xs">
    <summary className="cursor-pointer font-medium focus-visible:outline-2 focus-visible:outline-ring">自定义 Webhook 对接说明 · JSON 示例</summary>
    <div className="mt-3 space-y-3 leading-relaxed text-muted-foreground">
      <p>通过 POST 向公网 HTTPS 地址发送 JSON，Content-Type 为 application/json。以下为虚构的异常通知，不包含你的真实账号信息。</p>
      <div className="min-w-0 rounded-md border bg-background">
        <div className="flex items-center justify-between border-b px-3 py-1"><span>请求体示例</span><Button variant="ghost" size="sm" disabled={!example} onClick={copy}>复制 JSON</Button></div>
        <pre tabIndex={0} aria-label="Webhook JSON 请求体示例" className="max-h-64 overflow-auto p-3 text-[11px] text-foreground"><code>{example || "加载中…"}</code></pre>
      </div>
      {copyStatus && <p role="status">{copyStatus}</p>}
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 break-words">
        <dt className="font-mono">id</dt><dd>通知唯一字符串；重试不变，与请求头 X-Notification-ID 相同，用于去重。</dd>
        <dt className="font-mono">kind</dt><dd>account_request 请求保护、session 会话、reputation 礼仪分、pearl_hire 珍珠雇佣、test 测试。</dd>
        <dt className="font-mono">level / message</dt><dd>级别 info、warn 或 error；message 是经过筛选的可读说明。</dd>
        <dt className="font-mono">account_id /<br />account_name</dt><dd>游戏账号的数字 ID 和显示名称；测试通知不含账号字段，名称为空时省略。</dd>
        <dt className="font-mono">ts</dt><dd>事件时间，RFC 3339 字符串，携带时区，可能含小数秒。</dd>
        <dt className="font-mono">recovered</dt><dd>布尔值，true 表示恢复通知；异常和测试为 false。</dd>
        <dt className="font-mono">occurrences /<br />duration_seconds</dt><dd>本次异常累计出现次数、从首次出现到当前事件的秒数，均为整数；首次一般为 1 / 0，测试固定为 1 / 0。</dd>
      </dl>
      <p>接收端应在 10 秒内返回任意 HTTP 2xx，响应体不参与判断。网络失败、408、429、5xx 会退避重试，最多尝试 5 次；其他非 2xx 直接失败，不跟随重定向。通知超过 24 小时不再投递。</p>
      <p>重试可能重复投递，请先按 id 幂等入队再确认。每条新通知的 id 不同（包括同一异常的后续汇总和恢复）；不要把 id 当作异常编号。可按 account_id + kind 关联异常和恢复。</p>
      <p>当前自定义渠道不提供签名或自定义请求头，可使用接收 URL 中的随机令牌校验请求；请勿公开该地址。只发送必要状态，不含游戏凭据、原始响应或完整日志。</p>
    </div>
  </details>;
}
