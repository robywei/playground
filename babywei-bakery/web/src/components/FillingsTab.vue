<script setup>
import { computed, inject } from 'vue'
import * as api from '../api.js'
import RecipeEditor from './RecipeEditor.vue'

const shared = inject('shared')
const items = computed(() => shared.value.fillings)
</script>

<template>
  <RecipeEditor
    title="配料"
    :items="items"
    unit-key="weightG"
    unit-label="配方重量(g)"
    hint="以實際重量(g)管理比例。生產時會按這個比例換算到需要的總重。"
    :api="{ create: api.createFilling, update: api.updateFilling, remove: api.deleteFilling }"
  />
</template>
