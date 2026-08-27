"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Check, Flower2, Search, Sparkles } from "lucide-react";
import type { PlantableFlowerView, SellableFlowerArtView } from "@/lib/api/query-models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { allFlowers, flowerDisplay, itemName } from "@/lib/game/catalog";
import { flowerMatureCdSeconds } from "@/lib/game/flower-cd";
import { cn } from "@/lib/utils";
import { EmptyState, formatCount, QUALITY_LABELS, QUALITY_OPTIONS, toggleNumber } from "@/components/dashboard/policy-controls";

type FlowerPickerSortMode = "stock_desc" | "stock_asc" | "mature_asc" | "mature_desc";

const FLOWER_PICKER_SORT_OPTIONS: { value: FlowerPickerSortMode; label: string }[] = [
  { value: "stock_asc", label: "库存从低到高" },
  { value: "stock_desc", label: "库存从高到低" },
];

const PLANTABLE_FLOWER_PICKER_SORT_OPTIONS: { value: FlowerPickerSortMode; label: string }[] = [
  { value: "mature_asc", label: "成熟从短到长" },
  { value: "mature_desc", label: "成熟从长到短" },
  { value: "stock_asc", label: "库存从低到高" },
  { value: "stock_desc", label: "库存从高到低" },
];

export function FlowerArtMultiSelectRow({
  label,
  value,
  sellableArts,
  synced,
  onChange,
}: {
  label: string;
  value: number[];
  sellableArts: SellableFlowerArtView[];
  synced: boolean;
  onChange: (value: number[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [sortMode, setSortMode] = useState<FlowerPickerSortMode>("stock_desc");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const arts = useMemo(() => {
    const options = sellableArts.map((art) => ({
      id: art.artId,
      name: art.artName || itemName(art.artId),
      vaseName: art.vaseName || itemName(art.vaseId),
      vaseId: art.vaseId,
      stock: art.stock,
      saleValue: art.saleValue,
    }));
    const known = new Set(options.map((option) => option.id));
    for (const id of value) {
      if (known.has(id)) continue;
      options.push({
        id,
        name: itemName(id),
        vaseName: "",
        vaseId: 0,
        stock: 0,
        saleValue: 0,
      });
    }
    return options;
  }, [sellableArts, value]);
  const visibleArts = useMemo(() => {
    const text = query.trim().toLowerCase();
    return arts
      .filter((art) => {
        if (!text) return true;
        return (
          String(art.id).includes(text) ||
          art.name.toLowerCase().includes(text) ||
          art.vaseName.toLowerCase().includes(text) ||
          String(art.vaseId).includes(text)
        );
      })
      .sort((a, b) => {
        if (sortMode === "stock_desc" && a.stock !== b.stock) return b.stock - a.stock;
        if (a.stock !== b.stock) return a.stock - b.stock;
        if (a.saleValue !== b.saleValue) return b.saleValue - a.saleValue;
        return a.id - b.id;
      });
  }, [arts, query, sortMode]);
  const selectedPreview = value.slice(0, 3).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 3 ? value.length - 3 : 0;
  const toggleArt = (artID: number) => onChange(toggleNumber(value, artID));

  return (
    <div className="min-w-0 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5 sm:col-span-2">
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 sm:gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">可选 {arts.length}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "自动"}</Badge>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">仅展示当前账号已解锁花瓶对应的花艺；未选择时上架库存数量最多的花艺</p>
      <div className="flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden">
        <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择，按库存最多自动上架" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Sparkles className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) {
            setQuery("");
            setSortMode("stock_desc");
          }
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花艺名、花瓶或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
              <span className="shrink-0 text-xs text-muted-foreground">排序</span>
              <div className="flex flex-wrap gap-1">
                {FLOWER_PICKER_SORT_OPTIONS.map((option) => (
                  <FlowerPickerFilterChip
                    key={option.value}
                    selected={sortMode === option.value}
                    onClick={() => setSortMode(option.value)}
                  >
                    {option.label}
                  </FlowerPickerFilterChip>
                ))}
              </div>
              {!synced ? <span className="text-xs text-muted-foreground">登录后同步花瓶与库存</span> : null}
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleArts.length === 0 ? (
                <EmptyState title="没有可选花艺" detail={synced ? "请先解锁对应花瓶" : "登录后同步账号状态"} />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleArts.map((art) => {
                    const selected = selectedSet.has(art.id);
                    return (
                      <button
                        key={art.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleArt(art.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="truncate text-sm font-medium">{art.name}</span>
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            <span>{art.id}</span>
                            {art.vaseName ? <span>{art.vaseName}</span> : null}
                            <span>库存 {formatCount(art.stock)}</span>
                            {art.saleValue > 0 ? <span>售价 {formatCount(art.saleValue)}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button type="button" variant="ghost" className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted" onClick={() => onChange([])} disabled={value.length === 0}>
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                setQuery("");
                setSortMode("stock_desc");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export function CatalogFlowerMultiSelectRow({
  label,
  value,
  inventory,
  synced,
  onChange,
  className,
}: {
  label: string;
  value: number[];
  inventory: { [key: number]: number };
  synced: boolean;
  onChange: (value: number[]) => void;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [sortMode, setSortMode] = useState<FlowerPickerSortMode>("stock_asc");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const catalogCount = useMemo(() => allFlowers().length, []);
  const flowers = useMemo(() => {
    const catalogFlowers = allFlowers().map((flower) => {
      const display = flowerDisplay(flower.id);
      return {
        id: flower.id,
        name: display.name,
        seedName: display.seedName,
        color: display.item?.color,
        stock: inventory[flower.id] ?? 0,
      };
    });
    const known = new Set(catalogFlowers.map((flower) => flower.id));
    for (const id of value) {
      if (known.has(id)) continue;
      const display = flowerDisplay(id);
      catalogFlowers.push({
        id,
        name: display.name,
        seedName: display.seedName,
        color: display.item?.color,
        stock: inventory[id] ?? 0,
      });
    }
    return catalogFlowers;
  }, [inventory, value]);
  const visibleFlowers = useMemo(() => {
    const text = query.trim().toLowerCase();
    return flowers
      .filter((flower) => {
        if (!text) return true;
        return String(flower.id).includes(text) || flower.name.toLowerCase().includes(text) || flower.seedName.toLowerCase().includes(text);
      })
      .sort((a, b) => {
        if (sortMode === "stock_desc" && a.stock !== b.stock) return b.stock - a.stock;
        if (a.stock !== b.stock) return a.stock - b.stock;
        return a.id - b.id;
      });
  }, [flowers, query, sortMode]);
  const selectedPreview = value.slice(0, 4).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 4 ? value.length - 4 : 0;
  const toggleFlower = (flowerID: number) => onChange(toggleNumber(value, flowerID));

  return (
    <div className={cn("min-w-0 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5", className)}>
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 sm:gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">花库 {catalogCount}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "未选择"}</Badge>
        </div>
      </div>
      <div className="flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden">
        <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择时不限制" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Flower2 className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) {
            setQuery("");
            setSortMode("stock_asc");
          }
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花名、种子或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
              <span className="shrink-0 text-xs text-muted-foreground">排序</span>
              <div className="flex flex-wrap gap-1">
                {FLOWER_PICKER_SORT_OPTIONS.map((option) => (
                  <FlowerPickerFilterChip
                    key={option.value}
                    selected={sortMode === option.value}
                    onClick={() => setSortMode(option.value)}
                  >
                    {option.label}
                  </FlowerPickerFilterChip>
                ))}
              </div>
              {!synced ? <span className="text-xs text-muted-foreground">登录后同步库存</span> : null}
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleFlowers.length === 0 ? (
                <EmptyState title="没有匹配花朵" detail="换个名称或 ID 再试试" />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleFlowers.map((flower) => {
                    const selected = selectedSet.has(flower.id);
                    return (
                      <button
                        key={flower.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleFlower(flower.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="truncate text-sm font-medium">{flower.name}</span>
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            <span>{flower.id}</span>
                            {flower.color ? <span>品质 {flower.color}</span> : null}
                            <span>库存 {formatCount(flower.stock)}</span>
                            {flower.seedName ? <span>{flower.seedName}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button type="button" variant="ghost" className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted" onClick={() => onChange([])} disabled={value.length === 0}>
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                setQuery("");
                setSortMode("stock_asc");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

type FlowerPickerOption = {
  id: number;
  name: string;
  seedName: string;
  stock: number;
  gold: number;
  experience: number;
  lvl: number;
  cdSeconds: number;
  quality: number;
  plantable: boolean;
};

export function resetFlowerPickerFilters(
  setQuery: (value: string) => void,
  setQualityFilter: (value: number[]) => void,
  setLevelFilter: (value: number[]) => void,
  setSortMode: (value: FlowerPickerSortMode) => void,
  defaultSortMode: FlowerPickerSortMode = "stock_asc",
) {
  setQuery("");
  setQualityFilter([]);
  setLevelFilter([]);
  setSortMode(defaultSortMode);
}

export function compareFlowerPickerOptions(a: FlowerPickerOption, b: FlowerPickerOption, sortMode: FlowerPickerSortMode) {
  if (a.plantable !== b.plantable) return a.plantable ? -1 : 1;
  if (sortMode === "mature_asc" || sortMode === "mature_desc") {
    const aCD = a.cdSeconds > 0 ? a.cdSeconds : Number.POSITIVE_INFINITY;
    const bCD = b.cdSeconds > 0 ? b.cdSeconds : Number.POSITIVE_INFINITY;
    if (aCD !== bCD) return sortMode === "mature_desc" ? bCD - aCD : aCD - bCD;
    if (a.stock !== b.stock) return a.stock - b.stock;
    return a.id - b.id;
  }
  if (sortMode === "stock_desc" && a.stock !== b.stock) return b.stock - a.stock;
  if (a.stock !== b.stock) return a.stock - b.stock;
  return a.id - b.id;
}

export function formatFlowerMatureDuration(cdSeconds: number) {
  if (cdSeconds <= 0) return "";
  if (cdSeconds < 60) return `${cdSeconds}秒`;
  const minutes = Math.floor(cdSeconds / 60);
  const seconds = cdSeconds % 60;
  if (minutes < 60) {
    return seconds > 0 ? `${minutes}分${seconds}秒` : `${minutes}分钟`;
  }
  const hours = Math.floor(minutes / 60);
  const remMinutes = minutes % 60;
  return remMinutes > 0 ? `${hours}小时${remMinutes}分` : `${hours}小时`;
}

export function FlowerPickerFilterChip({
  selected,
  onClick,
  children,
}: {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex h-7 min-w-7 items-center justify-center rounded border px-1.5 text-xs font-medium",
        selected
          ? "border-primary bg-primary text-primary-foreground"
          : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
      )}
    >
      {children}
    </button>
  );
}

export function FlowerMultiSelectRow({
  label,
  value,
  plantableFlowers,
  synced,
  onChange,
}: {
  label: string;
  value: number[];
  plantableFlowers: PlantableFlowerView[];
  synced: boolean;
  onChange: (value: number[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [qualityFilter, setQualityFilter] = useState<number[]>([]);
  const [levelFilter, setLevelFilter] = useState<number[]>([]);
  const [sortMode, setSortMode] = useState<FlowerPickerSortMode>("mature_asc");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const flowers = useMemo<FlowerPickerOption[]>(() => {
    const options = plantableFlowers.map((flower) => {
      const display = flowerDisplay(flower.flowerId);
      return {
        id: flower.flowerId,
        name: flower.flowerName || display.name,
        seedName: display.seedName,
        stock: flower.stock,
        gold: flower.gold,
        experience: flower.experience,
        lvl: flower.lvl,
        // Always recompute from current cultivation level (not catalog lvl1 / base row alone).
        cdSeconds: flowerMatureCdSeconds(flower.flowerId, flower.lvl) || flower.cdSeconds,
        quality: display.item?.color ?? 0,
        plantable: true,
      };
    });
    const known = new Set(options.map((option) => option.id));
    for (const id of value) {
      if (known.has(id)) continue;
      const display = flowerDisplay(id);
      options.push({
        id,
        name: display.name,
        seedName: display.seedName,
        stock: 0,
        gold: display.flower?.gold ?? 0,
        experience: display.flower?.experience ?? 0,
        lvl: 0,
        cdSeconds: 0,
        quality: display.item?.color ?? 0,
        plantable: false,
      });
    }
    return options;
  }, [plantableFlowers, value]);
  const availableLevels = useMemo(() => {
    const levels = new Set<number>();
    for (const flower of flowers) {
      if (flower.lvl > 0) levels.add(flower.lvl);
    }
    return [...levels].sort((a, b) => a - b);
  }, [flowers]);
  const qualityCounts = useMemo(() => {
    const counts: Record<number, number> = {};
    for (const quality of QUALITY_OPTIONS) counts[quality] = 0;
    for (const flower of flowers) {
      if (flower.quality > 0) counts[flower.quality] = (counts[flower.quality] ?? 0) + 1;
    }
    return counts;
  }, [flowers]);
  const visibleFlowers = useMemo(() => {
    const text = query.trim().toLowerCase();
    const qualitySet = qualityFilter.length > 0 ? new Set(qualityFilter) : null;
    const levelSet = levelFilter.length > 0 ? new Set(levelFilter) : null;
    return flowers
      .filter((flower) => {
        if (qualitySet && !qualitySet.has(flower.quality)) return false;
        if (levelSet && !levelSet.has(flower.lvl)) return false;
        if (!text) return true;
        const qualityLabel = QUALITY_LABELS[flower.quality] ?? "";
        return (
          String(flower.id).includes(text) ||
          flower.name.toLowerCase().includes(text) ||
          flower.seedName.toLowerCase().includes(text) ||
          qualityLabel.includes(text) ||
          (flower.lvl > 0 && (`lv${flower.lvl}` === text || `等级${flower.lvl}` === text || String(flower.lvl) === text))
        );
      })
      .sort((a, b) => compareFlowerPickerOptions(a, b, sortMode));
  }, [flowers, levelFilter, qualityFilter, query, sortMode]);
  const selectedPreview = value.slice(0, 4).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 4 ? value.length - 4 : 0;
  const toggleFlower = (flowerID: number) => onChange(toggleNumber(value, flowerID));
  const filterActive = qualityFilter.length > 0 || levelFilter.length > 0;

  return (
    <div className="min-w-0 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 sm:gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">可种 {plantableFlowers.length}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "未选择"}</Badge>
        </div>
      </div>
      <div className="flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden">
        <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择时不限制" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Flower2 className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) resetFlowerPickerFilters(setQuery, setQualityFilter, setLevelFilter, setSortMode, "mature_asc");
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花名、种子、品质或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="flex shrink-0 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">品质</span>
                <div className="flex flex-wrap gap-1">
                  {QUALITY_OPTIONS.map((quality) => (
                    <FlowerPickerFilterChip
                      key={quality}
                      selected={qualityFilter.includes(quality)}
                      onClick={() => setQualityFilter((current) => toggleNumber(current, quality))}
                    >
                      {QUALITY_LABELS[quality]}({qualityCounts[quality] ?? 0})
                    </FlowerPickerFilterChip>
                  ))}
                </div>
              </div>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">等级</span>
                <div className="flex flex-wrap gap-1">
                  {availableLevels.length === 0 ? (
                    <span className="text-xs text-muted-foreground">{synced ? "暂无等级数据" : "登录后同步等级"}</span>
                  ) : (
                    availableLevels.map((level) => (
                      <FlowerPickerFilterChip
                        key={level}
                        selected={levelFilter.includes(level)}
                        onClick={() => setLevelFilter((current) => toggleNumber(current, level))}
                      >
                        Lv.{level}
                      </FlowerPickerFilterChip>
                    ))
                  )}
                </div>
                {filterActive ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="ml-auto h-7 px-2 text-xs"
                    onClick={() => {
                      setQualityFilter([]);
                      setLevelFilter([]);
                    }}
                  >
                    清除筛选
                  </Button>
                ) : null}
              </div>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">排序</span>
                <div className="flex flex-wrap gap-1">
                  {PLANTABLE_FLOWER_PICKER_SORT_OPTIONS.map((option) => (
                    <FlowerPickerFilterChip
                      key={option.value}
                      selected={sortMode === option.value}
                      onClick={() => setSortMode(option.value)}
                    >
                      {option.label}
                    </FlowerPickerFilterChip>
                  ))}
                </div>
              </div>
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleFlowers.length === 0 ? (
                <EmptyState
                  title={synced ? "没有匹配花种" : "尚未同步可种花种"}
                  detail={synced ? (filterActive || query.trim() ? "试试调整品质/等级筛选或搜索词" : undefined) : "登录账号并同步培育状态后可选择"}
                />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleFlowers.map((flower) => {
                    const selected = selectedSet.has(flower.id);
                    const qualityLabel = flower.quality > 0 ? QUALITY_LABELS[flower.quality] : "";
                    const matureLabel = formatFlowerMatureDuration(flower.cdSeconds);
                    return (
                      <button
                        key={flower.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleFlower(flower.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex min-w-0 items-center gap-1.5">
                            <span className="shrink-0 text-xs text-muted-foreground">{flower.id}</span>
                            <span className="truncate text-sm font-medium">{flower.name}</span>
                            {flower.lvl > 0 ? <Badge variant="secondary">Lv.{flower.lvl}</Badge> : null}
                            {!flower.plantable && <Badge variant="outline">当前不可种</Badge>}
                          </span>
                          {matureLabel ? (
                            <span className="mt-1 block text-xs text-muted-foreground">
                              成熟 {matureLabel}
                              {flower.lvl > 0 ? `（按 Lv.${flower.lvl}）` : null}
                            </span>
                          ) : null}
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            {qualityLabel ? <span>{qualityLabel}</span> : null}
                            {flower.stock > 0 ? <span>库存 {formatCount(flower.stock)}</span> : null}
                            {flower.gold ? <span>金币 {formatCount(flower.gold)}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button
              type="button"
              variant="ghost"
              className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted"
              onClick={() => onChange([])}
              disabled={value.length === 0}
            >
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                resetFlowerPickerFilters(setQuery, setQualityFilter, setLevelFilter, setSortMode, "mature_asc");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
