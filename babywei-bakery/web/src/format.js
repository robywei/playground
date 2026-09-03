// 顯示用的格式化工具。

export const today = () => {
  const d = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

export const fmt = (n, digits = 1) =>
  Number(n || 0).toLocaleString('zh-TW', { maximumFractionDigits: digits })

export const money = (n) => `$${fmt(n, 0)}`

// 毛利率。售價為 0 時回傳 null，呼叫端顯示「—」而非 Infinity 或 NaN。
export const marginPct = (price, cost) => {
  if (!price || price <= 0) return null
  return ((price - cost) / price) * 100
}

// CSV 下載。BOM 不可省略 —— 少了它 Excel 開啟中文會亂碼。
export function downloadCSV(filename, header, rows) {
  const escape = (v) => {
    const s = String(v ?? '')
    return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s
  }
  const csv =
    '﻿' + [header, ...rows].map((r) => r.map(escape).join(',')).join('\n')
  downloadBlob(filename, new Blob([csv], { type: 'text/csv;charset=utf-8;' }))
}

export function downloadJSON(filename, data) {
  downloadBlob(
    filename,
    new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }),
  )
}

function downloadBlob(filename, blob) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
