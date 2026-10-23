<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import recycleBinMune from './recycleBinMune/recycleBinMune.vue'
import recycleBinInfo from './recycleBinInfo/recycleBinInfo.vue'
import { ref } from 'vue'
interface MenuDataModel {
  status: string
  isTop: number
  isDeleted: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isTop: 1,
  isDeleted: '1',
  startTime: '',
  endTime: '',
  value: '',
  orderWay: 'DESC',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}
</script>
<template>
  <div id="recycleBin">
    <recycleBinMune @updateMenuData="updateMenuData" />
    <recycleBinInfo :menuPropsData="menuData" />
  </div>
</template>
<style scoped>
#recycleBin {
  width: 100%;
}
</style>
