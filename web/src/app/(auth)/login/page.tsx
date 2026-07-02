"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowRight, KeyRound, LogIn } from "lucide-react";

const PRODUCT_NAME = "花序";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
      router.push("/");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-background px-4 py-8 text-foreground sm:px-6">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,color-mix(in_oklab,var(--primary)_8%,transparent),transparent_32%),linear-gradient(90deg,color-mix(in_oklab,var(--accent)_18%,transparent),transparent_46%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-primary/35" />
      <div className="absolute right-4 top-4 z-10 sm:right-6 sm:top-6">
        <ThemeToggle />
      </div>

      <div className="relative w-full max-w-[25rem]">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 flex size-11 items-center justify-center rounded-md border border-primary/25 bg-primary/10 text-primary">
            <KeyRound className="size-5" />
          </div>
          <h1 className="text-2xl font-semibold">{PRODUCT_NAME}</h1>
          <p className="mt-1 text-sm text-muted-foreground">本地花园自动化</p>
        </div>

        <Card className="border-border/80 bg-card/88 shadow-xl shadow-black/10 backdrop-blur dark:shadow-black/30">
          <CardHeader className="px-5 pt-5">
            <CardTitle className="text-base">登录</CardTitle>
          </CardHeader>
          <CardContent className="px-5 pb-5">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">用户名</Label>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="admin"
                  autoComplete="username"
                  autoFocus
                  required
                  className="h-11 bg-background/70"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">密码</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="输入密码"
                  autoComplete="current-password"
                  required
                  className="h-11 bg-background/70"
                />
              </div>

              {error && (
                <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </div>
              )}

              <Button type="submit" size="lg" className="w-full justify-between" disabled={loading}>
                <span className="inline-flex items-center gap-2">
                  <LogIn className="size-4" />
                  {loading ? "登录中..." : "登录"}
                </span>
                <ArrowRight className="size-4 opacity-70" />
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
