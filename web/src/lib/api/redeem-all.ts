import { apiBaseUrl, getAccessToken } from "@/lib/api/client";

export type RedeemResultRow = {
  stackLabel: string;
  accountName: string;
  ok: boolean;
  message: string;
};

export type RedeemAllOutcome = {
  results: RedeemResultRow[];
  successCount: number;
  failureCount: number;
};

/** Peer stack URLs / labels / admin creds injected by start-gardend.ps1. */
export function peerStackConfig(): {
  localApiUrl: string;
  localStackLabel: string;
  peerApiUrls: string[];
  peerStackLabels: string[];
  stackAdminUsername: string;
  stackAdminPassword: string;
  hasPeers: boolean;
} {
  const peerApiUrls = splitCSV(process.env.NEXT_PUBLIC_PEER_API_URLS);
  const peerStackLabels = splitCSV(process.env.NEXT_PUBLIC_PEER_STACK_LABELS);
  const stackAdminUsername = (process.env.NEXT_PUBLIC_STACK_ADMIN_USERNAME || "").trim();
  const stackAdminPassword = process.env.NEXT_PUBLIC_STACK_ADMIN_PASSWORD || "";
  return {
    localApiUrl: apiBaseUrl(),
    localStackLabel: (process.env.NEXT_PUBLIC_STACK_LABEL || "").trim() || "当前实例",
    peerApiUrls,
    peerStackLabels,
    stackAdminUsername,
    stackAdminPassword,
    hasPeers: peerApiUrls.length > 0 && Boolean(stackAdminUsername) && Boolean(stackAdminPassword),
  };
}

function splitCSV(raw: string | undefined): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

type RedeemCodeResponse = {
  results?: Array<{
    accountId?: string;
    accountName?: string;
    ok?: boolean;
    message?: string;
  }>;
  successCount?: number;
  failureCount?: number;
};

export async function redeemCodeAllStacks(params: {
  code: string;
  localApiUrl: string;
  localStackLabel: string;
  peerApiUrls: string[];
  peerStackLabels: string[];
  stackAdminUsername: string;
  stackAdminPassword: string;
}): Promise<RedeemAllOutcome> {
  const merged: RedeemAllOutcome = {
    results: [],
    successCount: 0,
    failureCount: 0,
  };

  const localToken = getAccessToken();
  if (localToken) {
    const local = await redeemOnAPI({
      apiUrl: params.localApiUrl,
      accessToken: localToken,
      code: params.code,
      stackLabel: params.localStackLabel,
    });
    mergeOutcome(merged, local);
  }

  for (let i = 0; i < params.peerApiUrls.length; i++) {
    const apiUrl = params.peerApiUrls[i];
    const stackLabel = params.peerStackLabels[i] || apiUrl;
    const token = await loginAPI(apiUrl, params.stackAdminUsername, params.stackAdminPassword);
    const peer = await redeemOnAPI({
      apiUrl,
      accessToken: token,
      code: params.code,
      stackLabel,
    });
    mergeOutcome(merged, peer);
  }

  if (merged.results.length === 0) {
    throw new Error("没有可兑换的实例或账号");
  }
  return merged;
}

function mergeOutcome(target: RedeemAllOutcome, source: RedeemAllOutcome) {
  target.results.push(...source.results);
  target.successCount += source.successCount;
  target.failureCount += source.failureCount;
}

function connectionError(apiUrl: string): Error {
  return new Error(`无法连接到后端服务（${apiUrl}）。请确认 gardend 已启动。`);
}

async function loginAPI(apiUrl: string, username: string, password: string): Promise<string> {
  let res: Response;
  try {
    res = await fetch(`${trimSlash(apiUrl)}/mygardenworld.v1.AuthService/Login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ username, password }),
    });
  } catch {
    throw connectionError(apiUrl);
  }
  const raw = await res.text();
  if (!res.ok) {
    throw new Error(`登录 ${apiUrl} 失败: HTTP ${res.status} ${raw}`);
  }
  const data = JSON.parse(raw) as { accessToken?: string };
  if (!data.accessToken) {
    throw new Error(`登录 ${apiUrl} 失败: 缺少 accessToken`);
  }
  return data.accessToken;
}

async function redeemOnAPI(params: {
  apiUrl: string;
  accessToken: string;
  code: string;
  stackLabel: string;
}): Promise<RedeemAllOutcome> {
  let res: Response;
  try {
    res = await fetch(`${trimSlash(params.apiUrl)}/mygardenworld.v1.AccountService/RedeemCode`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${params.accessToken}`,
      },
      credentials: "include",
      body: JSON.stringify({ code: params.code, accountIds: [] }),
    });
  } catch {
    throw connectionError(params.apiUrl);
  }
  const raw = await res.text();
  if (!res.ok) {
    throw new Error(`[${params.stackLabel}] 兑换失败: HTTP ${res.status} ${raw}`);
  }
  const data = JSON.parse(raw) as RedeemCodeResponse;
  const outcome: RedeemAllOutcome = {
    results: [],
    successCount: data.successCount ?? 0,
    failureCount: data.failureCount ?? 0,
  };
  for (const item of data.results ?? []) {
    outcome.results.push({
      stackLabel: params.stackLabel,
      accountName: item.accountName || item.accountId || "未知账号",
      ok: Boolean(item.ok),
      message: item.message || (item.ok ? "ok" : "失败"),
    });
  }
  return outcome;
}

function trimSlash(url: string): string {
  return url.replace(/\/+$/, "");
}
