<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
const router = useRouter()
const route = useRoute()

const current = ref<number>(0)
interface menuItemsInterFace {
  name: string
  path: string
}
const menuItems: menuItemsInterFace[] = [
  {
    name: '用户设置',
    path: '/setting/userInfo',
  },
  {
    name: '修改密码',
    path: '/setting/editPassword',
  },
]
const handleMenuItemClick = (index: number) => {
  if(current.value === index) {
    return
  }
  current.value = index
  router.push(menuItems[index].path)
}
onMounted(() => {
  for(let i = 0; i < menuItems.length;i++){
    if(route.path === menuItems[i].path){
      current.value = i
    }
  }
})
</script>
<template>
  <div id="menu">
    <ul class="menuList">
      <li
        class="listItem"
        v-for="(item, index) in menuItems"
        :key="index"
        @click="handleMenuItemClick(index)"
        :class="{active:index === current}"
      >
        {{ item.name }}
      </li>
    </ul>
  </div>
</template>
<style scoped>
#menu {
  width: 100%;
  /* height: var(--NavigationMenuHeight); */
  height: 50px;
  border-radius: var(--BorderRadius);
  background: var(--Main_Bg_BoxColor);
  box-shadow: 0px 0px 6px var(--BoxShadowColor);
  padding: 0 8px;
  position: relative;
  overflow: scroll;
  &::-webkit-scrollbar {
    display: none;
  }
}
.menuList {
  display: flex;
  height: 100%;
  width: 100%;
  align-items: center;
  .listItem {
    user-select: none;
    padding: 3px 2px;
    border-radius: var(--BorderRadius);
    color: #555;
    cursor: pointer;
    position: relative;
    margin-right: 10px;
    &:last-child {
      margin: 0;
    }
    &.active {
      color: rgb(23, 104, 255);
      &::after {
        content: '';
        width: 100%;
      }
    }
    &::after {
      content: '';
      width: 0;
      height: 2px;
      background-color: rgb(141, 175, 255);
      position: absolute;
      bottom: 0;
      left: 0;
      transition: 0.3s ease;
    }
    &::marker{
      content: '';
      display: none;
    }
    &:hover {
      &::after {
        width: 100%;
      }
    }
  }
}
</style>
