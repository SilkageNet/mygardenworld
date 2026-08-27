"use client";

import { useMemo, useState, type PointerEvent, type ReactNode } from "react";
import { GripVertical, Minus, Plus, Sparkles } from "lucide-react";
import { SelectionMode } from "@/gen/mygardenworld/v1/policy_pb";
import type { FriendTouchFriendView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { type SettingStatus } from "@/lib/feature-capabilities";
import { cn } from "@/lib/utils";

const NUMBER_FORMATTER = new Intl.NumberFormat("zh-CN");

const GOAL_OPTIONS = [
  { id: "order.customer", label: "顾客订单", defaultPriority: 90 },
  { id: "order.resident", label: "居民订单", defaultPriority: 80 },
  { id: "basic.task.main", label: "主线任务", defaultPriority: 70 },
  { id: "basic.task.daily", label: "日常任务", defaultPriority: 60 },
  { id: "basic.task.weekly", label: "周常任务", defaultPriority: 55 },
  { id: "order.flower_art", label: "花艺/花架", defaultPriority: 40 },
  { id: "fallback.auto_replant", label: "自主补种", defaultPriority: 10 },
];

export const QUALITY_OPTIONS = [1, 2, 3, 4, 5];
export const QUALITY_LABELS: Record<number, string> = { 1: "凡", 2: "普", 3: "珍", 4: "华", 5: "仙" };

export function PolicyGroup({ title, icon, description, children }: { title: string; icon: ReactNode; description?: string; children: ReactNode; }) {
  return (
    <section className="space-y-3 rounded-xl border border-border/55 bg-white/34 p-3 sm:p-4 dark:bg-white/5">
      <div>
        <SectionTitle icon={icon}>{title}</SectionTitle>
        {description && <p className="mt-1 pl-9 text-xs leading-5 text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  );
}

export function StatusRow({ label, value, tone }: { label: string; value: string; tone: "ready" | "muted" | "warn"; }) {
  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Badge variant={tone === "ready" ? "secondary" : tone === "warn" ? "destructive" : "outline"}>{value}</Badge>
    </div>
  );
}

export function TextRow({ label, value, description, onChange }: { label: string; value: string; description?: string; onChange: (value: string) => void; }) {
  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5">
      <SettingLabel label={label} description={description} />
      <Input className="h-9 w-full text-sm sm:w-48 sm:text-right" value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

export function BigIntNumberRow({
  label,
  value,
  min,
  description,
  onChange,
}: {
  label: string;
  value: bigint;
  min: number;
  description?: string;
  onChange: (value: bigint) => void;
}) {
  const floor = BigInt(min);
  const normalizedValue = value < floor ? floor : value;

  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5">
      <SettingLabel label={label} description={description} />
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        decrementDisabled={normalizedValue <= floor}
        onDecrement={() => onChange(normalizedValue - BigInt(1))}
        onIncrement={() => onChange(normalizedValue + BigInt(1))}
        onValueChange={(nextValue) => onChange(parseBigInt(nextValue, min))}
        wide
      />
    </div>
  );
}

export function NumericStepper({
  label,
  value,
  min,
  max,
  disabled = false,
  decrementDisabled = false,
  incrementDisabled = false,
  wide = false,
  onDecrement,
  onIncrement,
  onValueChange,
}: {
  label: string;
  value: string;
  min: number;
  max?: number;
  disabled?: boolean;
  decrementDisabled?: boolean;
  incrementDisabled?: boolean;
  wide?: boolean;
  onDecrement: () => void;
  onIncrement: () => void;
  onValueChange: (value: string) => void;
}) {
  const buttonClassName =
    "flex h-full items-center justify-center text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn(
        "grid h-9 shrink-0 grid-cols-[2.25rem_minmax(0,1fr)_2.25rem] overflow-hidden rounded-lg border border-input/85 bg-white/66 shadow-[inset_0_1px_0_rgba(255,255,255,0.78)] transition-[border-color,box-shadow,background-color] focus-within:border-ring focus-within:bg-white/88 focus-within:ring-3 focus-within:ring-ring/24 dark:bg-input/42 dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] dark:focus-within:bg-input/58",
        wide ? "w-40" : "w-36",
        disabled && "opacity-55",
      )}
    >
      <button
        type="button"
        aria-label={`减少${label}`}
        className={cn(buttonClassName, "border-r border-input/65")}
        disabled={disabled || decrementDisabled}
        onClick={onDecrement}
      >
        <Minus className="size-3.5" />
      </button>
      <input
        type="number"
        inputMode="numeric"
        aria-label={label}
        className="h-full min-w-0 bg-transparent px-1 text-center text-sm font-semibold tabular-nums text-foreground outline-none [appearance:textfield] disabled:cursor-not-allowed [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
        min={min}
        max={max}
        step={1}
        disabled={disabled}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
      />
      <button
        type="button"
        aria-label={`增加${label}`}
        className={cn(buttonClassName, "border-l border-input/65")}
        disabled={disabled || incrementDisabled}
        onClick={onIncrement}
      >
        <Plus className="size-3.5" />
      </button>
    </div>
  );
}

export function IntListRow({
  label,
  value,
  onChange,
  description,
}: {
  label: string;
  value: number[];
  onChange: (value: number[]) => void;
  description?: string;
}) {
  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,18rem)] sm:items-center dark:bg-white/5">
      <SettingLabel label={label} description={description} />
      <Input
        className="h-9 text-sm"
        value={formatIntList(value)}
        onChange={(event) => onChange(parseIntList(event.target.value))}
        placeholder="用逗号分隔 ID"
      />
    </div>
  );
}

export function QualityRow({
  label,
  value,
  onChange,
  labels,
  emptyMeansAll = false,
}: {
  label: string;
  value: number[];
  onChange: (value: number[]) => void;
  labels?: Record<number, string>;
  emptyMeansAll?: boolean;
}) {
  const selectedSet = useMemo(() => {
    if (emptyMeansAll && value.length === 0) return new Set(QUALITY_OPTIONS);
    return new Set(value);
  }, [emptyMeansAll, value]);

  const toggleQuality = (quality: number) => {
    const current = emptyMeansAll && value.length === 0 ? [...QUALITY_OPTIONS] : value;
    const next = toggleNumber(current, quality);
    if (emptyMeansAll && next.length === QUALITY_OPTIONS.length) {
      onChange([]);
      return;
    }
    onChange(next);
  };

  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5">
      <SettingLabel label={label} />
      <div className="flex gap-1">
        {QUALITY_OPTIONS.map((quality) => {
          const selected = selectedSet.has(quality);
          return (
            <button
              key={quality}
              type="button"
              onClick={() => toggleQuality(quality)}
              className={cn(
                "flex h-7 min-w-7 items-center justify-center rounded border px-1.5 text-xs font-medium",
                selected ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
              )}
            >
              {labels?.[quality] ?? quality}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function SegmentedRow<T extends number>({
  label,
  value,
  options,
  description,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string; }[];
  description?: string;
  onChange: (value: T) => void;
}) {
  return (
    <div className="grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5">
      <SettingLabel label={label} description={description} />
      <div className="flex flex-wrap gap-1 sm:justify-end">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "min-h-8 rounded border px-2 text-xs font-medium",
              option.value === value ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

export function DemandPriorityEditor({
  value,
  onChange,
}: {
  value: Record<string, number>;
  onChange: (value: Record<string, number>) => void;
}) {
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const orderedGoals = useMemo(() => {
    return [...GOAL_OPTIONS].sort((a, b) => {
      const priorityDelta = priorityForGoal(value, b) - priorityForGoal(value, a);
      if (priorityDelta !== 0) return priorityDelta;
      return GOAL_OPTIONS.findIndex((goal) => goal.id === a.id) - GOAL_OPTIONS.findIndex((goal) => goal.id === b.id);
    });
  }, [value]);

  const commitOrder = (goals: typeof GOAL_OPTIONS) => {
    const total = goals.length;
    onChange(Object.fromEntries(goals.map((goal, index) => [goal.id, (total - index) * 10])));
  };

  const moveGoal = (sourceId: string, targetId: string) => {
    if (sourceId === targetId) return;
    const from = orderedGoals.findIndex((goal) => goal.id === sourceId);
    const to = orderedGoals.findIndex((goal) => goal.id === targetId);
    if (from < 0 || to < 0) return;
    const next = [...orderedGoals];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    commitOrder(next);
  };

  const handlePointerUp = (event: PointerEvent<HTMLDivElement>, sourceId: string) => {
    const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
    const targetId = target?.dataset.goalId;
    if (targetId) moveGoal(sourceId, targetId);
    setDraggingId(null);
  };

  return (
    <div className="mt-3 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-muted-foreground">拖拽调整缺花补种顺序</span>
      </div>
      <div
        className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3"
        onMouseUp={(event) => {
          if (!draggingId) return;
          const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
          const targetId = target?.dataset.goalId;
          if (targetId) moveGoal(draggingId, targetId);
          setDraggingId(null);
        }}
        onMouseLeave={(event) => {
          if (event.buttons === 0) setDraggingId(null);
        }}
      >
        {orderedGoals.map((goal, index) => (
          <div
            key={goal.id}
            data-goal-id={goal.id}
            aria-grabbed={draggingId === goal.id}
            onMouseDown={(event) => {
              if (event.button !== 0) return;
              setDraggingId(goal.id);
            }}
            onPointerDown={(event) => {
              if (event.button !== 0) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              setDraggingId(goal.id);
            }}
            onPointerUp={(event) => handlePointerUp(event, goal.id)}
            onPointerCancel={() => setDraggingId(null)}
            className={cn(
              "flex min-h-11 cursor-grab touch-none items-center gap-2 rounded-md border border-border/70 bg-card px-2.5 py-2 text-sm shadow-sm transition active:cursor-grabbing",
              draggingId === goal.id ? "opacity-60 ring-1 ring-primary" : "hover:border-primary/50 hover:bg-white/66 dark:hover:bg-white/8",
            )}
          >
            <GripVertical className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <span className="flex size-6 shrink-0 items-center justify-center rounded bg-secondary text-xs font-medium text-muted-foreground dark:bg-white/8">{index + 1}</span>
            <span className="min-w-0 flex-1 truncate font-medium">{goal.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function priorityForGoal(value: Record<string, number>, goal: (typeof GOAL_OPTIONS)[number]) {
  return value[goal.id] || goal.defaultPriority;
}
export function ToggleRow({
  label,
  checked,
  onChange,
  status,
  description,
  disabled = false,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  status?: SettingStatus;
  description?: string;
  disabled?: boolean;
}) {
  return (
    <div
      className={cn(
        "grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5",
        disabled && "opacity-55",
      )}
    >
      <span className="flex min-w-0 flex-wrap items-center gap-2">
        <SettingLabel label={label} description={description} />
        {status && <SettingStatusBadge status={status} />}
      </span>
      <Switch checked={checked} disabled={disabled} onCheckedChange={onChange} />
    </div>
  );
}

export function SettingStatusBadge({ status }: { status: SettingStatus; }) {
  const variant = status.kind === "sync_only" ? "outline" : "destructive";
  return (
    <Badge variant={variant} title={status.detail} className="shrink-0">
      {status.label}
    </Badge>
  );
}

export function FriendTouchFriendList({
  friends,
  observed,
  mode,
  counts,
  excluded,
  autoBuy,
  maxBuyPerFriend,
  onCountChange,
  onExcludedChange,
}: {
  friends: FriendTouchFriendView[];
  observed: boolean;
  mode: SelectionMode;
  counts: Record<string, number>;
  excluded: Set<string>;
  autoBuy: boolean;
  maxBuyPerFriend: number;
  onCountChange: (uid: bigint, count: number) => void;
  onExcludedChange: (uid: bigint, excluded: boolean) => void;
}) {
  const specific = mode === SelectionMode.SPECIFIC;
  if (!observed) {
    return (
      <EmptyState
        title="尚未同步好友列表"
        detail="请先开启自动摸花并保存；下一轮会同步好友列表，随后即可配置指定目标。"
      />
    );
  }
  if (friends.length === 0) {
    return <EmptyState title="暂无好友" detail="游戏好友列表为空时无法配置摸花目标。" />;
  }
  return (
    <div className="mt-3 space-y-2">
      <div className="flex items-center justify-between gap-2 px-1">
        <span className="text-xs text-muted-foreground">
          {specific
            ? "为指定好友设置今日摸花次数；可勾选排除不主动摸取"
            : "自动摸取全部可摘好友；勾选排除后跳过该好友"}
        </span>
        <span className="text-xs text-muted-foreground">{friends.length} 人</span>
      </div>
      <div className="max-h-72 space-y-2 overflow-y-auto pr-1">
        {friends.map((friend) => {
          const key = friend.uid.toString();
          const target = counts[key] ?? 0;
          const isExcluded = excluded.has(key);
          const displayName = friend.name.trim() || (friend.profileObserved ? `UID ${key}` : `好友 ${key}`);
          const progress =
            friend.quotaObserved
              ? `今日 ${friend.stolenCount}/${friend.stealMax}`
              : "次数未同步";
          const targetMax = friend.baseStealMax + (autoBuy ? maxBuyPerFriend : friend.boughtCount);
          return (
            <div
              key={key}
              className={cn(
                "flex flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5",
                isExcluded && "opacity-60",
              )}
            >
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0 space-y-0.5">
                  <div className="truncate text-sm font-medium">{displayName}</div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <span>UID {key}</span>
                    <span>{progress}</span>
                    {friend.availabilityObserved && friend.canSteal ? (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                        可摘
                      </Badge>
                    ) : friend.availabilityObserved ? (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] text-muted-foreground">
                        暂不可摘
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] text-muted-foreground">
                        状态待同步
                      </Badge>
                    )}
                  </div>
                </div>
                <label className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
                  <Switch checked={isExcluded} onCheckedChange={(checked) => onExcludedChange(friend.uid, checked)} />
                  排除
                </label>
              </div>
              {specific && !isExcluded && (
                <div className="flex items-center justify-between gap-3">
                  <span className="text-xs text-muted-foreground">目标次数</span>
                  <NumericStepper
                    label={`${displayName} 目标次数`}
                    value={target.toString()}
                    min={0}
                    max={targetMax > 0 ? targetMax : undefined}
                    decrementDisabled={target <= 0}
                    onDecrement={() => onCountChange(friend.uid, target - 1)}
                    onIncrement={() => onCountChange(friend.uid, target + 1)}
                    onValueChange={(nextValue) => onCountChange(friend.uid, parseNumber(nextValue, 0))}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function NumberRow({
  label,
  value,
  min,
  max,
  disabled = false,
  onChange,
  description,
}: {
  label: string;
  value: number;
  min: number;
  max?: number;
  disabled?: boolean;
  onChange: (value: number) => void;
  description?: string;
}) {
  const normalizedValue = Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, Number.isFinite(value) ? Math.trunc(value) : min));
  const updateValue = (nextValue: number) => onChange(Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, nextValue)));

  return (
    <div
      className={cn(
        "grid min-h-14 gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2.5 transition-opacity sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center dark:bg-white/5",
        disabled && "opacity-55",
      )}
    >
      <SettingLabel label={label} description={description} />
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        max={max}
        disabled={disabled}
        decrementDisabled={normalizedValue <= min}
        incrementDisabled={max !== undefined && normalizedValue >= max}
        onDecrement={() => updateValue(normalizedValue - 1)}
        onIncrement={() => updateValue(normalizedValue + 1)}
        onValueChange={(nextValue) => updateValue(parseNumber(nextValue, min))}
      />
    </div>
  );
}

function SettingLabel({ label, description }: { label: string; description?: string }) {
  return (
    <Label className="flex min-w-0 flex-col gap-0.5 leading-5">
      <span className="text-sm font-medium text-foreground">{label}</span>
      {description && <span className="text-xs font-normal leading-5 text-muted-foreground">{description}</span>}
    </Label>
  );
}

export function SectionTitle({ icon, children }: { icon: ReactNode; children: ReactNode; }) {
  return (
    <div className="flex items-center gap-2 text-sm font-semibold">
      <span className="flex size-7 items-center justify-center rounded-md bg-secondary text-sky-600 dark:bg-white/8 dark:text-sky-300 [&_svg]:size-4">{icon}</span>
      {children}
    </div>
  );
}

export function EmptyState({ title, detail }: { title: string; detail?: string; }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}

export function formatCount(value: number | bigint) {
  const numeric = typeof value === "bigint" ? Number(value) : value;
  if (!Number.isFinite(numeric)) return "0";
  return NUMBER_FORMATTER.format(numeric);
}

export function parseNumber(value: string, min: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return min;
  return Math.max(min, Math.trunc(parsed));
}

export function parseBigInt(value: string, min: number) {
  const cleaned = value.trim();
  if (!cleaned) return BigInt(min);
  try {
    const parsed = BigInt(cleaned);
    const floor = BigInt(min);
    return parsed < floor ? floor : parsed;
  } catch {
    return BigInt(min);
  }
}

export function safeBigIntToNumber(value: bigint | undefined, fallback: number) {
  if (value === undefined) return fallback;
  const upper = BigInt(Number.MAX_SAFE_INTEGER);
  const lower = BigInt(Number.MIN_SAFE_INTEGER);
  if (value > upper) return Number.MAX_SAFE_INTEGER;
  if (value < lower) return Number.MIN_SAFE_INTEGER;
  return Number(value);
}

export function safeNumberToBigInt(value: number, fallback: number) {
  if (!Number.isFinite(value)) return BigInt(fallback);
  const integer = Math.trunc(value);
  if (integer > Number.MAX_SAFE_INTEGER) return BigInt(Number.MAX_SAFE_INTEGER);
  if (integer < Number.MIN_SAFE_INTEGER) return BigInt(Number.MIN_SAFE_INTEGER);
  return BigInt(integer);
}

export function parseIntList(value: string) {
  const seen = new Set<number>();
  const out: number[] = [];
  for (const part of value.split(/[,\s，、]+/)) {
    const parsed = Number(part.trim());
    if (!Number.isInteger(parsed) || parsed <= 0 || seen.has(parsed)) continue;
    seen.add(parsed);
    out.push(parsed);
  }
  return out;
}

export function formatIntList(value: number[]) {
  return value.join(", ");
}

export function toggleNumber(values: number[], value: number) {
  if (values.includes(value)) return values.filter((item) => item !== value);
  return [...values, value].sort((a, b) => a - b);
}
