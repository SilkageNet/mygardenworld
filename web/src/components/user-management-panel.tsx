"use client";

import { useEffect, useState, type FormEvent } from "react";
import { createClient } from "@connectrpc/connect";
import { AdminService } from "@/gen/mygardenworld/v1/admin_pb";
import type { User } from "@/gen/mygardenworld/v1/auth_pb";
import type { GetSystemStatsResponse } from "@/gen/mygardenworld/v1/admin_pb";
import { formatAPIError, transport } from "@/lib/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Cpu, Flower2, RotateCcw, Save, UserPlus, Users, Wifi } from "lucide-react";
import { cn } from "@/lib/utils";

const adminClient = createClient(AdminService, transport);

export function UserManagementPanel() {
  const [users, setUsers] = useState<User[]>([]);
  const [stats, setStats] = useState<GetSystemStatsResponse | null>(null);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busyUserId, setBusyUserId] = useState("");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [quotaDrafts, setQuotaDrafts] = useState<Record<string, string>>({});
  const [createForm, setCreateForm] = useState({
    username: "",
    email: "",
    password: "",
    maxAccounts: "5",
  });

  const refresh = async () => {
    setError("");
    try {
      const [userRes, statsRes] = await Promise.all([
        adminClient.listUsers({ page: 0, pageSize: 50 }),
        adminClient.getSystemStats({}),
      ]);
      setUsers(userRes.users);
      setQuotaDrafts(Object.fromEntries(userRes.users.map((user) => [user.id.toString(), user.maxAccounts.toString()])));
      setTotal(userRes.total);
      setStats(statsRes);
    } catch (err) {
      setError(formatAPIError(err, "加载管理数据失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  async function updateQuota(userId: bigint, maxAccounts: number) {
    if (!Number.isFinite(maxAccounts) || maxAccounts < 0) return;
    setBusyUserId(userId.toString());
    try {
      await adminClient.updateUser({ userId, maxAccounts });
      await refresh();
    } catch (err) {
      setError(formatAPIError(err, "更新配额失败"));
    } finally {
      setBusyUserId("");
    }
  }

  async function toggleStatus(userId: bigint, currentStatus: string) {
    const target = users.find((user) => user.id === userId);
    if (target?.role === "admin") {
      setError("管理员账号不能被禁用");
      return;
    }
    const newStatus = currentStatus === "active" ? "disabled" : "active";
    setBusyUserId(userId.toString());
    try {
      await adminClient.updateUser({ userId, status: newStatus });
      await refresh();
    } catch (err) {
      setError(formatAPIError(err, "更新用户状态失败"));
    } finally {
      setBusyUserId("");
    }
  }

  async function handleCreateUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setCreating(true);
    try {
      const maxAccounts = Number(createForm.maxAccounts);
      await adminClient.createUser({
        username: createForm.username,
        email: createForm.email,
        password: createForm.password,
        maxAccounts: Number.isFinite(maxAccounts) ? maxAccounts : 5,
      });
      setCreateForm({ username: "", email: "", password: "", maxAccounts: "5" });
      await refresh();
    } catch (err) {
      setError(formatAPIError(err, "创建用户失败"));
    } finally {
      setCreating(false);
    }
  }

  if (loading) {
    return (
      <div className="grid gap-3">
        <div className="grid gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <div key={index} className="h-20 animate-pulse rounded-lg bg-muted/35" />
          ))}
        </div>
        <div className="h-72 animate-pulse rounded-lg bg-muted/35" />
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      {stats && (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <AdminMetric icon={Users} label="用户总数" value={stats.totalUsers.toString()} tone="emerald" />
          <AdminMetric icon={Flower2} label="游戏账号" value={stats.totalGameAccounts.toString()} tone="amber" />
          <AdminMetric icon={Cpu} label="活跃 Runner" value={stats.activeRunners.toString()} tone="cyan" />
          <AdminMetric icon={Wifi} label="已连接" value={stats.connectedRunners.toString()} tone="violet" />
        </div>
      )}

      <Card size="sm">
        <CardHeader>
          <CardTitle>创建用户</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCreateUser} className="grid gap-3 md:grid-cols-[1fr_1fr_1fr_8rem_auto]">
            <Input
              value={createForm.username}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, username: event.target.value }))}
              placeholder="用户名"
              autoComplete="off"
              required
            />
            <Input
              type="email"
              value={createForm.email}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, email: event.target.value }))}
              placeholder="邮箱"
              autoComplete="off"
              required
            />
            <Input
              type="password"
              value={createForm.password}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, password: event.target.value }))}
              placeholder="初始密码"
              minLength={6}
              autoComplete="new-password"
              required
            />
            <Input
              type="number"
              min={0}
              value={createForm.maxAccounts}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, maxAccounts: event.target.value }))}
              placeholder="配额"
              required
            />
            <Button type="submit" disabled={creating}>
              <UserPlus className="size-4" />
              {creating ? "创建中" : "创建"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card size="sm">
        <CardHeader>
          <CardTitle>用户列表 ({total})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="dark-scrollbar overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户名</TableHead>
                  <TableHead>邮箱</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>账号配额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => {
                  const userId = user.id.toString();
                  const busy = busyUserId === userId;
                  const quotaDraft = quotaDrafts[userId] ?? user.maxAccounts.toString();
                  const quotaValue = Number(quotaDraft);
                  const quotaChanged = quotaDraft !== user.maxAccounts.toString();
                  const quotaInvalid = !Number.isInteger(quotaValue) || quotaValue < user.currentAccounts;
                  const isAdminUser = user.role === "admin";
                  return (
                    <TableRow key={userId}>
                      <TableCell className="font-medium">{user.username}</TableCell>
                      <TableCell className="text-muted-foreground">{user.email}</TableCell>
                      <TableCell>
                        <Badge variant={user.role === "admin" ? "default" : "secondary"} className={user.role === "admin" ? "bg-primary/10 text-primary" : ""}>
                          {user.role === "admin" ? "管理员" : "用户"}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="w-12 text-sm tabular-nums text-muted-foreground">{user.currentAccounts}/{user.maxAccounts}</span>
                          <Input
                            type="number"
                            min={user.currentAccounts}
                            className="h-8 w-20 text-right text-xs tabular-nums"
                            value={quotaDraft}
                            disabled={busy}
                            aria-invalid={quotaInvalid}
                            onChange={(event) => setQuotaDrafts((prev) => ({ ...prev, [userId]: event.target.value }))}
                          />
                          <Button
                            variant="outline"
                            size="icon-sm"
                            disabled={busy || !quotaChanged || quotaInvalid}
                            onClick={() => updateQuota(user.id, quotaValue)}
                            aria-label={`保存 ${user.username} 的账号配额`}
                          >
                            <Save className="size-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={busy || !quotaChanged}
                            onClick={() => setQuotaDrafts((prev) => ({ ...prev, [userId]: user.maxAccounts.toString() }))}
                            aria-label={`撤销 ${user.username} 的账号配额修改`}
                          >
                            <RotateCcw className="size-3.5" />
                          </Button>
                        </div>
                        {quotaInvalid && (
                          <p className="mt-1 text-xs text-destructive">配额不能小于当前账号数</p>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={user.status === "active" ? "default" : "destructive"} className={user.status === "active" ? "bg-primary/10 text-primary" : ""}>
                          {user.status === "active" ? "正常" : "禁用"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant={user.status === "active" ? "outline" : "default"}
                          size="sm"
                          disabled={busy || isAdminUser}
                          onClick={() => toggleStatus(user.id, user.status)}
                          title={isAdminUser ? "管理员账号不能被禁用" : undefined}
                        >
                          {isAdminUser ? "管理员" : busy ? "处理中" : user.status === "active" ? "禁用" : "启用"}
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })}
                {users.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="py-10 text-center text-muted-foreground">
                      暂无用户
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function AdminMetric({
  icon: Icon,
  label,
  value,
  tone,
}: {
  icon: typeof Users;
  label: string;
  value: string;
  tone: "emerald" | "amber" | "cyan" | "violet";
}) {
  const tones = {
    emerald: "bg-primary/10 text-primary",
    amber: "bg-amber-500/10 text-amber-300",
    cyan: "bg-cyan-500/10 text-cyan-300",
    violet: "bg-violet-500/10 text-violet-300",
  };

  return (
    <Card size="sm">
      <CardContent className="flex items-center gap-3">
        <div className={cn("flex size-9 items-center justify-center rounded-md", tones[tone])}>
          <Icon className="size-4" />
        </div>
        <div>
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-xl font-semibold tabular-nums">{value}</p>
        </div>
      </CardContent>
    </Card>
  );
}
