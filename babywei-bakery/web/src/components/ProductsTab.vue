<script setup>
import { computed, inject, ref } from 'vue'
import * as api from '../api.js'
import { fmt, marginPct, money } from '../format.js'

const toast = inject('toast')
const shared = inject('shared')
const reload = inject('reload')

const editing = ref(blank())
function blank() {
  return {
    id: '', name: '', price: 150,
    doughId: '', doughWeightG: 450,
    fill1Id: '', fill1WeightG: 0,
    fill2Id: '', fill2WeightG: 0,
  }
}
const isNew = computed(() => !editing.value.id)

const doughs = computed(() => shared.value.doughs)
const fillings = computed(() => shared.value.fillings)
const products = computed(() => shared.value.products)

// 後端已算好成本與明細，前端不重算 —— 計算邏輯只存在 domain 一處。
const current = computed(() => products.value.find((p) => p.id === editing.value.id))
const unitCost = computed(() => current.value?.unitCostTwd ?? null)
const breakdown = computed(() => current.value?.breakdown ?? [])
const margin = computed(() =>
  unitCost.value == null ? null : marginPct(editing.value.price, unitCost.value),
)

const unitWeight = computed(() =>
  Number(editing.value.doughWeightG || 0) +
  Number(editing.value.fill1WeightG || 0) +
  Number(editing.value.fill2WeightG || 0),
)

function load(id) {
  const found = products.value.find((p) => p.id === id)
  if (!found) return
  editing.value = {
    id: found.id, name: found.name, price: found.price,
    doughId: found.doughId, doughWeightG: found.doughWeightG,
    fill1Id: found.fill1Id, fill1WeightG: found.fill1WeightG,
    fill2Id: found.fill2Id, fill2WeightG: found.fill2WeightG,
  }
}

// 前端驗證只是體驗，後端仍會擋 —— 兩邊都要有。
function validate() {
  const e = editing.value
  if (!e.name) return '請輸入商品名稱'
  if (!e.doughId) return '請選擇產品配方'
  if (!(e.doughWeightG > 0)) return '配方使用重量必須大於 0'
  if (e.fill1Id && !(e.fill1WeightG > 0)) return '已選擇配料 1，請填寫重量'
  if (e.fill2Id && !(e.fill2WeightG > 0)) return '已選擇配料 2，請填寫重量'
  return ''
}

async function save() {
  const msg = validate()
  if (msg) return toast.err(msg)

  const payload = { ...editing.value }
  if (!payload.fill1Id) payload.fill1WeightG = 0
  if (!payload.fill2Id) payload.fill2WeightG = 0

  const saved = await toast.run(
    () => (isNew.value ? api.createProduct(payload) : api.updateProduct(payload)),
    isNew.value ? '商品已新增' : '商品已儲存',
  )
  if (!saved) return
  editing.value = { ...payload, id: saved.id }
  await reload()
}

async function remove() {
  if (isNew.value) return
  if (!confirm(
    `確定要刪除「${editing.value.name}」嗎？\n\n` +
    '已成立的出貨與生產紀錄會保留（名稱與金額都是當時的快照），報表數字不會改變。',
  )) return
  const done = await toast.run(() => api.deleteProduct(editing.value.id).then(() => true), '商品已刪除')
  if (!done) return
  editing.value = blank()
  await reload()
}
</script>

<template>
  <div class="card">
    <div class="search-box">
      <label>🔍 載入已建立的商品</label>
      <select :value="editing.id" @change="load($event.target.value)">
        <option value="">— 選擇要編輯的商品 —</option>
        <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </div>

    <h2>{{ isNew ? '新增' : '編輯' }}商品</h2>
    <div class="row">
      <div>
        <label>商品名稱</label>
        <input v-model.trim="editing.name" placeholder="例如：南瓜藜麥吐司">
      </div>
      <div>
        <label>預計售價（單顆 / 元）</label>
        <input v-model.number="editing.price" type="number" min="0" step="1">
      </div>
    </div>

    <h3>產品配方（必選）</h3>
    <div class="row">
      <div>
        <label>使用的配方</label>
        <select v-model="editing.doughId">
          <option value="">— 請選擇 —</option>
          <option v-for="d in doughs" :key="d.id" :value="d.id">{{ d.name }}</option>
        </select>
      </div>
      <div>
        <label>每顆使用重量（g）</label>
        <input v-model.number="editing.doughWeightG" type="number" min="0" step="0.1">
      </div>
    </div>

    <h3>配料（選填）</h3>
    <div class="row">
      <div>
        <label>配料 1</label>
        <select v-model="editing.fill1Id">
          <option value="">— 不使用 —</option>
          <option v-for="f in fillings" :key="f.id" :value="f.id">{{ f.name }}</option>
        </select>
      </div>
      <div>
        <label>配料 1 重量（g）</label>
        <input v-model.number="editing.fill1WeightG" type="number" min="0" step="0.1" :disabled="!editing.fill1Id">
      </div>
      <div>
        <label>配料 2</label>
        <select v-model="editing.fill2Id">
          <option value="">— 不使用 —</option>
          <option v-for="f in fillings" :key="f.id" :value="f.id">{{ f.name }}</option>
        </select>
      </div>
      <div>
        <label>配料 2 重量（g）</label>
        <input v-model.number="editing.fill2WeightG" type="number" min="0" step="0.1" :disabled="!editing.fill2Id">
      </div>
    </div>

    <div class="metrics">
      <div class="metric"><span>單顆總重</span><b>{{ fmt(unitWeight) }} g</b></div>
      <div class="metric">
        <span>單顆成本</span>
        <b>{{ unitCost == null ? '—' : money(unitCost) }}</b>
      </div>
      <div class="metric"><span>預計售價</span><b>{{ money(editing.price) }}</b></div>
      <div class="metric">
        <span>毛利率</span>
        <b :class="margin == null ? '' : margin >= 0 ? 'profit-good' : 'profit-warn'">
          {{ margin == null ? '—' : `${fmt(margin)}%` }}
        </b>
      </div>
    </div>
    <p v-if="isNew" class="muted">儲存後才會顯示成本與毛利率。</p>

    <div v-if="breakdown.length">
      <h3>成本明細（單顆）</h3>
      <div class="tablewrap">
        <table>
          <thead>
            <tr><th>材料</th><th>用量(g)</th><th>每克成本</th><th>成本</th></tr>
          </thead>
          <tbody>
            <tr v-for="b in breakdown" :key="b.ingredientName">
              <td>{{ b.ingredientName }}</td>
              <td>{{ fmt(b.gramsPerUnit) }}</td>
              <td>${{ b.costPerGram.toFixed(4) }}</td>
              <td><b>${{ fmt(b.costTwd, 2) }}</b></td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="breakdown.some((b) => b.costPerGram === 0)" class="notice warn" style="margin-top:12px">
        有材料的每克成本是 0 —— 代表它還沒有任何進貨紀錄，成本會被低估。
      </p>
    </div>

    <div class="toolbar">
      <button @click="save">💾 {{ isNew ? '新增商品' : '儲存商品' }}</button>
      <button class="danger" :disabled="isNew" @click="remove">🗑 刪除此商品</button>
      <button class="secondary" @click="editing = blank()">✨ 清空重新輸入</button>
    </div>
  </div>
</template>
