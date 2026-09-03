// Command bakery 是 BabyWei Bakery 的本地服務：起 HTTP 服務、開瀏覽器。
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
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

// version 由 build-release.sh 於建置時以 -ldflags -X 注入。
var version = "dev"

func main() {
	// 已經有一個實例在跑時，只把瀏覽器帶過去就好。
	// 使用者重複雙擊圖示是常態，開出第二個實例會讓兩個程序寫同一個
	// 資料庫檔，而且她分不出來哪個視窗是哪個。
	if url, ok := findRunningInstance(); ok {
		log.Printf("已有執行中的實例: %s", url)
		openBrowser(url)
		return
	}

	log.Printf("BabyWei Bakery %s", version)

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

	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}

	// 只關一次：中止訊號與介面上的「結束程式」都可能觸發。
	var once sync.Once
	shutdown := func(reason string) {
		once.Do(func() {
			log.Printf("%s，正在關閉…", reason)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("關閉服務時發生錯誤: %v", err)
			}
		})
	}

	srv.Handler = api.New(db, assets.FS(), func() { shutdown("收到介面的結束要求") })

	// 硬中斷雖然 SQLite 也能復原，但乾淨關閉才保證 data/ 只剩單一 .db 檔。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		shutdown("收到中止訊號")
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal("服務異常結束", err)
	}
	log.Print("已關閉")
}

// findRunningInstance 掃描本程式會用到的埠，找出已在執行的實例。
// 靠 /healthz 的回應內容辨識，避免把別的服務誤認成自己。
func findRunningInstance() (string, bool) {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	for i := range portTries {
		url := fmt.Sprintf("http://127.0.0.1:%d", basePort+i)
		resp, err := client.Get(url + "/healthz")
		if err != nil {
			continue
		}
		body := make([]byte, 64)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && bytes.Contains(body[:n], []byte(`"status":"ok"`)) {
			return url, true
		}
	}
	return "", false
}

// resolveDataDir 決定資料目錄。BAKERY_DATA_DIR 優先。
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
	return dataDirFor(exe), nil
}

// dataDirFor 由執行檔位置推導資料目錄。抽成純函數以便測試。
//
// 兩種佈局：
//
//	交付：BabyWei Bakery/BabyWei Bakery.app/Contents/MacOS/bakery
//	      → 往上四層 → BabyWei Bakery/data
//	開發：babywei-bakery/bin/bakery
//	      → 往上一層 → babywei-bakery/data
//
// 其他情況一律用執行檔所在目錄下的 data/。
func dataDirFor(exe string) string {
	dir := filepath.Dir(exe)
	switch {
	case filepath.Base(filepath.Dir(dir)) == "Contents":
		// .app bundle 內：MacOS → Contents → .app → 外層目錄
		dir = filepath.Dir(filepath.Dir(filepath.Dir(dir)))
	case filepath.Base(dir) == "bin":
		// 開發佈局：資料不該埋在 bin/ 裡，跟建置產物混在一起
		dir = filepath.Dir(dir)
	}
	return filepath.Join(dir, "data")
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
