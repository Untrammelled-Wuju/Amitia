<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import draftBoxMune from './draftBoxMune/draftBoxMune.vue';
import draftBoxInfo from './draftBoxInfo/draftBoxInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  isTop: string
  status: string
  isDeleted: string
  IsComment: string
  categoryId: string
  tagId: string
  startTime: string
  endTime: string
  title: string
  orderWay: string
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '2',
  isDeleted: '',
  IsComment: '',
  categoryId: '',
  tagId: '',
  startTime: '',
  endTime: '',
  title: '',
  orderWay: '',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}

</script>
<template>
   <div id="draftBox">
      <draftBoxMune @updateMenuData="updateMenuData"/>
      <draftBoxInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#draftBox{
  width: 100%;
}
</style>

