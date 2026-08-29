"use client";

import { useEffect, useState, type FormEvent } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { formatAPIError } from "@/lib/api/client";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowRight, LoaderCircle, LockKeyhole, Ticket, UserRound } from "lucide-react";

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
      className="theme-sky-background fixed left-0 top-0 isolate flex h-dvh w-screen items-center justify-center overflow-y-auto overflow-x-hidden px-4 py-8 text-foreground transition-colors sm:px-6"
    >
      <div className="absolute right-4 top-4 z-20 rounded-md border border-white/55 bg-card/60 p-1 shadow-sm backdrop-blur-xl dark:border-white/10 dark:bg-card/72">
        <ThemeToggle />
      </div>
      <span className="sky-decor-plane sky-glide left-[7%] top-20 bg-[linear-gradient(135deg,#ff8e88,#ff6268)] opacity-80 [--plane-duration:18s] [--plane-glide-x:2.4rem] [--plane-glide-y:-1.2rem] [--plane-loop-x:0.7rem] [--plane-loop-y:0.5rem] [--plane-rotate:18deg]" aria-hidden />
      <span className="sky-decor-plane sky-glide right-[8%] top-28 bg-[linear-gradient(135deg,#ffd45d,#ffb331)] opacity-80 [--plane-duration:21s] [--plane-glide-x:-2.2rem] [--plane-glide-y:-1rem] [--plane-loop-x:-0.6rem] [--plane-loop-y:0.4rem] [--plane-rotate:-18deg]" aria-hidden />
      <span className="sky-decor-cloud sky-float left-[12%] top-32 opacity-72" aria-hidden />
      <span className="sky-decor-cloud sky-float right-[12%] bottom-24 opacity-70" aria-hidden />
      <span className="sky-decor-star left-[30%] top-28 opacity-75" aria-hidden />
      <span className="sky-decor-star right-[30%] top-40 bg-[#ff8bb1] opacity-70" aria-hidden />
      <div className="login-sky-wash pointer-events-none absolute inset-0" />
      <div className="login-sky-glow pointer-events-none absolute inset-0" />
      <div className="login-sky-horizon pointer-events-none absolute inset-x-0 bottom-0 h-32" />

      <section className="cloud-surface toy-shadow relative w-full max-w-[25rem] rounded-lg border border-white/70 bg-card/78 px-5 py-7 backdrop-blur-xl transition-colors dark:border-white/10 dark:bg-card/90 sm:px-8 sm:py-8">
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
          <h1 className="mt-2 text-2xl font-semibold leading-tight tracking-normal text-foreground">
            小云朵
          </h1>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username" className="sr-only">
              账号
            </Label>
            <div className="relative">
              <UserRound className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-ring" />
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
                className="h-12 rounded-lg pl-12 pr-4"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="password" className="sr-only">
              密码
            </Label>
            <div className="relative">
              <LockKeyhole className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-ring" />
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
                className="h-12 rounded-lg pl-12 pr-4"
              />
            </div>
          </div>

          {error && (
            <div
              id="login-error"
              className="rounded-lg border border-destructive/35 bg-destructive/10 px-3 py-2 text-sm text-destructive"
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
        <Link href="/redeem" className={buttonVariants({ variant: "ghost", className: "mt-3 w-full" })}>
          <Ticket className="size-4" />
          查看与录入兑换码
        </Link>
      </section>
    </main>
  );
}
