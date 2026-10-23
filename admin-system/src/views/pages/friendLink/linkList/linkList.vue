<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import linkListMenu from './linkListMenu/linkListMenu.vue'
import linkListInfo from './linkListInfo/linkListInfo.vue'
import { ref } from 'vue'
interface MenuDataModel {
  value: string
  orderWay: string
  isDeleted: string
  startTime: string
  endTime: string
  status: string
}
const menuData = ref<MenuDataModel>({
  value: '',
  orderWay: '',
  isDeleted: '',
  startTime: '',
  endTime: '',
  status: '1',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
  console.log(menuData.value)
}
</script>
<template>
  <div id="articleList">
    <linkListMenu @updateMenuData="updateMenuData" />
    <linkListInfo :menuPropsData="menuData" />
  </div>
</template>
<style scoped>
#articleList {
  width: 100%;
}
</style>
