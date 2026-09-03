package api

import (
	"io/fs"
	"net/http"
	"strings"

	"babywei-bakery/internal/store"
)

// Server 持有 handler 需要的相依。
type Server struct {
	db     *store.DB
	static fs.FS
}

// New 組出完整的 HTTP handler。
func New(db *store.DB, static fs.FS) http.Handler {
	s := &Server{db: db, static: static}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s.routeCRUD(mux)
	s.routeOps(mux)
	s.routeTransfer(mux)

	// 靜態檔與 SPA fallback。順序很重要：/api 前綴的未知路徑必須回 JSON
	// 404，否則前端只會拿到一頁 HTML 然後在 JSON.parse 爆掉，難以診斷。
	fileServer := http.FileServer(http.FS(static))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "找不到此 API 端點: "+r.URL.Path)
			return
		}
		if _, err := fs.Stat(static, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			// 不是實際檔案 → 交給前端路由
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	return mux
}
