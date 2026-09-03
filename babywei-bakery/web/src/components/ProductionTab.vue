<script setup>
import { computed, inject, ref, watch } from 'vue'
import * as api from '../api.js'
import { fmt, today } from '../format.js'

const toast = inject('toast')
const shared = inject('shared')
const reload = inject('reload')

const productId = ref('')
const qty = ref(10)
const scaled = ref(null)
const confirming = ref(false)

const products = computed(() => shared.value.products)
const selected = computed(() => products.value.find((p) => p.id === productId.value))

// 每次改選商品或數量就重新換算。preview 不寫入任何資料。
watch([productId, qty], async () => {
  scaled.value = null
  if (!productId.value || !(qty.value > 0)) return
  const got = await toast.run(() => api.previewProduction(productId.value, Number(qty.value)))
  if (got) scaled.value = got
}, { immediate: false })

async function confirm() {
  if (!scaled.value) return
  if (!window.confirm(
    `確認生產「${selected.value.name}」× ${qty.value} 個？\n\n` +
    '系統會依上方換算結果扣除原料庫存，並把消耗量寫死成紀錄。\n' +
    '這個動作無法在畫面上撤銷。',
  )) return

  confirming.value = true
  const done = await toast.run(
    () => api.confirmProduction(productId.value, Number(qty.value)),
    '已完成生產，庫存已扣除',
  )
  confirming.value = false
  if (done) await reload() // 庫存 tab 下次開啟會重新載入
}

function printSheet() {
  if (!scaled.value) return
  const s = scaled.value
  const sections = s.sections.map((sec) => `
    <h2>${escapeHTML(sec.title)}</h2>
    <table>
      <thead><tr><th>材料</th><th>原比例</th><th>本批用量</th></tr></thead>
      <tbody>${sec.items.map((it) => `
        <tr>
          <td>${escapeHTML(it.name)}</td>
          <td>${fmt(it.ratio)}${sec.unit === 'pct' ? '%' : 'g'}</td>
          <td><b>${fmt(it.usageG)} g</b></td>
        </tr>`).join('')}
      </tbody>
    </table>`).join('')

  const w = window.open('', '_blank')
  if (!w) return toast.err('瀏覽器封鎖了新視窗，請允許彈出視窗後再試')
  w.document.write(`<!doctype html><html lang="zh-Hant"><head><meta charset="utf-8">
    <title>生產表 ${escapeHTML(selected.value.name)}</title>
    <style>
      @page { size: A4; margin: 15mm; }
      body { font-family: -apple-system, "Noto Sans TC", sans-serif; color: #333; }
      h1 { font-size: 20px; margin: 0 0 4px; }
      h2 { font-size: 15px; margin: 20px 0 6px; }
      .meta { color: #777; font-size: 13px; margin-bottom: 6px; }
      table { width: 100%; border-collapse: collapse; }
      th, td { padding: 7px 6px; border-bottom: 1px solid #ddd; text-align: right; font-size: 14px; }
      th:first-child, td:first-child { text-align: left; }
    </style></head><body>
    <h1>${escapeHTML(selected.value.name)} × ${s.qty} 個</h1>
    <p class="meta">${today()}　單顆總重 ${fmt(s.unitWeightG)}g　
       配方總重 ${fmt(s.doughTotalG)}g　配料總重 ${fmt(s.fillTotalG)}g</p>
    ${sections}
    </body></html>`)
  w.document.close()
  w.focus()
  setTimeout(() => w.print(), 400)
}

function escapeHTML(s) {
  return String(s ?? '').replace(/[&<>"']/g, (m) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[m])
}
</script>

<template>
  <div class="card">
    <h2>🧮 廚房生產配方表</h2>
    <div class="notice">
      先算出今天要製作的用量。<strong>換算階段不會動到庫存</strong> ——
      按下「確認完成生產」才會扣除，並把消耗量寫死成紀錄。
    </div>

    <div class="row">
      <div>
        <label>選擇商品</label>
        <select v-model="productId">
          <option value="">— 請選擇 —</option>
          <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </div>
      <div>
        <label>生產數量（個）</label>
        <input v-model.number="qty" type="number" min="1" step="1">
      </div>
    </div>

    <div v-if="scaled">
      <div class="metrics">
        <div class="metric"><span>單顆總重</span><b>{{ fmt(scaled.unitWeightG) }} g</b></div>
        <div class="metric"><span>本批配方總重</span><b>{{ fmt(scaled.doughTotalG) }} g</b></div>
        <div class="metric"><span>本批配料總重</span><b>{{ fmt(scaled.fillTotalG) }} g</b></div>
        <div class="metric"><span>生產數量</span><b>{{ scaled.qty }} 個</b></div>
      </div>

      <div class="grid">
        <div v-for="sec in scaled.sections" :key="sec.title">
          <h3>{{ sec.title }}</h3>
          <div class="tablewrap">
            <table>
              <thead><tr><th>材料</th><th>原比例</th><th>本批用量</th></tr></thead>
              <tbody>
                <tr v-for="it in sec.items" :key="it.name">
                  <td>{{ it.name }}</td>
                  <td>{{ fmt(it.ratio) }}{{ sec.unit === 'pct' ? '%' : 'g' }}</td>
                  <td><b>{{ fmt(it.usageG) }} g</b></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="toolbar">
        <button class="secondary" @click="printSheet">📄 產生 A4 生產表</button>
        <button :disabled="confirming" @click="confirm">
          ✅ 確認完成生產並扣除庫存
        </button>
      </div>
    </div>

    <p v-else-if="productId" class="muted" style="margin-top:16px">換算中…</p>
    <p v-else class="muted" style="margin-top:16px">請先選擇商品。</p>
  </div>
</template>
