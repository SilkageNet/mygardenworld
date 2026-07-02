import { createServer } from "node:http";
import { createReadStream, promises as fs } from "node:fs";
import path from "node:path";
import { Readable } from "node:stream";

const root = path.resolve(
  process.env.GARDEN_LOCAL_ROOT ||
    "C:/Users/93541/AppData/Local/Temp/garden-sea666-local",
);
const upstream = new URL(process.env.GARDEN_UPSTREAM || "https://garden.sea666.cn");
const port = Number(process.env.PORT || 18080);
const host = process.env.HOST || "127.0.0.1";

const mimeTypes = new Map([
  [".html", "text/html; charset=utf-8"],
  [".js", "application/javascript; charset=utf-8"],
  [".css", "text/css; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".png", "image/png"],
  [".jpg", "image/jpeg"],
  [".jpeg", "image/jpeg"],
  [".webp", "image/webp"],
  [".ico", "image/x-icon"],
  [".svg", "image/svg+xml"],
  [".map", "application/json; charset=utf-8"],
]);

const hopByHopHeaders = new Set([
  "connection",
  "content-length",
  "host",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);

function shouldProxy(pathname) {
  return (
    pathname.startsWith("/api/") ||
    pathname.startsWith("/cdn/") ||
    pathname.startsWith("/static/media/")
  );
}

function safeLocalPath(pathname) {
  const decoded = decodeURIComponent(pathname);
  const relative = decoded === "/" ? "index.html" : decoded.replace(/^\/+/, "");
  const file = path.resolve(root, relative);
  if (file !== root && !file.startsWith(root + path.sep)) {
    return null;
  }
  return file;
}

async function serveFile(res, file) {
  const stat = await fs.stat(file);
  if (!stat.isFile()) return false;

  res.writeHead(200, {
    "content-type": mimeTypes.get(path.extname(file).toLowerCase()) || "application/octet-stream",
    "content-length": String(stat.size),
    "cache-control": "no-store",
  });
  createReadStream(file).pipe(res);
  return true;
}

async function proxyRequest(req, res, parsedURL) {
  const target = new URL(parsedURL.pathname + parsedURL.search, upstream);
  const headers = new Headers();

  for (const [name, value] of Object.entries(req.headers)) {
    const lower = name.toLowerCase();
    if (hopByHopHeaders.has(lower)) continue;
    if (value == null) continue;
    headers.set(name, Array.isArray(value) ? value.join(", ") : value);
  }

  headers.set("accept-encoding", "identity");
  headers.set("origin", upstream.origin);
  headers.set("referer", `${upstream.origin}/`);

  const hasBody = !["GET", "HEAD"].includes(req.method || "GET");
  const response = await fetch(target, {
    method: req.method,
    headers,
    body: hasBody ? req : undefined,
    duplex: hasBody ? "half" : undefined,
    redirect: "manual",
  });

  const outHeaders = {};
  response.headers.forEach((value, name) => {
    const lower = name.toLowerCase();
    if (hopByHopHeaders.has(lower)) return;
    if (lower === "content-encoding") return;
    if (lower === "content-length") return;
    outHeaders[name] = value;
  });
  outHeaders["cache-control"] = "no-store";

  res.writeHead(response.status, outHeaders);
  if (req.method === "HEAD" || !response.body) {
    res.end();
    return;
  }
  Readable.fromWeb(response.body).pipe(res);
}

const server = createServer(async (req, res) => {
  try {
    const parsedURL = new URL(req.url || "/", `http://${req.headers.host || `${host}:${port}`}`);

    if (shouldProxy(parsedURL.pathname)) {
      await proxyRequest(req, res, parsedURL);
      return;
    }

    const file = safeLocalPath(parsedURL.pathname);
    if (file) {
      try {
        if (await serveFile(res, file)) return;
      } catch (error) {
        if (error?.code !== "ENOENT") throw error;
      }
    }

    const fallback = path.join(root, "index.html");
    if (await serveFile(res, fallback)) return;

    res.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
    res.end("Not found");
  } catch (error) {
    console.error(`[proxy] ${req.method} ${req.url}`, error);
    res.writeHead(502, { "content-type": "text/plain; charset=utf-8" });
    res.end(`Proxy error: ${error?.message || error}`);
  }
});

server.listen(port, host, () => {
  console.log(`Serving ${root}`);
  console.log(`Proxying /api, /cdn, /static/media to ${upstream.origin}`);
  console.log(`Open http://${host}:${port}/#/login`);
});
