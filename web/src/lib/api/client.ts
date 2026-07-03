import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";

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
