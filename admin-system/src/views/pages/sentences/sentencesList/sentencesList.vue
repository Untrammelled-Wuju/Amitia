<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import sentencesListMenu from './sentencesListMenu/sentencesListMenu.vue';
import sentencesListInfo from './sentencesListInfo/sentencesListInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  isTop: string
  status: string
  isDeleted: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '0',
  isDeleted: '',
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
   <div id="sentencesList">
      <sentencesListMenu @updateMenuData="updateMenuData"/>
      <sentencesListInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#sentencesList{
  width: 100%;
}
</style>

