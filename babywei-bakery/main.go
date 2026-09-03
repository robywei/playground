// Command bakery 是 BabyWei Bakery 的本地服務：起 HTTP 服務、開瀏覽器。
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"babywei-bakery/internal/api"
	"babywei-bakery/internal/assets"
	"babywei-bakery/internal/store"
)

// basePort 是預設埠。被占用時往後試，最多 portTries 個。
const (
	basePort  = 8787
	portTries = 10
)

func main() {
	dataDir, err := resolveDataDir()
	if err != nil {
		fatal("無法決定資料目錄", err)
	}
	log.Printf("資料目錄: %s", dataDir)

	db, err := store.Open(dataDir)
	if err != nil {
		fatal("無法開啟資料庫", err)
	}
	defer db.Close()

	ln, err := listen()
	if err != nil {
		fatal("無法開啟服務埠", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	log.Printf("服務已啟動: %s", url)

	go openBrowser(url)

	srv := &http.Server{
		Handler:           api.New(db, assets.FS()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal("服務異常結束", err)
	}
}

// resolveDataDir 決定資料目錄。
//
// BAKERY_DATA_DIR 優先 —— 開發時 binary 不在 .app 內，路徑推導不成立。
// 否則由執行檔位置向上解析四層取得 "BabyWei Bakery/"：
//
//	BabyWei Bakery/BabyWei Bakery.app/Contents/MacOS/bakery
//	→ MacOS → Contents → .app → BabyWei Bakery/
func resolveDataDir() (string, error) {
	if dir := os.Getenv("BAKERY_DATA_DIR"); dir != "" {
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("取得執行檔路徑: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("解析執行檔路徑: %w", err)
	}
	dir := filepath.Dir(exe)
	if filepath.Base(filepath.Dir(dir)) == "Contents" {
		// 在 .app bundle 內：再往上三層到 BabyWei Bakery/
		dir = filepath.Dir(filepath.Dir(filepath.Dir(dir)))
	}
	return filepath.Join(dir, "data"), nil
}

// listen 從 basePort 起往後找一個可用的埠。
func listen() (net.Listener, error) {
	var lastErr error
	for i := range portTries {
		addr := fmt.Sprintf("127.0.0.1:%d", basePort+i)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("連續 %d 個埠都無法使用（自 %d 起）: %w", portTries, basePort, lastErr)
}

// openBrowser 開啟瀏覽器。失敗不影響服務 —— 使用者仍可自行輸入網址。
func openBrowser(url string) {
	// 等服務真的能接受連線再開，否則瀏覽器會看到連線被拒。
	for range 50 {
		if c, err := net.DialTimeout("tcp", url[len("http://"):], 100*time.Millisecond); err == nil {
			c.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := exec.Command("open", url).Run(); err != nil {
		log.Printf("無法自動開啟瀏覽器（請手動前往 %s）: %v", url, err)
	}
}

func fatal(msg string, err error) {
	log.Fatalf("✗ %s: %v", msg, err)
}
