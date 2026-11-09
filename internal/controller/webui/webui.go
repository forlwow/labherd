// Package webui 提供前端页面：静态文件，以及单页应用的路由回退。
package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// placeholder 是未嵌入前端时返回的页面。
const placeholder = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>LabHerd</title></head>
<body style="font-family:sans-serif;max-width:40rem;margin:4rem auto;line-height:1.6">
<h1>LabHerd</h1>
<p>这个 Controller 构建时没有嵌入前端页面。接口仍然可用：<a href="/api/v1/nodes">/api/v1/nodes</a>。</p>
<p>需要页面时，先运行 <code>make web-build</code>，再运行 <code>make build</code>。</p>
</body></html>
`

// Handler 返回提供前端页面的 http.Handler。
//
// 存在的文件直接返回；不带扩展名的路径（如 /nodes/gpu-01）返回 index.html，交给前端路由；
// 带扩展名但不存在的路径返回 404，避免把缺失的脚本当成 HTML。
// ok 为 false 时表示没有嵌入前端，所有路径返回说明页。
func Handler(fsys fs.FS, ok bool) http.Handler {
	if !ok {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(placeholder))
		})
	}
	return &spaHandler{fsys: fsys}
}

type spaHandler struct {
	fsys fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name != "" && isFile(h.fsys, name) {
		// Vite 输出的 assets/ 文件名带内容哈希，可以长期缓存。
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFileFS(w, r, h.fsys, name)
		return
	}
	if path.Ext(name) != "" {
		http.NotFound(w, r)
		return
	}

	// index.html 引用的资源名会随每次构建变化，不能缓存。
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, h.fsys, "index.html")
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
