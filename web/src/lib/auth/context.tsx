"use client";

import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from "react";
import { createClient } from "@connectrpc/connect";
import { AuthService } from "@/gen/mygardenworld/v1/auth_pb";
import type { User } from "@/gen/mygardenworld/v1/auth_pb";
import { transport } from "@/lib/api/client";

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

const authClient = createClient(AuthService, transport);

function getStoredToken(key: string): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function setStoredToken(key: string, value: string) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(key, value);
  } catch {
    // ignore
  }
}

function removeStoredToken(key: string) {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem(key);
  } catch {
    // ignore
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!mounted) return;

    const token = getStoredToken("access_token");
    if (!token) {
      setLoading(false);
      return;
    }

    authClient.getMe({}).then((res) => {
      setUser(res.user ?? null);
    }).catch(() => {
      removeStoredToken("access_token");
      removeStoredToken("refresh_token");
    }).finally(() => setLoading(false));
  }, [mounted]);

  const login = useCallback(async (username: string, password: string) => {
    const res = await authClient.login({ username, password });
    setStoredToken("access_token", res.accessToken);
    setStoredToken("refresh_token", res.refreshToken);
    setUser(res.user ?? null);
  }, []);

  const logout = useCallback(() => {
    const refreshToken = getStoredToken("refresh_token");
    if (refreshToken) {
      authClient.logout({ refreshToken }).catch(() => {});
    }
    removeStoredToken("access_token");
    removeStoredToken("refresh_token");
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
