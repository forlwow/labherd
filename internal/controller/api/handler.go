// Package api 提供给前端的 HTTP JSON 接口。
//
// M1 先用标准库 net/http 手写 JSON，不使用 connect-go，原因见 docs/adr/0004-web-api-connect.md。
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/forlwow/labherd/internal/controller/registry"
)

// NodeLister 提供节点数据，由 registry.Registry 实现。
type NodeLister interface {
	List() []registry.Node
	Get(nodeID string) (registry.Node, bool)
}

// Handler 处理 /api/v1/ 下的请求。
type Handler struct {
	nodes NodeLister
	log   *slog.Logger
	mux   *http.ServeMux
}

// NewHandler 返回读取 nodes 的 Handler。
func NewHandler(nodes NodeLister, log *slog.Logger) *Handler {
	h := &Handler{nodes: nodes, log: log, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /api/v1/nodes", h.listNodes)
	h.mux.HandleFunc("GET /api/v1/nodes/{nodeID}", h.getNode)
	return h
}

// ServeHTTP 实现 http.Handler。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) listNodes(w http.ResponseWriter, _ *http.Request) {
	nodes := h.nodes.List()
	resp := nodeListResponse{Nodes: make([]nodeJSON, 0, len(nodes))}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, toNodeJSON(n))
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	n, ok := h.nodes.Get(r.PathValue("nodeID"))
	if !ok {
		h.writeJSON(w, http.StatusNotFound, errorResponse{Error: "node not found"})
		return
	}
	h.writeJSON(w, http.StatusOK, toNodeJSON(n))
}

// errorResponse 是出错时的响应体。
type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// 状态码已经发出，只能记录日志。通常是客户端提前断开。
		h.log.Warn("write json response failed", "err", err)
	}
}
