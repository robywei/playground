<script setup>
import { computed, inject, ref } from 'vue'
import * as api from '../api.js'
import { downloadCSV, fmt, today } from '../format.js'

const toast = inject('toast')
const shared = inject('shared')
const reload = inject('reload')

const form = ref(blank())
function blank() {
  return { name: '', brand: '', purchaseDate: today(), channel: '', price: 120, weightG: 1000 }
}

const query = ref({ from: '', to: '', q: '' })
const results = ref(null) // null = 尚未篩選，顯示全部

const rows = computed(() => results.value ?? shared.value.purchases)

// 既有材料名稱，供 datalist 自動完成
const names = computed(() => [...new Set(shared.value.purchases.map((p) => p.name))])

const unitCost = (p) => (p.weightG > 0 ? p.price / p.weightG : 0)

async function add() {
  const created = await toast.run(
    () => api.createPurchase({ ...form.value }),
    '進貨紀錄已新增',
  )
  if (!created) return
  form.value = blank()
  results.value = null
  await reload()
}

async function search() {
  const got = await toast.run(() =>
    api.listPurchases(query.value.from, query.value.to, query.value.q),
  )
  if (got) results.value = got
}

function clearSearch() {
  query.value = { from: '', to: '', q: '' }
  results.value = null
}

async function remove(p) {
  if (!confirm(`確定要刪除「${p.name}」這筆進貨紀錄嗎？\n刪除後庫存與成本會跟著改變。`)) return
  if (await toast.run(() => api.deletePurchase(p.id).then(() => true), '已刪除')) {
    results.value = null
    await reload()
  }
}

function exportCSV() {
  if (!rows.value.length) return toast.err('沒有可匯出的資料')
  downloadCSV(
    `BabyWei_採購明細_${today()}.csv`,
    ['採購日期', '材料名稱', '品牌', '購入管道', '進貨價格', '包裝重量(g)', '每克單位成本'],
    rows.value.map((p) => [
      p.purchaseDate, p.name, p.brand, p.channel,
      p.price, p.weightG, unitCost(p).toFixed(4),
    ]),
  )
}
</script>

<template>
  <div class="card">
    <h2>📝 新進貨紀錄</h2>
    <div class="notice">
      輸入材料的進貨價格、數量與購入管道，系統會自動算出「每克單位成本」。
      同一材料有多筆進貨時，成本採<strong>全期加權平均</strong>（總價 ÷ 總重）。
    </div>
    <div class="row">
      <div>
        <label>材料名稱</label>
        <input v-model.trim="form.name" list="ingredient-names" placeholder="例如：高筋麵粉">
        <datalist id="ingredient-names">
          <option v-for="n in names" :key="n" :value="n" />
        </datalist>
      </div>
      <div>
        <label>品牌 / 規格</label>
        <input v-model.trim="form.brand" placeholder="例如：水手牌 1kg">
      </div>
      <div>
        <label>購入日期</label>
        <input v-model="form.purchaseDate" type="date">
      </div>
      <div>
        <label>購入管道</label>
        <input v-model.trim="form.channel" placeholder="例如：烘焙材料行、PChome">
      </div>
      <div>
        <label>進貨總價格 ($)</label>
        <input v-model.number="form.price" type="number" min="0" step="0.01">
      </div>
      <div>
        <label>進貨總重量 (g)</label>
        <input v-model.number="form.weightG" type="number" min="0" step="0.1">
      </div>
    </div>
    <div class="toolbar">
      <button @click="add">➕ 新增進貨紀錄</button>
    </div>
  </div>

  <div class="card">
    <h2>🔍 採購明細查詢</h2>
    <div class="row">
      <div><label>起始日期</label><input v-model="query.from" type="date"></div>
      <div><label>結束日期</label><input v-model="query.to" type="date"></div>
      <div>
        <label>品項 / 關鍵字</label>
        <input v-model.trim="query.q" placeholder="材料名稱、品牌或管道…" @keyup.enter="search">
      </div>
    </div>
    <div class="toolbar">
      <button @click="search">🔍 篩選</button>
      <button class="secondary" @click="clearSearch">清除條件</button>
      <button class="secondary" @click="exportCSV">📊 匯出 CSV</button>
    </div>

    <p class="muted" style="margin-top:14px">
      共 {{ rows.length }} 筆{{ results ? '（已篩選）' : '' }}
    </p>
    <div class="tablewrap">
      <table>
        <thead>
          <tr>
            <th>採購日期</th><th>材料名稱</th><th>品牌</th><th>購入管道</th>
            <th>進貨價格</th><th>包裝重量(g)</th><th>每克成本</th><th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length">
            <td colspan="8" style="text-align:center;color:#8c8177">尚無進貨紀錄</td>
          </tr>
          <tr v-for="p in rows" :key="p.id">
            <td>{{ p.purchaseDate }}</td>
            <td><b>{{ p.name }}</b></td>
            <td>{{ p.brand || '—' }}</td>
            <td>{{ p.channel || '—' }}</td>
            <td>${{ fmt(p.price, 2) }}</td>
            <td>{{ fmt(p.weightG) }}</td>
            <td><b>${{ unitCost(p).toFixed(4) }}</b></td>
            <td><button class="danger" @click="remove(p)">刪</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
