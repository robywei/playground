package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type httpRecorder = httptest.ResponseRecorder

// doRaw 送出未經 marshal 的原始 body，供測試貼近真實輸入的 JSON。
func doRaw(t *testing.T, h http.Handler, method, path, body string) *httpRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeIntoRaw(t *testing.T, rec *httpRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("解析回應失敗: %v\nbody: %s", err, rec.Body.String())
	}
}
