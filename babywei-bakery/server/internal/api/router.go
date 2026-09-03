package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"babywei-bakery/internal/store"
)

// Server 持有 handler 需要的相依。
type Server struct {
	db     *store.DB
	static fs.FS
	// shutdown 由 main 注入。nil 表示此組態不支援從介面結束程式
	// （例如測試），此時 /api/shutdown 回 501。
	shutdown func()
}

// New 組出完整的 HTTP handler。shutdown 可為 nil。
func New(db *store.DB, static fs.FS, shutdown func()) http.Handler {
	s := &Server{db: db, static: static, shutdown: shutdown}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 結束程式。這是 .app 宣告 LSUIElement 之後唯一的關閉途徑 ——
	// 沒有 Dock 圖示可以按右鍵結束。
	//
	// 只收 POST：GET 會被瀏覽器的預先擷取、書籤檢查或使用者誤點連結觸發，
	// 那會在她不知情的狀況下把程式關掉。
	mux.HandleFunc("POST /api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if s.shutdown == nil {
			writeError(w, http.StatusNotImplemented, "此組態不支援從介面結束程式")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "shutting-down"})
		// 先讓回應送出去，再觸發關閉。同步呼叫會讓連線在回應寫完前就斷，
		// 前端只會看到網路錯誤，分不出「關掉了」還是「壞掉了」。
		go func() {
			time.Sleep(200 * time.Millisecond)
			s.shutdown()
		}()
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
