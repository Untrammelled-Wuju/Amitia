<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import navigationListMenu from './navigationListMenu/navigationListMenu.vue';
import navigationListInfo from './navigationListInfo/navigationListInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  status: string
  isDeleted: string
  category: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isDeleted: '',
  category: '',
  startTime: '',
  endTime: '',
  value: '',
  orderWay: 'DESC',
})
const updateMenuData = (data:MenuDataModel) => {
  menuData.value = data
  console.log(menuData.value)
}
</script>
<template>
   <div id="ideaList">
      <navigationListMenu @updateMenuData="updateMenuData"/>
      <navigationListInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#ideaList{
  width: 100%;
}
</style>

