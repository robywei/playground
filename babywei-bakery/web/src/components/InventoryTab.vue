<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import * as api from '../api.js'
import { fmt } from '../format.js'

const toast = inject('toast')
const rows = ref([])

async function load() {
  const got = await toast.run(() => api.getInventory())
  if (got) rows.value = got
}
onMounted(load)
defineExpose({ load })

const LABEL = { ok: '✅ 充足', low: '⚡ 庫存偏低', out: '⚠️ 庫存見底' }

// 剩餘為負代表有消耗卻沒有對應進貨 —— 通常是漏登，值得單獨提示。
const missingPurchases = computed(() => rows.value.filter((r) => r.remainingG < 0))
</script>

<template>
  <div class="card">
    <h2>📦 現有原料庫存盤點</h2>
    <div class="notice">
      總進貨量減去「今日生產」實際扣除的克數。消耗量是<strong>生產當下寫死的快照</strong>，
      事後修改配方或刪除商品都不會回頭改變這裡的數字。
    </div>

    <div v-if="missingPurchases.length" class="notice warn">
      有 {{ missingPurchases.length }} 項材料的剩餘量是負數
      （{{ missingPurchases.map((r) => r.name).join('、') }}）。
      這代表配方用到了它，但沒有對應的進貨紀錄 —— 很可能是進貨漏登了。
    </div>

    <div class="toolbar" style="margin-top:0;margin-bottom:14px">
      <button class="secondary" @click="load">🔄 重新整理</button>
    </div>

    <div class="tablewrap">
      <table>
        <thead>
          <tr>
            <th>材料名稱</th><th>品牌</th><th>總進貨量(g)</th>
            <th>已生產消耗(g)</th><th>目前剩餘(g)</th><th>狀態</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length">
            <td colspan="6" style="text-align:center;color:#8c8177">尚無材料與庫存資料</td>
          </tr>
          <tr v-for="r in rows" :key="r.name">
            <td><b>{{ r.name }}</b></td>
            <td>{{ r.brand || '—' }}</td>
            <td>{{ fmt(r.totalBoughtG) }}</td>
            <td>{{ fmt(r.totalUsedG) }}</td>
            <td><b>{{ fmt(r.remainingG) }}</b></td>
            <td><span class="badge" :class="`badge-${r.status}`">{{ LABEL[r.status] }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
