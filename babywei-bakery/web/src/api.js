// 前端唯一的 fetch 封裝層。其他檔案不得直接呼叫 fetch —— 錯誤處理、
// 空回應與查詢字串的規則只存在這裡一處。

async function request(path, { method = 'GET', body } = {}) {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return null

  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      throw new Error(`後端回應不是合法 JSON（HTTP ${res.status}）`)
    }
  }
  if (!res.ok) throw new Error(data?.error || `HTTP ${res.status}`)
  return data
}

const qs = (params) => {
  const s = new URLSearchParams(
    Object.entries(params).filter(([, v]) => v !== '' && v != null),
  ).toString()
  return s ? `?${s}` : ''
}

const enc = encodeURIComponent

// --- 進貨 ---
export const listPurchases = (from = '', to = '', q = '') =>
  request(`/api/purchases${qs({ from, to, q })}`)
export const createPurchase = (p) => request('/api/purchases', { method: 'POST', body: p })
export const updatePurchase = (p) =>
  request(`/api/purchases/${enc(p.id)}`, { method: 'PATCH', body: p })
export const deletePurchase = (id) =>
  request(`/api/purchases/${enc(id)}`, { method: 'DELETE' })

// --- 產品配方 ---
export const listDoughs = () => request('/api/doughs')
export const createDough = (d) => request('/api/doughs', { method: 'POST', body: d })
export const updateDough = (d) => request(`/api/doughs/${enc(d.id)}`, { method: 'PATCH', body: d })
export const deleteDough = (id) => request(`/api/doughs/${enc(id)}`, { method: 'DELETE' })

// --- 配料 ---
export const listFillings = () => request('/api/fillings')
export const createFilling = (f) => request('/api/fillings', { method: 'POST', body: f })
export const updateFilling = (f) =>
  request(`/api/fillings/${enc(f.id)}`, { method: 'PATCH', body: f })
export const deleteFilling = (id) => request(`/api/fillings/${enc(id)}`, { method: 'DELETE' })

// --- 商品 ---
export const listProducts = () => request('/api/products')
export const createProduct = (p) => request('/api/products', { method: 'POST', body: p })
export const updateProduct = (p) =>
  request(`/api/products/${enc(p.id)}`, { method: 'PATCH', body: p })
export const deleteProduct = (id) => request(`/api/products/${enc(id)}`, { method: 'DELETE' })

// --- 出貨 ---
export const listSales = (from = '', to = '') => request(`/api/sales${qs({ from, to })}`)
export const createSale = (saleDate, productId, qty) =>
  request('/api/sales', { method: 'POST', body: { saleDate, productId, qty } })
export const deleteSale = (id) => request(`/api/sales/${enc(id)}`, { method: 'DELETE' })

// --- 生產 ---
export const previewProduction = (productId, qty) =>
  request('/api/production/preview', { method: 'POST', body: { productId, qty } })
export const confirmProduction = (productId, qty, loggedDate = '') =>
  request('/api/production', { method: 'POST', body: { productId, qty, loggedDate } })

// --- 庫存與報表 ---
export const getInventory = () => request('/api/inventory')
export const getSummary = () => request('/api/reports/summary')
export const getSalesReport = (from = '', to = '') =>
  request(`/api/reports/sales${qs({ from, to })}`)

// --- 匯出與匯入 ---
export const exportBackup = () => request('/api/export/backup.json')
export const importLegacy = (snapshot) =>
  request('/api/import', { method: 'POST', body: snapshot })
