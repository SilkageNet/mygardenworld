"use client";

import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from "react";
import { createClient } from "@connectrpc/connect";
import { AuthService } from "@/gen/mygardenworld/v1/auth_pb";
import type { User } from "@/gen/mygardenworld/v1/auth_pb";
import { clearClientAuthState, getAccessToken, refreshAccessToken, setAccessToken, transport } from "@/lib/api/client";

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

const authClient = createClient(AuthService, transport);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!mounted) return;

    let active = true;
    async function bootstrapSession() {
      try {
        if (!getAccessToken()) {
          const refreshed = await refreshAccessToken();
          if (!refreshed) {
            if (active) setUser(null);
            return;
          }
        }
        const res = await authClient.getMe({});
        if (active) setUser(res.user ?? null);
      } catch {
        clearClientAuthState();
        if (active) setUser(null);
      } finally {
        if (active) setLoading(false);
      }
    }

    void bootstrapSession();
    return () => {
      active = false;
    };
  }, [mounted]);

  const login = useCallback(async (username: string, password: string) => {
    const res = await authClient.login({ username, password });
    setAccessToken(res.accessToken);
    setUser(res.user ?? null);
  }, []);

  const logout = useCallback(() => {
    authClient.logout({}).catch(() => {});
    clearClientAuthState();
    setUser(null);
  }, []);

  if (!mounted) {
    return (
      <AuthContext.Provider value={{ user: null, loading: true, login, logout }}>
        {children}
      </AuthContext.Provider>
    );
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
