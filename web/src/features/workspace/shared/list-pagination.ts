export type ListPageWindow = {
  pageIndex: number;
  loadedPages: number;
  start: number;
  end: number;
};

export function resolveListPageWindow(
  requestedPage: number,
  itemCount: number,
  pageSize: number,
): ListPageWindow {
  if (!Number.isInteger(pageSize) || pageSize <= 0) {
    throw new Error("pageSize must be a positive integer");
  }
  const loadedPages = Math.max(1, Math.ceil(itemCount / pageSize));
  const pageIndex = Math.min(
    Math.max(0, Math.trunc(requestedPage)),
    loadedPages - 1,
  );
  const start = pageIndex * pageSize;
  return {
    pageIndex,
    loadedPages,
    start,
    end: Math.min(itemCount, start + pageSize),
  };
}
