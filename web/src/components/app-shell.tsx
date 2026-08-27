"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { useAuth } from "@/lib/auth/context";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { LogOut, Shield, Sparkles } from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { UserManagementPanel } from "@/components/user-management-panel";
import { UserRole } from "@/gen/mygardenworld/v1/auth_pb";

export default function AppLayout({ children }: { children: ReactNode }) {
  const { user, loading, logout } = useAuth();
  const router = useRouter();
  const [userManagementOpen, setUserManagementOpen] = useState(false);

  useEffect(() => {
    if (!loading && !user) {
      router.replace("/login");
    }
  }, [user, loading, router]);

  if (loading || !user) {
    return (
      <div className="relative isolate flex min-h-dvh items-center justify-center overflow-hidden bg-background xl:h-screen">
        <SkySceneDecor />
        <div className="toy-shadow relative z-10 h-9 rounded-md border border-white/70 bg-card/86 px-4 py-2 text-sm text-muted-foreground backdrop-blur-xl dark:border-white/10">
          加载中...
        </div>
      </div>
    );
  }

  function handleLogout() {
    logout();
    router.push("/login");
  }

  const isAdmin = user.role === UserRole.ADMIN;

  return (
    <div className="relative isolate flex min-h-dvh flex-col overflow-x-hidden bg-transparent text-foreground xl:h-screen xl:overflow-hidden">
      <SkySceneDecor />
      <header className="sticky top-0 z-20 shrink-0 border-b border-white/45 bg-white/58 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/62 xl:static">
        <div className="flex h-[3.25rem] items-center justify-between px-3 sm:h-14 sm:px-6 lg:px-8 2xl:px-10">
          <Link
            href="/"
            className="group flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 transition-colors hover:bg-white/55 dark:hover:bg-white/8"
            aria-label="小云朵首页"
          >
            <span className="relative size-9 shrink-0 overflow-hidden sm:size-10">
              <Image
                src="/brand/cloud-logo.svg"
                alt="小云朵"
                fill
                priority
                unoptimized
                sizes="2.5rem"
                className="object-contain drop-shadow-[0_4px_10px_rgba(46,137,199,0.24)] transition-transform group-hover:-translate-y-0.5"
              />
            </span>
            <span className="hidden min-w-0 sm:block">
              <span className="flex items-center gap-1.5 text-sm font-semibold leading-none text-foreground">
                小云朵
                <Sparkles className="size-3.5 text-amber-400" />
              </span>
            </span>
          </Link>
          <div className="flex items-center gap-1.5 rounded-md border border-white/55 bg-white/42 p-1 shadow-sm shadow-sky-900/5 dark:border-white/10 dark:bg-white/6">
            {isAdmin && (
              <Button variant="ghost" size="icon-sm" onClick={() => setUserManagementOpen(true)} aria-label="用户管理">
                <Shield className="size-4" />
              </Button>
            )}
            <ThemeToggle />
            <Button variant="ghost" size="icon-sm" onClick={handleLogout} aria-label="退出登录">
              <LogOut className="size-4" />
            </Button>
          </div>
        </div>
      </header>

      <main className="relative z-10 flex-1 xl:min-h-0 xl:overflow-hidden">
        <div className="w-full px-3 py-3 sm:px-6 sm:py-4 lg:px-8 xl:h-full xl:overflow-hidden 2xl:px-10">
          {children}
        </div>
      </main>

      <Dialog open={userManagementOpen} onOpenChange={setUserManagementOpen}>
        <DialogContent className="max-h-[88vh] overflow-hidden sm:max-w-6xl">
          <DialogHeader>
            <DialogTitle>用户管理</DialogTitle>
            <DialogDescription>管理用户配额、状态和系统运行概况</DialogDescription>
          </DialogHeader>
          <div className="dark-scrollbar max-h-[calc(88vh-7.5rem)] overflow-y-auto pr-1">
            <UserManagementPanel />
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SkySceneDecor() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
      <span className="sky-decor-plane sky-glide left-[4%] top-20 bg-[linear-gradient(135deg,#ff8e88,#ff6268)] opacity-75 [--plane-duration:19s] [--plane-glide-x:2.4rem] [--plane-glide-y:-1.2rem] [--plane-loop-x:0.8rem] [--plane-loop-y:0.45rem] [--plane-rotate:18deg] sm:left-[6%]" />
      <span className="sky-decor-plane sky-glide right-[6%] top-24 bg-[linear-gradient(135deg,#ffd45d,#ffb331)] opacity-80 [--plane-duration:23s] [--plane-glide-x:-2.2rem] [--plane-glide-y:-1.1rem] [--plane-loop-x:-0.7rem] [--plane-loop-y:0.45rem] [--plane-rotate:-18deg]" />
      <span className="sky-decor-cloud sky-float left-[10%] top-32 opacity-70" />
      <span className="sky-decor-cloud sky-float right-[13%] top-36 scale-75 opacity-65" />
      <span className="sky-decor-star left-[35%] top-24 opacity-75" />
      <span className="sky-decor-star right-[27%] top-44 scale-75 bg-[#ff8bb1] opacity-70" />
      <span className="sky-decor-star left-[22%] bottom-24 scale-75 bg-[#b59cff] opacity-55" />
    </div>
  );
}
