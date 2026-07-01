"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Activity,
  ArrowRight,
  DatabaseZap,
  KeyRound,
  LogIn,
  Radar,
  ShieldCheck,
  Sprout,
} from "lucide-react";
import { itemName } from "@/lib/game/catalog";
import { ThemeToggle } from "@/components/theme-toggle";

const FEATURE_ITEMS = [
  { label: "田地状态", value: "实时同步", icon: Activity },
  { label: "订单任务", value: "自动分流", icon: Radar },
  { label: "本地数据", value: "SQLite", icon: DatabaseZap },
];

const SHOWCASE_FLOWERS = [23001, 23004, 23006, 23007, 23008, 22007];

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
    <main className="login-page relative min-h-screen overflow-hidden bg-[#edf4e9] text-foreground dark:bg-[#09110e]">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(115deg,rgba(70,151,93,0.16),transparent_34%),linear-gradient(180deg,rgba(238,183,84,0.2),transparent_42%),linear-gradient(0deg,rgba(246,249,242,0.44),rgba(246,249,242,0.68))] dark:bg-[linear-gradient(115deg,rgba(39,116,82,0.32),transparent_34%),linear-gradient(180deg,rgba(243,190,95,0.12),transparent_42%),linear-gradient(0deg,rgba(14,19,15,0.2),rgba(14,19,15,0.82))]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_1px_1px,rgba(25,54,31,0.08)_1px,transparent_0)] bg-[length:32px_32px] opacity-55 dark:bg-[radial-gradient(circle_at_1px_1px,rgba(255,255,255,0.08)_1px,transparent_0)] dark:opacity-45" />
      <div className="login-aurora login-aurora-a pointer-events-none absolute" />
      <div className="login-aurora login-aurora-b pointer-events-none absolute" />
      <div className="relative flex min-h-screen items-center justify-center px-4 py-8 sm:px-6 lg:px-8">
        <div className="absolute right-4 top-4 z-10">
          <ThemeToggle />
        </div>

        <div className="login-shell grid w-full max-w-5xl overflow-hidden rounded-lg border border-emerald-950/10 bg-white/72 shadow-2xl shadow-emerald-950/10 backdrop-blur dark:border-white/10 dark:bg-[#0f1712]/88 dark:shadow-black/35 md:grid-cols-[minmax(0,1.04fr)_minmax(380px,0.72fr)]">
          <section className="relative hidden min-h-[620px] overflow-hidden border-r border-emerald-950/10 bg-[#edf6ea] p-8 text-foreground dark:border-white/10 dark:bg-[#0b1510] dark:text-white md:flex md:flex-col md:justify-between">
            <div className="absolute inset-0 bg-[linear-gradient(150deg,rgba(76,164,95,0.2),transparent_42%),linear-gradient(28deg,rgba(227,167,74,0.18),transparent_44%)] dark:bg-[linear-gradient(150deg,rgba(106,211,129,0.18),transparent_42%),linear-gradient(28deg,rgba(227,167,74,0.16),transparent_44%)]" />
            <div className="login-light-ribbon pointer-events-none absolute inset-x-[-18%] top-24 h-24 rotate-[-12deg]" />
            <div className="login-orbit login-orbit-one pointer-events-none absolute rounded-full" />
            <div className="login-orbit login-orbit-two pointer-events-none absolute rounded-full" />
            <div className="absolute inset-x-0 bottom-0 h-44 bg-[linear-gradient(0deg,rgba(228,239,222,0.86),transparent)] dark:bg-[linear-gradient(0deg,rgba(5,10,7,0.78),transparent)]" />
            <div className="relative">
              <div className="flex items-center gap-3">
                <div>
                  <div className="text-base font-semibold">花园世界</div>
                  <div className="text-xs text-muted-foreground dark:text-white/58">本地自动化控制台</div>
                </div>
              </div>

              <div className="mt-12 max-w-md">
                <div className="mb-4 flex items-center gap-2 text-xs font-medium text-primary dark:text-[#95e99e]">
                  <ShieldCheck className="size-4" />
                  Local Control Plane
                </div>
                <h1 className="text-4xl font-semibold leading-tight text-foreground dark:text-white">
                  登录后接管花园的运行节奏
                </h1>
                <p className="mt-4 max-w-sm text-sm leading-6 text-muted-foreground dark:text-white/66">
                  管理账号连接、订单、任务、田地和资源状态，所有自动化策略都从本地服务发起并记录。
                </p>
              </div>
            </div>

            <div className="relative">
              <FlowerShowcase />
              <div className="mt-8 grid grid-cols-3 gap-3">
                {FEATURE_ITEMS.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div key={item.label} className="login-feature-tile rounded-md border border-emerald-950/10 bg-white/40 p-3 dark:border-white/10 dark:bg-black/18">
                      <Icon className="mb-3 size-4 text-amber-600 dark:text-[#f0c875]" />
                      <div className="text-xs text-muted-foreground dark:text-white/52">{item.label}</div>
                      <div className="mt-1 text-sm font-medium text-foreground dark:text-white">{item.value}</div>
                    </div>
                  );
                })}
              </div>
            </div>
          </section>

          <section className="relative flex min-h-[calc(100vh-4rem)] items-center justify-center bg-card/92 p-5 sm:min-h-[620px] sm:p-8">
            <div className="w-full max-w-sm">
              <div className="mb-8 flex items-center gap-3 md:hidden">
                <div>
                  <div className="text-base font-semibold">花园世界</div>
                  <div className="text-xs text-muted-foreground">本地自动化控制台</div>
                </div>
              </div>

              <Card className="login-card border-border/80 bg-background/72 shadow-none ring-border/70 dark:bg-background/58">
                <CardHeader className="gap-2 px-5 pt-5">
                  <div className="mb-2 flex size-10 items-center justify-center rounded-md border border-primary/18 bg-primary/10 text-primary">
                    <KeyRound className="size-5" />
                  </div>
                  <CardTitle className="text-xl">欢迎回来</CardTitle>
                  <CardDescription>登录本地控制面，继续管理你的花园自动化。</CardDescription>
                </CardHeader>
                <CardContent className="px-5 pb-5">
                  <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="space-y-2">
                      <Label htmlFor="username">用户名</Label>
                      <Input
                        id="username"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        placeholder="用户名或邮箱"
                        autoComplete="username"
                        required
                        className="h-10 bg-card/75 dark:bg-card/65"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="password">密码</Label>
                      <Input
                        id="password"
                        type="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        placeholder="请输入密码"
                        autoComplete="current-password"
                        required
                        className="h-10 bg-card/75 dark:bg-card/65"
                      />
                    </div>
                    {error && (
                      <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                        {error}
                      </div>
                    )}
                    <Button type="submit" size="lg" className="login-primary-button w-full justify-between" disabled={loading}>
                      <span className="inline-flex items-center gap-2">
                        <LogIn className="size-4" />
                        {loading ? "登录中..." : "登录"}
                      </span>
                      <ArrowRight className="size-4 opacity-70" />
                    </Button>
                  </form>

                  <div className="mt-5 flex items-center gap-2 rounded-md border border-border/70 bg-muted/35 px-3 py-2 text-xs text-muted-foreground">
                    <Sprout className="size-4 shrink-0 text-primary" />
                    <span className="min-w-0">凭据仅用于访问本地守护进程，不在浏览器里持久保存。</span>
                  </div>
                </CardContent>
              </Card>
            </div>
          </section>
        </div>
      </div>
      <LoginMotionStyles />
    </main>
  );
}

function FlowerShowcase() {
  return (
    <div className="flex max-w-md flex-wrap gap-2">
      {SHOWCASE_FLOWERS.map((id) => {
        const name = itemName(id);
        return (
          <div
            key={id}
            className="login-flower-tile rounded-md border border-emerald-950/10 bg-white/55 px-2.5 py-1.5 shadow-sm shadow-emerald-950/10 dark:border-white/10 dark:bg-white/[0.055] dark:shadow-black/20"
            title={`${name} #${id}`}
          >
            <span className="text-xs font-medium text-primary dark:text-[#95e99e]">{name}</span>
            <span className="ml-1.5 text-[10px] text-muted-foreground dark:text-white/45">#{id}</span>
          </div>
        );
      })}
    </div>
  );
}

function LoginMotionStyles() {
  return (
    <style jsx global>{`
      .login-page {
        isolation: isolate;
      }

      .login-aurora {
        width: 36rem;
        height: 36rem;
        border-radius: 9999px;
        filter: blur(54px);
        opacity: 0.2;
        mix-blend-mode: multiply;
      }

      .dark .login-aurora {
        opacity: 0.28;
        mix-blend-mode: screen;
      }

      .login-aurora-a {
        left: -11rem;
        top: -10rem;
        background: radial-gradient(circle, rgba(68, 162, 91, 0.64), rgba(68, 162, 91, 0) 67%);
        animation: loginAuroraA 13s ease-in-out infinite;
      }

      .dark .login-aurora-a {
        background: radial-gradient(circle, rgba(108, 232, 145, 0.72), rgba(108, 232, 145, 0) 67%);
      }

      .login-aurora-b {
        right: -13rem;
        bottom: -15rem;
        background: radial-gradient(circle, rgba(224, 164, 66, 0.48), rgba(224, 164, 66, 0) 69%);
        animation: loginAuroraB 15s ease-in-out infinite;
      }

      .dark .login-aurora-b {
        background: radial-gradient(circle, rgba(239, 183, 87, 0.52), rgba(239, 183, 87, 0) 69%);
      }

      .login-shell {
        position: relative;
      }

      .login-shell::before {
        content: "";
        position: absolute;
        inset: 0;
        border-radius: inherit;
        padding: 1px;
        background: linear-gradient(120deg, rgba(54, 139, 76, 0.34), transparent 27%, transparent 70%, rgba(211, 143, 42, 0.24));
        mask:
          linear-gradient(#000 0 0) content-box,
          linear-gradient(#000 0 0);
        mask-composite: exclude;
        pointer-events: none;
        animation: loginBorderGlow 6s ease-in-out infinite;
      }

      .dark .login-shell::before {
        background: linear-gradient(120deg, rgba(137, 239, 162, 0.44), transparent 27%, transparent 70%, rgba(255, 211, 124, 0.34));
      }

      .login-light-ribbon {
        background: linear-gradient(90deg, transparent, rgba(58, 151, 78, 0.14), rgba(220, 152, 52, 0.18), transparent);
        filter: blur(18px);
        animation: loginRibbon 8s ease-in-out infinite;
      }

      .dark .login-light-ribbon {
        background: linear-gradient(90deg, transparent, rgba(139, 245, 166, 0.2), rgba(255, 216, 131, 0.22), transparent);
      }

      .login-orbit {
        border: 1px solid rgba(42, 112, 58, 0.14);
        box-shadow: inset 0 0 26px rgba(65, 160, 87, 0.06);
      }

      .dark .login-orbit {
        border-color: rgba(157, 255, 181, 0.14);
        box-shadow: inset 0 0 26px rgba(115, 235, 144, 0.06);
      }

      .login-orbit-one {
        width: 18rem;
        height: 18rem;
        right: -7rem;
        top: 7rem;
        animation: loginOrbit 18s linear infinite;
      }

      .login-orbit-two {
        width: 12rem;
        height: 12rem;
        right: 4rem;
        top: 14rem;
        animation: loginOrbit 14s linear infinite reverse;
      }

      .login-feature-tile,
      .login-card {
        position: relative;
        overflow: hidden;
      }

      .login-feature-tile::before,
      .login-card::before {
        content: "";
        position: absolute;
        inset: 0;
        background: linear-gradient(115deg, transparent, rgba(255, 255, 255, 0.4), transparent);
        transform: translateX(-120%);
        animation: loginSheen 7s ease-in-out infinite;
        pointer-events: none;
      }

      .dark .login-feature-tile::before,
      .dark .login-card::before {
        background: linear-gradient(115deg, transparent, rgba(255, 255, 255, 0.075), transparent);
      }

      .login-card::before {
        animation-delay: 0.8s;
      }

      .login-primary-button {
        position: relative;
        overflow: hidden;
        box-shadow: 0 0 28px rgba(47, 155, 76, 0.16);
      }

      .dark .login-primary-button {
        box-shadow: 0 0 28px rgba(86, 204, 112, 0.18);
      }

      .login-primary-button::after {
        content: "";
        position: absolute;
        inset: 0;
        background: linear-gradient(110deg, transparent 18%, rgba(255, 255, 255, 0.24), transparent 45%);
        transform: translateX(-120%);
        animation: loginButtonSweep 3.8s ease-in-out infinite;
        pointer-events: none;
      }

      @keyframes loginAuroraA {
        0%,
        100% {
          transform: translate3d(0, 0, 0) scale(1);
        }
        50% {
          transform: translate3d(4rem, 2.5rem, 0) scale(1.12);
        }
      }

      @keyframes loginAuroraB {
        0%,
        100% {
          transform: translate3d(0, 0, 0) scale(1);
        }
        50% {
          transform: translate3d(-3rem, -2rem, 0) scale(1.1);
        }
      }

      @keyframes loginBorderGlow {
        0%,
        100% {
          opacity: 0.46;
        }
        50% {
          opacity: 0.86;
        }
      }

      @keyframes loginRibbon {
        0%,
        100% {
          transform: translateX(-8%) rotate(-12deg);
          opacity: 0.42;
        }
        50% {
          transform: translateX(8%) rotate(-12deg);
          opacity: 0.86;
        }
      }

      @keyframes loginOrbit {
        from {
          transform: rotate(0deg);
        }
        to {
          transform: rotate(360deg);
        }
      }

      @keyframes loginSheen {
        0%,
        54% {
          transform: translateX(-120%);
        }
        76%,
        100% {
          transform: translateX(120%);
        }
      }

      @keyframes loginButtonSweep {
        0%,
        46% {
          transform: translateX(-120%);
        }
        74%,
        100% {
          transform: translateX(120%);
        }
      }

      @media (prefers-reduced-motion: reduce) {
        .login-aurora,
        .login-shell::before,
        .login-light-ribbon,
        .login-orbit,
        .login-feature-tile::before,
        .login-card::before,
        .login-primary-button::after {
          animation: none;
        }
      }
    `}</style>
  );
}
