<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import websiteConfigMenu from './websiteConfigMenu/websiteConfigMenu.vue'
</script>
<template>
  <div id="websiteConfig">
    <websiteConfigMenu />
    <div id="websiteConfigurationContent">
      <RouterView />
    </div>
  </div>
</template>
<style scoped>
#websiteConfig{
  display: flex;
  flex-direction: column;
  height: 99%;
  width: 100%;
}
#websiteConfigurationContent {
  width: 100%;
  flex: 1;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
  overflow: scroll;
  &::-webkit-scrollbar {
    width: 5px;
    height: 2px;
  }
  &::-webkit-scrollbar-thumb {
    background: rgb(188, 188, 188);
    border-radius: 10px;
  }
}
</style>
