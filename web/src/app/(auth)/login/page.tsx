"use client";

import { useEffect, useState, type CSSProperties, type FormEvent } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { formatAPIError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ArrowRight, LoaderCircle, LockKeyhole, UserRound } from "lucide-react";

const LOGIN_SKY_STYLE = {
  "--background": "#e9f8ff",
  "--foreground": "#17345f",
  "--card": "#fbfdff",
  "--primary": "#ff6f61",
  "--primary-foreground": "#fffafa",
  "--secondary": "#e7f6ff",
  "--muted": "#eaf6fb",
  "--accent": "#fff1b8",
  "--border": "#bfddec",
  "--input": "#cce5f2",
  "--ring": "#2f87ed",
  "--cloud-shadow": "rgba(80, 130, 190, 0.22)",
  "--cloud-glow": "rgba(255, 255, 255, 0.82)",
  "--toy-shadow": "rgba(45, 103, 165, 0.18)",
} as CSSProperties & Record<string, string>;

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
      setError(formatAPIError(err, "登录失败"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main
      className="fixed left-0 top-0 isolate flex h-dvh w-screen items-center justify-center overflow-y-auto overflow-x-hidden bg-[#e9f8ff] px-4 py-8 text-[#17345f] transition-colors sm:px-6"
      style={LOGIN_SKY_STYLE}
    >
      <span className="sky-decor-plane sky-glide left-[7%] top-20 bg-[linear-gradient(135deg,#ff8e88,#ff6268)] opacity-80 [--plane-duration:18s] [--plane-glide-x:2.4rem] [--plane-glide-y:-1.2rem] [--plane-loop-x:0.7rem] [--plane-loop-y:0.5rem] [--plane-rotate:18deg]" aria-hidden />
      <span className="sky-decor-plane sky-glide right-[8%] top-28 bg-[linear-gradient(135deg,#ffd45d,#ffb331)] opacity-80 [--plane-duration:21s] [--plane-glide-x:-2.2rem] [--plane-glide-y:-1rem] [--plane-loop-x:-0.6rem] [--plane-loop-y:0.4rem] [--plane-rotate:-18deg]" aria-hidden />
      <span className="sky-decor-cloud sky-float left-[12%] top-32 opacity-72" aria-hidden />
      <span className="sky-decor-cloud sky-float right-[12%] bottom-24 opacity-70" aria-hidden />
      <span className="sky-decor-star left-[30%] top-28 opacity-75" aria-hidden />
      <span className="sky-decor-star right-[30%] top-40 bg-[#ff8bb1] opacity-70" aria-hidden />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(90deg,rgba(240,250,255,0.78)_0%,rgba(240,250,255,0.42)_40%,rgba(240,250,255,0.74)_100%)]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_38%,rgba(255,255,255,0.56),rgba(255,255,255,0)_34%),linear-gradient(180deg,rgba(255,255,255,0.18),rgba(174,232,255,0.34))]" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-[#f7fbff]/80 to-transparent" />

      <section className="cloud-surface toy-shadow relative w-full max-w-[25rem] rounded-lg border border-white/70 bg-white/72 px-5 py-7 shadow-[0_24px_80px_rgba(45,103,165,0.18)] backdrop-blur-xl transition-colors sm:px-8 sm:py-8">
        <div className="mx-auto mb-5 flex w-full flex-col items-center text-center">
          <div className="relative size-16 overflow-hidden sm:size-20">
            <Image
              src="/brand/cloud-logo.svg"
              alt="小云朵"
              fill
              priority
              unoptimized
              sizes="5rem"
              className="object-contain drop-shadow-[0_8px_18px_rgba(46,137,199,0.22)]"
            />
          </div>
          <h1 className="mt-2 text-2xl font-semibold leading-tight tracking-normal text-[#17345f]">
            小云朵
          </h1>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username" className="sr-only">
              账号
            </Label>
            <div className="relative">
              <UserRound className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-[#2f87ed]" />
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
                className="h-12 rounded-lg border-[#b8dff2]/90 !bg-white/84 pl-12 pr-4 !text-[#17345f] placeholder:!text-[#657b96] focus-visible:border-[#2f87ed] focus-visible:ring-[#2f87ed]/24"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="password" className="sr-only">
              密码
            </Label>
            <div className="relative">
              <LockKeyhole className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-[#2f87ed]" />
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
                className="h-12 rounded-lg border-[#b8dff2]/90 !bg-white/84 pl-12 pr-4 !text-[#17345f] placeholder:!text-[#657b96] focus-visible:border-[#2f87ed] focus-visible:ring-[#2f87ed]/24"
              />
            </div>
          </div>

          {error && (
            <div
              id="login-error"
              className="rounded-lg border border-[#d56d5d]/35 bg-[#fff3ee]/86 px-3 py-2 text-sm text-[#b64939]"
            >
              {error}
            </div>
          )}

          <Button
            type="submit"
            size="lg"
            className="relative h-12 w-full rounded-lg text-base disabled:opacity-65"
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
