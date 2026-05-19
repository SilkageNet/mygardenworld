import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = typeof window !== "undefined" ? localStorage.getItem("access_token") : null;
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
      const refreshed = await tryRefresh();
      if (refreshed) {
        req.header.set("Authorization", `Bearer ${localStorage.getItem("access_token")}`);
        return await next(req);
      }
      if (typeof window !== "undefined") {
        localStorage.removeItem("access_token");
        localStorage.removeItem("refresh_token");
        window.location.href = "/login";
      }
    }
    throw err;
  }
};

async function tryRefresh(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return false;
  try {
    const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/mygardenworld.v1.AuthService/Refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
    });
    if (!res.ok) return false;
    const data = await res.json();
    localStorage.setItem("access_token", data.accessToken);
    localStorage.setItem("refresh_token", data.refreshToken);
    return true;
  } catch {
    return false;
  }
}

export const transport = createConnectTransport({
  baseUrl: apiBaseUrl(),
  interceptors: [authInterceptor],
});

function apiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL;
  if (configured) return configured;
  if (typeof window !== "undefined") return window.location.origin;
  return "http://127.0.0.1:50051";
}
