<script setup>
/*
 * 產品配方與配料的共用編輯器。兩者結構相同，差別只在單位：
 * 配方是 Baker's %、配料是絕對克數。換算公式對兩者一致
 * （用量 = 總重 × ratio / Σratio），所以只需一份 UI。
 */
import { computed, inject, ref } from 'vue'
import { fmt } from '../format.js'

const props = defineProps({
  title: String,
  items: Array, // 既有的配方清單
  unitKey: String, // 'pct' 或 'weightG'
  unitLabel: String, // 'Baker %' 或 '配方重量(g)'
  hint: String,
  api: Object, // { create, update, remove }
})

const toast = inject('toast')
const reload = inject('reload')

const editing = ref(blank())
function blank() {
  return { id: '', name: '', ingredients: [{ name: '', [props.unitKey]: 100 }] }
}

const isNew = computed(() => !editing.value.id)

// 預覽：以 1000g 為基準顯示各材料用量，讓使用者輸入時就看得到比例
const PREVIEW_TOTAL_G = 1000
const preview = computed(() => {
  const items = editing.value.ingredients.filter((i) => i.name && i[props.unitKey] > 0)
  const sum = items.reduce((a, i) => a + Number(i[props.unitKey] || 0), 0)
  if (!sum) return []
  return items.map((i) => ({
    name: i.name,
    usageG: (PREVIEW_TOTAL_G * Number(i[props.unitKey])) / sum,
  }))
})

function load(id) {
  const found = props.items.find((x) => x.id === id)
  if (!found) return
  // 深拷貝：直接綁定共用資料會讓未儲存的編輯洩漏到其他 tab
  editing.value = JSON.parse(JSON.stringify(found))
}

function addRow() {
  editing.value.ingredients.push({ name: '', [props.unitKey]: 0 })
}

function removeRow(i) {
  editing.value.ingredients.splice(i, 1)
}

async function save() {
  const payload = {
    ...editing.value,
    ingredients: editing.value.ingredients
      .filter((i) => i.name.trim())
      .map((i) => ({ name: i.name.trim(), [props.unitKey]: Number(i[props.unitKey]) })),
  }
  const saved = await toast.run(
    () => (isNew.value ? props.api.create(payload) : props.api.update(payload)),
    isNew.value ? '已新增' : '已儲存',
  )
  if (!saved) return
  editing.value = JSON.parse(JSON.stringify(saved))
  await reload()
}

async function remove() {
  if (isNew.value) return
  if (!confirm(`確定要刪除「${editing.value.name}」嗎？`)) return
  const done = await toast.run(() => props.api.remove(editing.value.id).then(() => true), '已刪除')
  if (!done) return
  editing.value = blank()
  await reload()
}
</script>

<template>
  <div class="card">
    <div class="search-box">
      <label>🔍 載入已建立的{{ title }}</label>
      <select :value="editing.id" @change="load($event.target.value)">
        <option value="">— 選擇要編輯的項目 —</option>
        <option v-for="x in items" :key="x.id" :value="x.id">{{ x.name }}</option>
      </select>
    </div>

    <h2>{{ isNew ? '新增' : '編輯' }}{{ title }}</h2>
    <label>名稱</label>
    <input v-model.trim="editing.name" :placeholder="`例如：${title === '產品配方' ? '奶酥吐司麵團' : '奶酥餡'}`">

    <div class="notice" style="margin-top:12px">{{ hint }}</div>

    <div class="tablewrap">
      <table>
        <thead>
          <tr><th>材料名稱</th><th>{{ unitLabel }}</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="(it, i) in editing.ingredients" :key="i">
            <td><input v-model.trim="it.name" placeholder="材料名稱"></td>
            <td><input v-model.number="it[unitKey]" type="number" min="0" step="0.1"></td>
            <td><button class="danger" @click="removeRow(i)">刪</button></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="preview.length" style="margin-top:14px">
      <h3>換算預覽（以 {{ fmt(PREVIEW_TOTAL_G, 0) }}g 為例）</h3>
      <p class="muted">
        <span v-for="(p, i) in preview" :key="p.name">
          {{ i ? '、' : '' }}{{ p.name }} {{ fmt(p.usageG) }}g
        </span>
      </p>
    </div>

    <div class="toolbar">
      <button class="secondary" @click="addRow">＋ 新增材料</button>
      <button @click="save">💾 {{ isNew ? '新增' : '儲存' }}</button>
      <button class="danger" :disabled="isNew" @click="remove">🗑 刪除</button>
      <button class="secondary" @click="editing = blank()">✨ 清空重新輸入</button>
    </div>
  </div>
</template>
