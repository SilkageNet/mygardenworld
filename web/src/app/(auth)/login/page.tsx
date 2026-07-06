"use client";

import { useEffect, useState, type FormEvent } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ArrowRight, LoaderCircle, LockKeyhole, UserRound } from "lucide-react";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const root = document.documentElement;
    const previousScrollbarGutter = root.style.scrollbarGutter;
    root.style.scrollbarGutter = "auto";
    return () => {
      root.style.scrollbarGutter = previousScrollbarGutter;
    };
  }, []);

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
    <main
      className="fixed left-0 top-0 flex h-dvh w-screen items-center justify-center overflow-y-auto overflow-x-hidden bg-[#edf5ea] px-4 py-8 text-[#1e3d28] transition-colors dark:bg-[#07110d] dark:text-[#edf6eb] sm:px-6"
      style={{
        backgroundImage: "url('/brand/huaxu-login-background.png')",
        backgroundPosition: "center",
        backgroundSize: "cover",
      }}
    >
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(90deg,rgba(247,252,245,0.94)_0%,rgba(247,252,245,0.72)_40%,rgba(247,252,245,0.92)_100%)] dark:bg-[linear-gradient(90deg,rgba(4,12,8,0.92)_0%,rgba(8,18,12,0.76)_44%,rgba(4,12,8,0.9)_100%)]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_38%,rgba(255,255,255,0.52),rgba(255,255,255,0)_34%),linear-gradient(180deg,rgba(255,255,255,0.18),rgba(210,232,209,0.32))] dark:bg-[radial-gradient(circle_at_50%_38%,rgba(116,215,122,0.12),rgba(116,215,122,0)_36%),linear-gradient(180deg,rgba(255,255,255,0.04),rgba(3,10,6,0.48))]" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-[#dfeee0]/70 to-transparent dark:from-[#07110d]/90" />

      <section className="relative w-full max-w-[27rem] rounded-lg border border-white/70 bg-white/62 px-5 py-7 shadow-[0_24px_80px_rgba(39,81,43,0.18)] backdrop-blur-xl transition-colors dark:border-[#2c3f2c]/85 dark:bg-[#101910]/78 dark:shadow-[0_24px_80px_rgba(0,0,0,0.42)] sm:px-8 sm:py-9">
        <div className="mx-auto mb-7 flex w-full justify-center">
          <div className="relative size-20 overflow-hidden sm:size-24">
            <Image
              src="/brand/cloud-logo.svg"
              alt="小云朵"
              fill
              priority
              unoptimized
              sizes="6rem"
              className="object-contain drop-shadow-[0_8px_18px_rgba(46,137,199,0.22)]"
            />
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username" className="sr-only">
              账号
            </Label>
            <div className="relative">
              <UserRound className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-[#377e45] dark:text-[#74d77a]" />
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="账号"
                autoComplete="username"
                autoFocus
                required
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "login-error" : undefined}
                className="h-12 rounded-lg border-[#a9caa8]/85 !bg-white/82 pl-12 pr-4 !text-[#1e3d28] shadow-[inset_0_1px_0_rgba(255,255,255,0.9)] placeholder:!text-[#6f856d] focus-visible:border-[#3b8f4d] focus-visible:ring-[#69ad71]/30 dark:border-[#345338]/85 dark:!bg-[#0c150e]/86 dark:!text-[#edf6eb] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] dark:placeholder:!text-[#91a28f] dark:focus-visible:border-[#74d77a] dark:focus-visible:ring-[#74d77a]/25"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="password" className="sr-only">
              密码
            </Label>
            <div className="relative">
              <LockKeyhole className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-[#377e45] dark:text-[#74d77a]" />
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="密码"
                autoComplete="current-password"
                required
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "login-error" : undefined}
                className="h-12 rounded-lg border-[#a9caa8]/85 !bg-white/82 pl-12 pr-4 !text-[#1e3d28] shadow-[inset_0_1px_0_rgba(255,255,255,0.9)] placeholder:!text-[#6f856d] focus-visible:border-[#3b8f4d] focus-visible:ring-[#69ad71]/30 dark:border-[#345338]/85 dark:!bg-[#0c150e]/86 dark:!text-[#edf6eb] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] dark:placeholder:!text-[#91a28f] dark:focus-visible:border-[#74d77a] dark:focus-visible:ring-[#74d77a]/25"
              />
            </div>
          </div>

          {error && (
            <div
              id="login-error"
              className="rounded-lg border border-[#d56d5d]/35 bg-[#fff3ee]/86 px-3 py-2 text-sm text-[#b64939] dark:border-[#ff756a]/35 dark:bg-[#2a1411]/86 dark:text-[#ffb8ad]"
            >
              {error}
            </div>
          )}

          <Button
            type="submit"
            size="lg"
            className="relative h-12 w-full rounded-lg bg-[#5e9f5d] text-base font-semibold text-white shadow-[0_14px_28px_rgba(64,128,64,0.26)] hover:bg-[#4f914f] focus-visible:ring-[#69ad71]/35 disabled:opacity-65"
            disabled={loading}
          >
            <span className="inline-flex items-center gap-2">
              {loading && <LoaderCircle className="size-4 animate-spin" />}
              {loading ? "登录中..." : "登录"}
            </span>
            {!loading && <ArrowRight className="absolute right-4 size-4 opacity-80" />}
          </Button>
        </form>
      </section>
    </main>
  );
}
