// Package api 提供 HTTP 路由與 handler，負責串接 store 與 domain。
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"babywei-bakery/internal/store"
)

// errorBody 是所有錯誤回應的統一格式。
type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// 此時 header 已送出，只能記錄。
		log.Printf("寫入 JSON 回應失敗: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// writeStoreError 把 store 的哨兵錯誤映射到適當的 HTTP 狀態碼。
// 外鍵約束失敗要回 409 而不是 500 —— 那是使用者可以理解並修正的狀況。
func writeStoreError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("%s: %v", fallback, err)
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// decodeJSON 讀取請求 body。失敗時直接寫出 400 並回傳 false。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "請求內容不是合法的 JSON: "+err.Error())
		return false
	}
	return true
}
