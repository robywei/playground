// Package assets 持有編進 binary 的前端產出。
package assets

import (
	"embed"
	"io/fs"
)

// dist 由 web/ 的 `vite build` 產生。用 all: 前綴才會包含以 . 或 _
// 開頭的檔案。
//
// repo 保留一份佔位 dist/index.html，否則乾淨 clone 會在編譯期就失敗
// —— go:embed 找不到目標目錄不是執行期錯誤，是編譯錯誤。
//
//go:embed all:dist
var dist embed.FS

// FS 回傳以 dist/ 為根的檔案系統。
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // 建置期就該發現
	}
	return sub
}
