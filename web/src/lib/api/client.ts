import { createConnectTransport } from "@connectrpc/connect-web";
import { Code, ConnectError, type Interceptor } from "@connectrpc/connect";

let accessToken: string | null = null;
let refreshInFlight: Promise<boolean> | null = null;

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getAccessToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  try {
    return await next(req);
  } catch (err: unknown) {
    if (
      err &&
      typeof err === "object" &&
      "code" in err &&
      (err as { code: number }).code === 16
    ) {
      if (isPublicAuthRequest(req.url)) {
        throw err;
      }
      const refreshed = await refreshAccessToken();
      if (refreshed) {
        const nextToken = getAccessToken();
        if (nextToken) {
          req.header.set("Authorization", `Bearer ${nextToken}`);
        }
        return await next(req);
      }
      if (typeof window !== "undefined") {
        clearClientAuthState();
        window.location.href = "/login";
      }
    }
    throw err;
  }
};

export function getAccessToken(): string | null {
  return accessToken;
}

export function setAccessToken(token: string) {
  accessToken = token || null;
  clearLegacyStoredTokens();
}

export function clearClientAuthState() {
  accessToken = null;
  clearLegacyStoredTokens();
}

export async function refreshAccessToken(): Promise<boolean> {
  if (refreshInFlight) {
    return refreshInFlight;
  }
  refreshInFlight = doRefreshAccessToken().finally(() => {
    refreshInFlight = null;
  });
  return refreshInFlight;
}

async function doRefreshAccessToken(): Promise<boolean> {
  try {
    const legacyRefreshToken = getLegacyStoredToken("refresh_token");
    if (await requestAccessTokenRefresh(legacyRefreshToken)) {
      return true;
    }
    await wait(150);
    if (await requestAccessTokenRefresh(null)) {
      return true;
    }
    clearClientAuthState();
    return false;
  } catch {
    clearClientAuthState();
    return false;
  }
}

async function requestAccessTokenRefresh(legacyRefreshToken: string | null): Promise<boolean> {
  const res = await fetchWithCredentials(`${apiBaseUrl()}/mygardenworld.v1.AuthService/Refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(legacyRefreshToken ? { refreshToken: legacyRefreshToken } : {}),
  });
  if (!res.ok) {
    return false;
  }
  const data = await res.json();
  if (!data.accessToken) {
    return false;
  }
  setAccessToken(data.accessToken);
  return true;
}

async function wait(ms: number): Promise<void> {
  await new Promise((resolve) => window.setTimeout(resolve, ms));
}

const fetchWithCredentials: typeof fetch = (input, init) => {
  return fetch(input, { ...init, credentials: "include" });
};

export const transport = createConnectTransport({
  baseUrl: apiBaseUrl(),
  interceptors: [authInterceptor],
  fetch: fetchWithCredentials,
});

export function formatAPIError(err: unknown, fallback = "操作失败"): string {
  if (err instanceof ConnectError) {
    const raw = err.rawMessage.trim();
    if (isNetworkFetchError(err, raw)) {
      return `无法连接到后端服务（${apiBaseUrl()}）。请确认 gardend 已启动。`;
    }
    return translateConnectError(err.code, raw || fallback);
  }
  if (err instanceof TypeError && isFetchFailureMessage(err.message)) {
    return `无法连接到后端服务（${apiBaseUrl()}）。请确认 gardend 已启动。`;
  }
  if (err instanceof Error) {
    return translatePlainError(err.message, fallback);
  }
  const message = String(err ?? "").trim();
  return message ? translatePlainError(message, fallback) : fallback;
}

function apiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL;
  if (configured) return configured;
  if (typeof window !== "undefined") {
    const url = new URL(window.location.origin);
    const localDevHost = url.hostname === "localhost" || url.hostname === "127.0.0.1" || url.hostname === "::1";
    if (localDevHost && /^30\d\d$/.test(url.port)) {
      url.protocol = "http:";
      url.port = "50051";
      return url.origin;
    }
    return window.location.origin;
  }
  return "http://127.0.0.1:50051";
}

function isPublicAuthRequest(url: string): boolean {
  return url.endsWith("/mygardenworld.v1.AuthService/Login") ||
    url.endsWith("/mygardenworld.v1.AuthService/Refresh");
}

function isNetworkFetchError(err: ConnectError, raw: string): boolean {
  return (err.code === Code.Unknown || err.code === Code.Unavailable) && isFetchFailureMessage(raw);
}

function isFetchFailureMessage(message: string): boolean {
  return /failed to fetch|networkerror|load failed/i.test(message);
}

function translateConnectError(code: Code, raw: string): string {
  const message = translateKnownBackendMessage(raw);
  if (message) return message;
  switch (code) {
    case Code.Unauthenticated:
      return raw || "登录已过期，请重新登录。";
    case Code.PermissionDenied:
      return raw || "没有权限执行此操作。";
    case Code.InvalidArgument:
      return raw || "请求参数不正确。";
    case Code.NotFound:
      return raw || "请求的资源不存在。";
    case Code.AlreadyExists:
      return raw || "资源已存在。";
    case Code.ResourceExhausted:
      return raw || "请求过于频繁，请稍后再试。";
    case Code.FailedPrecondition:
      return raw || "当前状态不允许执行此操作。";
    case Code.Unavailable:
      return raw || "后端服务暂时不可用，请稍后再试。";
    case Code.DeadlineExceeded:
      return raw || "请求超时，请稍后再试。";
    case Code.Internal:
      return raw || "后端服务内部错误。";
    default:
      return raw || "请求失败。";
  }
}

function translatePlainError(message: string, fallback: string): string {
  const raw = stripConnectCodePrefix(message.trim());
  return translateKnownBackendMessage(raw) || raw || fallback;
}

function stripConnectCodePrefix(message: string): string {
  return message.replace(/^\[[^\]]+\]\s*/, "");
}

function translateKnownBackendMessage(message: string): string {
  switch (message) {
    case "username/password required":
      return "请输入账号和密码。";
    case "invalid credentials":
      return "账号或密码不正确。";
    case "account disabled":
      return "账号已被禁用。";
    case "refresh_token required":
    case "invalid or expired refresh token":
    case "not authenticated":
    case "token expired":
    case "token invalid":
      return "登录已过期，请重新登录。";
    case "runner not started":
      return "账号尚未登录或运行器未启动。";
    default:
      return message;
  }
}

function getLegacyStoredToken(key: string): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function clearLegacyStoredTokens() {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
  } catch {
    // ignore
  }
}
