<!-- eslint-disable vue/block-lang -->
<script setup>
import { onMounted, ref } from 'vue'

onMounted(() => {
  if (localStorage.getItem('NavShowFoldModel') === null) {
    localStorage.setItem('NavShowFoldModel', 'false')
  } else if (localStorage.getItem('NavShowFoldModel') === 'true') {
    icon.value.classList.toggle('animation')
  }
})
const icon = ref(null)
const changeFoldModel = () => {
  if (localStorage.getItem('NavShowFoldModel') === 'false') {
    icon.value.classList.toggle('animation')
    localStorage.setItem('NavShowFoldModel', 'true')
  } else {
    icon.value.classList.toggle('animation')
    localStorage.setItem('NavShowFoldModel', 'false')
  }
}
</script>
<template>
  <svg
    ref="icon"
    @click="changeFoldModel()"
    xmlns="http://www.w3.org/2000/svg"
    xmlns:xlink="http://www.w3.org/1999/xlink"
    width="24"
    height="24"
    viewBox="0 0 24 24"
    fill="none"
  >
    <path
      d="M5.25 21C5.25 21.4142 5.58579 21.75 6 21.75C6.41422 21.75 6.75 21.4142 6.75 21L6.75 2.99997C6.75 2.58576 6.41422 2.24997 6 2.24997C5.58579 2.24997 5.25 2.58576 5.25 2.99997L5.25 21ZM9.25001 21C9.25001 21.4142 9.58579 21.75 10 21.75C10.4142 21.75 10.75 21.4142 10.75 21L10.75 2.99997C10.75 2.58576 10.4142 2.24997 10 2.24997C9.58579 2.24997 9.25001 2.58576 9.25001 2.99997L9.25001 21ZM13.25 21C13.25 21.4142 13.5858 21.75 14 21.75C14.4142 21.75 14.75 21.4142 14.75 21L14.75 13C14.75 12.5858 14.4142 12.25 14 12.25C13.5858 12.25 13.25 12.5858 13.25 13L13.25 21ZM13.4697 5.96965C13.1768 6.26254 13.1768 6.73741 13.4697 7.03031L15.9697 9.53031C16.2626 9.8232 16.7375 9.8232 17.0303 9.53031C17.3232 9.23742 17.3232 8.76254 17.0303 8.46965L15.8107 7.24998L20 7.24998C20.4142 7.24998 20.75 6.91419 20.75 6.49998C20.75 6.08576 20.4142 5.74998 20 5.74998L15.8107 5.74998L17.0303 4.5303C17.3232 4.23741 17.3232 3.76254 17.0303 3.46964C16.7375 3.17675 16.2626 3.17675 15.9697 3.46964L13.4697 5.96965ZM17.25 21C17.25 21.4142 17.5858 21.75 18 21.75C18.4142 21.75 18.75 21.4142 18.75 21L18.75 13C18.75 12.5858 18.4142 12.25 18 12.25C17.5858 12.25 17.25 12.5858 17.25 13L17.25 21Z"
      fill-rule="evenodd"
      fill="currentColor"
    ></path>
  </svg>
</template>
<style scoped>
svg {
  margin-right: 24px;
  width: var(--OverviewMenuIconwh);
  height: var(--OverviewMenuIconwh);
  cursor: pointer;
  transition: 0.3s ease;
  fill: var(--Main_font_SecondColor);
  * {
    fill: var(--Main_font_SecondColor);
  }
  &.animation {
    transform: rotateY(180deg);
  }
}
</style>
