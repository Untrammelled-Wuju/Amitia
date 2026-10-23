<script setup lang="ts">
import { ref } from 'vue';
import searchFunc from './searchFunc.vue';
const emit = defineEmits(['updateMenuData','renewInfo']);
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
  orderWay: '1',
  isDeleted: '',
  startTime: '',
  endTime: '',
  status: '0',
})
const changeStudentData = (serachValue: { searchValue: string, status: string }) => {
  menuData.value.value = serachValue.searchValue
  menuData.value.status = serachValue.status
  emit('updateMenuData', menuData.value)
}
</script>
<template>
  <div id="draftBoxMune">
    <searchFunc @serachValue="changeStudentData"/>
  </div>
</template>
<style scoped>
#draftBoxMune{
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
