import { ref } from 'vue'

// 全域訊息佇列。錯誤要讓使用者看得見 —— 靜默失敗會讓她以為存檔成功。
const messages = ref([])
let seq = 0

function push(text, kind) {
  const id = ++seq
  messages.value.push({ id, text, kind })
  setTimeout(() => {
    messages.value = messages.value.filter((m) => m.id !== id)
  }, kind === 'err' ? 8000 : 3500)
}

export function useToast() {
  return {
    messages,
    ok: (text) => push(text, 'ok'),
    err: (text) => push(text, 'err'),
    // run 包住任何 async 操作：成功顯示訊息，失敗顯示後端的錯誤內容。
    async run(fn, okText) {
      try {
        const result = await fn()
        if (okText) push(okText, 'ok')
        return result
      } catch (e) {
        push(e.message || String(e), 'err')
        return undefined
      }
    },
  }
}
