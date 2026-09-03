package main

import (
	"path/filepath"
	"testing"
)

func TestDataDirFor(t *testing.T) {
	cases := []struct {
		name string
		exe  string
		want string
	}{
		{
			name: "交付佈局：.app bundle 內",
			exe:  "/Users/x/BabyWei Bakery/BabyWei Bakery.app/Contents/MacOS/bakery",
			want: "/Users/x/BabyWei Bakery/data",
		},
		{
			name: "開發佈局：bin/ 下",
			exe:  "/Users/x/playground/babywei-bakery/bin/bakery",
			want: "/Users/x/playground/babywei-bakery/data",
		},
		{
			name: "其他：執行檔同層",
			exe:  "/tmp/bakery",
			want: "/tmp/data",
		},
		{
			name: "bin 的父層是 Contents 時，.app 規則優先",
			exe:  "/Users/x/A.app/Contents/MacOS/bakery",
			want: "/Users/x/data",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dataDirFor(c.exe); got != filepath.Clean(c.want) {
				t.Errorf("dataDirFor(%q) = %q, want %q", c.exe, got, c.want)
			}
		})
	}
}
