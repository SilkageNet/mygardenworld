package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:static
var staticFS embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	return &spaHandler{
		fsys: sub,
	}
}

type spaHandler struct {
	fsys fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "." || name == "" {
		h.serveIndex(w, r)
		return
	}
	if h.exists(name) {
		h.serveFile(w, r, name)
		return
	}
	if h.exists(path.Join(name, "index.html")) {
		h.serveFile(w, r, path.Join(name, "index.html"))
		return
	}
	if h.exists(name + ".html") {
		h.serveFile(w, r, name+".html")
		return
	}
	h.serveIndex(w, r)
}

func (h *spaHandler) exists(name string) bool {
	info, err := fs.Stat(h.fsys, name)
	return err == nil && !info.IsDir()
}

func (h *spaHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(h.fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if h.exists("index.html") {
		h.serveFile(w, r, "index.html")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>mygardenworld</title></head><body><main style="font-family:sans-serif;max-width:720px;margin:48px auto;line-height:1.6"><h1>当前构建未内嵌完整 Web UI</h1><p>本地 <code>go build</code> / <code>go install</code> 不强制编译前端。请使用 release 二进制，或在发布构建中先执行前端构建并将 <code>web/out</code> 复制到 <code>internal/webui/static</code> 后再编译 <code>gardend</code>。</p></main></body></html>`))
}
