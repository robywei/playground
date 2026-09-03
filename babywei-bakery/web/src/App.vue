<script setup>
import { computed, onMounted, provide, ref } from 'vue'
import * as api from './api.js'
import { useToast } from './useToast.js'

import PurchasesTab from './components/PurchasesTab.vue'
import InventoryTab from './components/InventoryTab.vue'
import DoughsTab from './components/DoughsTab.vue'
import FillingsTab from './components/FillingsTab.vue'
import ProductsTab from './components/ProductsTab.vue'
import ProductionTab from './components/ProductionTab.vue'
import ReportsTab from './components/ReportsTab.vue'

const TABS = [
  { id: 'purchases', label: '💰 成本庫 (進貨管理)', comp: PurchasesTab },
  { id: 'inventory', label: '📦 庫存管理', comp: InventoryTab },
  { id: 'doughs', label: '🥣 產品配方', comp: DoughsTab },
  { id: 'fillings', label: '🍯 配料(克數)', comp: FillingsTab },
  { id: 'products', label: '📦 商品設定', comp: ProductsTab },
  { id: 'production', label: '🧮 今日生產', comp: ProductionTab },
  { id: 'reports', label: '📊 利潤與報表', comp: ReportsTab },
]

const active = ref('purchases')
const toast = useToast()
provide('toast', toast)

// 共用的參照資料。任何 tab 改動後呼叫 reload，其他 tab 立刻拿到新值 ——
// 否則使用者會在別的分頁看到過期的配方或商品清單。
const shared = ref({ purchases: [], doughs: [], fillings: [], products: [] })
provide('shared', shared)

async function reload() {
  await toast.run(async () => {
    const [purchases, doughs, fillings, products] = await Promise.all([
      api.listPurchases(),
      api.listDoughs(),
      api.listFillings(),
      api.listProducts(),
    ])
    shared.value = { purchases, doughs, fillings, products }
  })
}
provide('reload', reload)

onMounted(reload)

const activeComponent = computed(() => TABS.find((t) => t.id === active.value)?.comp)

// 結束程式。.app 宣告 LSUIElement 之後沒有 Dock 圖示可以按右鍵結束，
// 這裡是唯一的關閉途徑。
const quitting = ref(false)
async function quit() {
  if (!window.confirm(
    '確定要結束程式嗎？\n\n' +
    '資料都已經存好了。下次要用時，再雙擊「BabyWei Bakery」圖示即可。',
  )) return

  quitting.value = true
  try {
    await api.shutdown()
  } catch (e) {
    // 後端可能在回應送達前就關了，那也算成功
    console.debug('結束程式時的回應:', e)
  }
  toast.ok('已結束。這個分頁可以關掉了。')
}
</script>

<template>
  <header>
    <div class="head">
      <div class="brand">
        <h1>🥖 BabyWei Bakery</h1>
        <p>本地版 · 資料存在你的電腦上</p>
      </div>
      <button class="secondary" :disabled="quitting" @click="quit">
        {{ quitting ? '正在關閉…' : '⏻ 結束程式' }}
      </button>
    </div>
  </header>

  <nav>
    <button
      v-for="t in TABS"
      :key="t.id"
      class="tab"
      :class="{ active: active === t.id }"
      @click="active = t.id"
    >
      {{ t.label }}
    </button>
  </nav>

  <main>
    <component :is="activeComponent" />
  </main>

  <div class="toast">
    <div v-for="m in toast.messages.value" :key="m.id" :class="m.kind">
      {{ m.text }}
    </div>
  </div>
</template>
