<!-- eslint-disable vue/block-lang -->
<script setup>
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router' // 添加useRouter
const route = useRoute()
const router = useRouter() // 新增router实例

const nowRouterPath = ref(route.path)
watch(route, () => {
  nowRouterPath.value = route.path
})

const RenewPage = () => {
  const icon = document.querySelector('.RenweIcon')
  icon.classList.remove('active')
  setTimeout(() => {
    icon.classList.add('active')
  }, 0)
  // 先保存原始查询参数
  const originalQuery = { ...route.query }

  // 临时添加并立即移除参数
  router
    .replace({
      path: route.path,
      query: { ...originalQuery, __secret: Date.now() },
    })
    .then(() => {
      router.replace({ path: route.path, query: originalQuery })
    })
}
</script>
<template>
  <svg
    @click="RenewPage"
    class="RenweIcon"
    xmlns="http://www.w3.org/2000/svg"
    xmlns:xlink="http://www.w3.org/1999/xlink"
    width="24"
    height="24"
    viewBox="0 0 24 24"
    fill="none"
  >
    <path
      d="M12.0789 2.25C7.07619 2.25 3.00244 6.23968 2.93077 11.2003L2.52824 10.8009C2.2342 10.5092 1.75933 10.5111 1.46759 10.8051C1.17585 11.0991 1.17772 11.574 1.47176 11.8657L3.15157 13.5324C3.44398 13.8225 3.91565 13.8225 4.20805 13.5324L5.88787 11.8657C6.18191 11.574 6.18377 11.0991 5.89203 10.8051C5.60029 10.511 5.12542 10.5092 4.83138 10.8009L4.431 11.1982C4.50372 7.0777 7.8945 3.75 12.0789 3.75C14.8484 3.75 17.2727 5.20845 18.6156 7.39279C18.8325 7.74565 19.2944 7.85585 19.6473 7.63892C20.0001 7.42199 20.1103 6.96007 19.8934 6.60721C18.287 3.99427 15.3873 2.25 12.0789 2.25ZM20.8409 10.4666C20.5487 10.1778 20.0786 10.1778 19.7864 10.4666L18.1002 12.1333C17.8056 12.4244 17.8028 12.8993 18.094 13.1939C18.3852 13.4885 18.86 13.4913 19.1546 13.2001L19.5625 12.7969C19.4922 16.9171 16.0887 20.25 11.8825 20.25C9.09999 20.25 6.6656 18.7903 5.31769 16.6061C5.10016 16.2536 4.63806 16.1442 4.28557 16.3618C3.93307 16.5793 3.82366 17.0414 4.0412 17.3939C5.65381 20.007 8.56379 21.75 11.8825 21.75C16.9017 21.75 20.9934 17.7604 21.0628 12.7949L21.4727 13.2001C21.7673 13.4913 22.2421 13.4885 22.5333 13.1939C22.8245 12.8993 22.8217 12.4244 22.5271 12.1333L20.8409 10.4666Z"
      fill-rule="evenodd"
      fill="currentColor"
    ></path>
  </svg>
</template>
<style scoped>
@keyframes rotateAnimation {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}
.RenweIcon {
  margin-left: 24px;
  width: var(--OverviewMenuIconwh);
  height: var(--OverviewMenuIconwh);
  cursor: pointer;
  fill: var(--Main_font_SecondColor);
  * {
    fill: var(--Main_font_SecondColor);
  }
  &.active {
    animation: rotateAnimation 0.6s ease forwards;
  }
}
</style>
