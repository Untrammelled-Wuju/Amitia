<script setup lang="ts">
import { ref } from 'vue';
import searchFunc from './searchFunc.vue';
const emit = defineEmits(['updateMenuData']);
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
  isCarousel: string
  type: string
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '0',
  isDeleted: '1',
  IsComment: '',
  categoryId: '',
  tagId: '',
  startTime: '',
  endTime: '',
  title: '',
  orderWay: '',
  isCarousel: '',
  type: '',
})
const changeStudentData = (serachValue: { searchValue: string, orderWay: string }) => {
  menuData.value.title = serachValue.searchValue
  menuData.value.orderWay = serachValue.orderWay
  emit('updateMenuData', menuData.value)
}
</script>
<template>
  <div id="recycleBinMune">
    <searchFunc @serachValue="changeStudentData"/>
  </div>
</template>
<style scoped>
#recycleBinMune{
  width: 100%;
  height: var(--NavigationMenuHeight);
  border-radius: var(--BorderRadius);
  background: var(--Main_Bg_BoxColor);
  box-shadow: 0px 0px 6px var(--BoxShadowColor);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 8px;
  position: relative;
  .right{
    display: flex;
    height: 100%;
    align-items: center;
  }
}
</style>
