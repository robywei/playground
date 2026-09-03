import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  // 產出直接落在 Go 的 embed 目錄。go:embed 不能跨越 ..，
  // 所以產物必須實際位於 internal/assets/ 之下。
  build: {
    outDir: '../internal/assets/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    // 開發時前端在 5173、後端在 8787。/api 一律代理過去，
    // 讓 api.js 用相對路徑即可，dev 與 production 共用同一份程式碼。
    proxy: {
      '/api': { target: 'http://127.0.0.1:8787', changeOrigin: true },
    },
  },
})
