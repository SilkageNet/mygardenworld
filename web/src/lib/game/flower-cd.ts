import flowerCd from "./flower-cd.json";

type FlowerCdCatalog = {
  cfg: Record<string, number>;
  base: Record<string, number>;
  exact: Record<string, number>;
};

const catalog = flowerCd as FlowerCdCatalog;

/**
 * Catalog grow CD in seconds for a flower at cultivation level.
 * Mirrors backend FlowerLvlCDSeconds / client G.CFG.getFlowerLvlCfg:
 * per-level row → base-row scaled by cfg(level)/cfg(1) → bare cfg.
 */
export function flowerMatureCdSeconds(flowerId: number, level: number): number {
  if (flowerId <= 0 || level <= 0) return 0;
  const exact = catalog.exact[String(flowerId * 100 + level)];
  if (exact > 0) return exact;
  const base = catalog.base[String(flowerId)];
  const levelCd = catalog.cfg[String(level)];
  const cfg1 = catalog.cfg["1"];
  if (base > 0 && levelCd > 0 && cfg1 > 0) {
    return Math.round((base * levelCd) / cfg1);
  }
  return levelCd > 0 ? levelCd : 0;
}
