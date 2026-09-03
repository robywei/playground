<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import * as api from '../api.js'
import { downloadCSV, downloadJSON, fmt, money, today } from '../format.js'

const toast = inject('toast')
const shared = inject('shared')
const reload = inject('reload')

const summary = ref(null)
const sale = ref({ saleDate: today(), productId: '', qty: 1 })
const query = ref({ from: '', to: '' })
const report = ref(null)
const importText = ref('')
const importResult = ref(null)

const products = computed(() => shared.value.products)

const BUCKETS = [
  { key: 'day', label: '本日' },
  { key: 'month', label: '本月' },
  { key: 'quarter', label: '本季' },
  { key: 'year', label: '本年度' },
]

async function loadSummary() {
  const got = await toast.run(() => api.getSummary())
  if (got) summary.value = got
}
onMounted(async () => {
  await loadSummary()
  await runQuery()
})

async function addSale() {
  if (!sale.value.productId) return toast.err('請選擇出貨商品')
  if (!(sale.value.qty > 0)) return toast.err('出貨數量必須大於 0')
  const done = await toast.run(
    () => api.createSale(sale.value.saleDate, sale.value.productId, Number(sale.value.qty)),
    '出貨紀錄已新增',
  )
  if (!done) return
  sale.value.qty = 1
  await loadSummary()
  await runQuery()
}

async function runQuery() {
  const got = await toast.run(() => api.getSalesReport(query.value.from, query.value.to))
  if (got) report.value = got
}

async function removeSale(s) {
  if (!confirm(`確定要刪除 ${s.saleDate}「${s.productName}」× ${s.qty} 的出貨紀錄嗎？`)) return
  if (await toast.run(() => api.deleteSale(s.id).then(() => true), '已刪除')) {
    await loadSummary()
    await runQuery()
  }
}

function exportSalesCSV() {
  if (!report.value?.sales.length) return toast.err('沒有可匯出的資料')
  downloadCSV(
    `BabyWei_銷售報表_${today()}.csv`,
    ['出貨日期', '商品名稱', '數量', '單顆成本', '單顆售價', '總營收', '總利潤'],
    report.value.sales.map((s) => [
      s.saleDate, s.productName, s.qty,
      s.unitCost.toFixed(2), s.unitPrice,
      (s.qty * s.unitPrice).toFixed(2),
      (s.qty * (s.unitPrice - s.unitCost)).toFixed(2),
    ]),
  )
}

async function backup() {
  const snap = await toast.run(() => api.exportBackup(), '備份已下載')
  if (snap) downloadJSON(`babywei-backup-${today()}.json`, snap)
}

async function runImport() {
  let parsed
  try {
    parsed = JSON.parse(importText.value)
  } catch (e) {
    return toast.err(`貼上的內容不是合法 JSON：${e.message}`)
  }
  if (!confirm(
    '⚠️ 匯入會清空目前所有資料再寫入貼上的內容，無法復原。\n\n' +
    '（系統會在匯入前自動存一份備份到 data/backups/）\n\n確定要繼續嗎？',
  )) return

  const rep = await toast.run(() => api.importLegacy(parsed), '匯入完成')
  if (!rep) return
  importResult.value = rep
  importText.value = ''
  await reload()
  await loadSummary()
  await runQuery()
}
</script>

<template>
  <div class="card">
    <h2>📈 財務與利潤總覽</h2>
    <div class="metrics" v-if="summary">
      <div v-for="b in BUCKETS" :key="b.key" class="metric">
        <span>{{ b.label }}利潤</span>
        <b :class="summary[b.key].profitTwd >= 0 ? 'profit-good' : 'profit-warn'">
          {{ money(summary[b.key].profitTwd) }}
        </b>
        <span style="font-size:12px">營收 {{ money(summary[b.key].revenueTwd) }}</span>
      </div>
    </div>
    <p v-else class="muted">載入中…</p>
    <div class="toolbar" style="margin-top:0">
      <button class="secondary" @click="loadSummary">🔄 重新整理</button>
    </div>
  </div>

  <div class="card">
    <h2>📝 新增出貨紀錄</h2>
    <div class="notice">
      出貨當下會把<strong>單顆成本與售價寫死成快照</strong>。
      日後調整售價或進貨價，都不會改變已成立的紀錄。
    </div>
    <div class="row">
      <div><label>出貨日期</label><input v-model="sale.saleDate" type="date"></div>
      <div>
        <label>出貨商品</label>
        <select v-model="sale.productId">
          <option value="">— 請選擇 —</option>
          <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </div>
      <div><label>出貨數量</label><input v-model.number="sale.qty" type="number" min="1"></div>
    </div>
    <div class="toolbar"><button @click="addSale">➕ 加入紀錄</button></div>
  </div>

  <div class="card">
    <h2>🔍 歷史出貨查詢</h2>
    <div class="row">
      <div><label>起始日期</label><input v-model="query.from" type="date"></div>
      <div><label>結束日期</label><input v-model="query.to" type="date"></div>
    </div>
    <div class="toolbar">
      <button @click="runQuery">🔍 查詢</button>
      <button class="secondary" @click="query = { from: '', to: '' }; runQuery()">清除條件</button>
      <button class="secondary" @click="exportSalesCSV">📊 匯出 CSV</button>
    </div>

    <template v-if="report">
      <h3>
        共 {{ report.sales.length }} 筆　
        總營收 {{ money(report.totals.revenueTwd) }}　
        總成本 {{ money(report.totals.costTwd) }}　
        毛利
        <span :class="report.totals.profitTwd >= 0 ? 'profit-good' : 'profit-warn'">
          {{ money(report.totals.profitTwd) }}
        </span>
      </h3>
      <div class="tablewrap">
        <table>
          <thead>
            <tr>
              <th>日期</th><th>商品</th><th>數量</th><th>單顆成本</th>
              <th>單顆售價</th><th>總營收</th><th>總利潤</th><th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!report.sales.length">
              <td colspan="8" style="text-align:center;color:#8c8177">此區間沒有出貨紀錄</td>
            </tr>
            <tr v-for="s in report.sales" :key="s.id">
              <td>{{ s.saleDate }}</td>
              <td><b>{{ s.productName }}</b></td>
              <td>{{ s.qty }}</td>
              <td>${{ fmt(s.unitCost, 2) }}</td>
              <td>${{ fmt(s.unitPrice, 0) }}</td>
              <td>{{ money(s.qty * s.unitPrice) }}</td>
              <td :class="s.unitPrice >= s.unitCost ? 'profit-good' : 'profit-warn'">
                <b>{{ money(s.qty * (s.unitPrice - s.unitCost)) }}</b>
              </td>
              <td><button class="danger" @click="removeSale(s)">刪</button></td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>

  <div class="card">
    <h2>💾 備份與資料轉移</h2>
    <div class="notice">
      資料存在這台電腦的 <code>data/babywei.db</code>。每次啟動程式都會自動存一份
      快照到 <code>data/backups/</code>（保留最近 30 份）。
      要整包帶走，複製整個資料夾即可。
    </div>
    <div class="toolbar" style="margin-top:0">
      <button class="secondary" @click="backup">⬇️ 下載完整備份（JSON）</button>
    </div>

    <h3 style="margin-top:24px">匯入舊版資料</h3>
    <div class="notice warn">
      ⚠️ 匯入會<strong>清空目前所有資料</strong>再寫入，無法復原。
      系統會在匯入前自動存一份備份。<br>
      舊版的生產紀錄沒有原料消耗明細，匯入時會用貼上資料中的配方回推 ——
      這是近似值，還原不了當時的真實消耗。
    </div>
    <textarea
      v-model="importText"
      placeholder="貼上舊版 localStorage 的 babywei_local 內容（JSON）…"
    ></textarea>
    <div class="toolbar">
      <button class="danger" :disabled="!importText.trim()" @click="runImport">
        ⚠️ 清空並匯入
      </button>
    </div>

    <div v-if="importResult" style="margin-top:16px">
      <h3>匯入結果</h3>
      <p class="muted">
        <span v-for="(v, k) in importResult.counts" :key="k">{{ k }}: {{ v }}　</span>
      </p>
      <div v-if="importResult.warnings.length" class="notice warn">
        <div v-for="(w, i) in importResult.warnings" :key="i">• {{ w }}</div>
      </div>
    </div>
  </div>
</template>
