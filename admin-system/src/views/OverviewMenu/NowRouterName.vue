<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
const route = useRoute()

const nowRouterName = ref(route.name)
document.title = `后台管理系统-${route.meta.title || nowRouterName.value}`
watch(route, () => {
  nowRouterName.value = route.name
  document.title = `后台管理系统-${route.meta.title || nowRouterName.value}`
})

</script>
<template>
  <div class="now-router-name">
    {{ nowRouterName === '' ? '/' : nowRouterName }}
  </div>
</template>
<style scoped>
.now-router-name{
  font-size: 16px;
  color: var(--Main_font_SecondColor);
}
</style>
