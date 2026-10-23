<script setup lang="ts">
import { ref } from 'vue';
import addButton from './addButton.vue';
import searchFunc from './searchFunc.vue';
const emit = defineEmits(['updateMenuData','renewInfo']);
interface MenuDataModel {
  name: string
  orderWay: string
}
const menuData = ref<MenuDataModel>({
  name: '',
  orderWay: '',
})
const changeStudentData = (serachValue: { searchValue: string, orderWay: string }) => {
  menuData.value.name = serachValue.searchValue
  menuData.value.orderWay = serachValue.orderWay
  emit('updateMenuData', menuData.value)
}
</script>
<template>
  <div id="draftBoxMune">
    <searchFunc @serachValue="changeStudentData"/>
    <addButton @add_RenewData="$emit('renewInfo')"/>
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
