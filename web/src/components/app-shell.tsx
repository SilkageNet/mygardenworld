"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/lib/auth/context";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { LogOut, Shield } from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { UserManagementPanel } from "@/components/user-management-panel";

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
      <div className="flex min-h-dvh items-center justify-center bg-background xl:h-screen xl:overflow-hidden">
        <div className="h-9 rounded-md border border-border bg-card px-4 py-2 text-sm text-muted-foreground shadow-sm">
          加载中...
        </div>
      </div>
    );
  }

  function handleLogout() {
    logout();
    router.push("/login");
  }

  const isAdmin = user.role === "admin";

  return (
    <div className="flex min-h-dvh flex-col bg-background text-foreground xl:h-screen xl:overflow-hidden">
      <header className="shrink-0 border-b border-border/70 bg-card/92 backdrop-blur">
        <div className="flex h-14 items-center justify-between px-4 sm:px-6 lg:px-8 2xl:px-10">
          <Link href="/" className="min-w-0">
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold">花园世界</div>
            </div>
          </Link>
          <div className="flex items-center gap-2">
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

      <main className="flex-1 xl:min-h-0 xl:overflow-hidden">
        <div className="w-full px-4 py-4 sm:px-6 lg:px-8 xl:h-full xl:overflow-hidden 2xl:px-10">
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
