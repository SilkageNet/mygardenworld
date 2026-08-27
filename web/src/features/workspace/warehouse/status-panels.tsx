"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Flower2, Package, Search, Sparkles } from "lucide-react";
import type { InventoryLedgerItem, InventoryLedgerView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

type WarehouseCategory = "flower" | "art" | "item";

const WAREHOUSE_CATEGORIES: { id: WarehouseCategory; label: string; icon: ReactNode }[] = [
  { id: "flower", label: "鲜花", icon: <Flower2 /> },
  { id: "art", label: "花艺", icon: <Sparkles /> },
  { id: "item", label: "道具", icon: <Package /> },
];

function warehouseCategoryForItem(item: InventoryLedgerItem): WarehouseCategory {
  const id = item.itemId;
  if (id >= 23000 && id < 24000) return "flower";
  if (id >= 300000 && id < 400000) return "art";
  return "item";
}

function warehouseCategoryLabel(category: WarehouseCategory) {
  return WAREHOUSE_CATEGORIES.find((entry) => entry.id === category)?.label ?? "仓库";
}

function warehouseSearchPlaceholder(category: WarehouseCategory) {
  switch (category) {
    case "flower": return "搜索花朵或 ID";
    case "art": return "搜索花艺或 ID";
    case "item": return "搜索道具或 ID";
  }
}

export function WarehouseMonitorPanel({ ledger }: { ledger?: InventoryLedgerView }) {
  const [inventoryQuery, setInventoryQuery] = useState("");
  const [warehouseCategory, setWarehouseCategory] = useState<WarehouseCategory>("flower");
  const inventoryItems = useMemo(() => [...(ledger?.items ?? [])]
    .filter((item) => item.owned > 0 || item.allocated > 0)
    .sort((left, right) => right.owned - left.owned || right.allocated - left.allocated || left.itemId - right.itemId), [ledger]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<WarehouseCategory, number>();
    for (const category of WAREHOUSE_CATEGORIES) counts.set(category.id, 0);
    for (const item of inventoryItems) {
      const category = warehouseCategoryForItem(item);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [inventoryItems]);
  const categoryItems = useMemo(
    () => inventoryItems.filter((item) => warehouseCategoryForItem(item) === warehouseCategory),
    [inventoryItems, warehouseCategory],
  );
  const visibleItems = useMemo(() => {
    const query = inventoryQuery.trim().toLowerCase();
    if (!query) return categoryItems;
    return categoryItems.filter((item) => {
      const name = item.itemName || itemName(item.itemId);
      return name.toLowerCase().includes(query) || String(item.itemId).includes(query);
    });
  }, [categoryItems, inventoryQuery]);
  const categoryLabel = warehouseCategoryLabel(warehouseCategory);

  return (
    <CollapsibleCard
      title="仓库"
      actions={inventoryItems.length > 0 ? (
        <>
          <Badge variant="secondary">种类 {inventoryItems.length}</Badge>
          {inventoryQuery.trim() && <Badge variant="outline">匹配 {visibleItems.length}</Badge>}
        </>
      ) : undefined}
    >
      {inventoryItems.length > 0 && (
        <div className="mb-3 grid gap-2 lg:grid-cols-[minmax(296px,1fr)_minmax(150px,0.65fr)] lg:items-center">
          <div className="grid min-w-0 grid-cols-3 rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5">
            {WAREHOUSE_CATEGORIES.map((category) => (
              <button
                key={category.id}
                type="button"
                aria-pressed={warehouseCategory === category.id}
                onClick={() => {
                  setWarehouseCategory(category.id);
                  setInventoryQuery("");
                }}
                className={cn(
                  "flex h-8 min-w-0 items-center justify-center gap-1.5 rounded px-2 text-xs font-medium transition-colors",
                  warehouseCategory === category.id
                    ? "bg-white text-foreground shadow-sm dark:bg-muted"
                    : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
                )}
              >
                <span className="shrink-0 [&_svg]:size-3.5">{category.icon}</span>
                <span className="shrink-0 whitespace-nowrap">{category.label}</span>
                <span className="shrink-0 tabular-nums text-muted-foreground">{categoryCounts.get(category.id) ?? 0}</span>
              </button>
            ))}
          </div>
          <div className="relative min-w-0">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={inventoryQuery}
              onChange={(event) => setInventoryQuery(event.target.value)}
              placeholder={warehouseSearchPlaceholder(warehouseCategory)}
              className="h-10 rounded-md pl-9"
            />
          </div>
        </div>
      )}
      {inventoryItems.length === 0 ? (
        <EmptyState title="暂无仓库数据" />
      ) : categoryItems.length === 0 ? (
        <EmptyState title={`暂无${categoryLabel}`} />
      ) : visibleItems.length === 0 ? (
        <EmptyState title={`没有匹配${categoryLabel}`} detail="换个名称或 ID 再试试" />
      ) : (
        <div className="dark-scrollbar max-h-[440px] overflow-y-auto rounded-md border border-border/58 bg-white/42 sm:h-[560px] sm:max-h-none dark:bg-white/5">
          <Table>
            <TableHeader className="sticky top-0 z-10 bg-card/92 shadow-[0_1px_0_0_var(--border)] backdrop-blur-xl">
              <TableRow className="hover:bg-transparent">
                <TableHead className="h-9 text-xs">名称</TableHead>
                <TableHead className="h-9 text-xs">数量</TableHead>
                <TableHead className="h-9 text-xs">预留</TableHead>
                <TableHead className="h-9 text-xs">可用</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleItems.map((item) => (
                <TableRow key={item.itemId} className="h-10 hover:bg-muted/35">
                  <TableCell className="min-w-0">
                    <div className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate font-medium">{item.itemName || itemName(item.itemId)}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">{item.itemId}</span>
                    </div>
                  </TableCell>
                  <TableCell>{item.owned}</TableCell>
                  <TableCell className={cn(item.allocated > 0 && "text-primary")}>{item.allocated}</TableCell>
                  <TableCell className={cn(item.available < 0 && "text-destructive")}>{item.available}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </CollapsibleCard>
  );
}
