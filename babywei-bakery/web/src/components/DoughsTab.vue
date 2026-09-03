<script setup>
import { computed, inject } from 'vue'
import * as api from '../api.js'
import RecipeEditor from './RecipeEditor.vue'

const shared = inject('shared')
const items = computed(() => shared.value.doughs)
</script>

<template>
  <RecipeEditor
    title="產品配方"
    :items="items"
    unit-key="pct"
    unit-label="Baker %"
    hint="以 Baker's % 表示：主粉類為 100%，其他材料相對於粉重。實際換算會正規化，所以基準不必剛好是 100%。"
    :api="{ create: api.createDough, update: api.updateDough, remove: api.deleteDough }"
  />
</template>
