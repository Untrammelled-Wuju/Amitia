<script setup lang="ts">
import { ref } from 'vue';
import ChangeTheme from '@/utils/themeContral';

const lightMode = ref(false)
if(localStorage.getItem('theme') && localStorage.getItem('theme') === 'dark'){
  lightMode.value = true
} else {
  lightMode.value = false
}
const enterRotateValue = ref(180);
const leaveRotateValue = ref(-180);
const changeTheme = () => {
  lightMode.value = !lightMode.value
  enterRotateValue.value = lightMode.value ? 180 : -180;
  leaveRotateValue.value = lightMode.value ? -180 : 180;
  ChangeTheme();
}
</script>
<template>
  <button class="changeTheme" @click="changeTheme">
    <transition
      name="rotate"
      @before-enter="(el: Element) => (el as HTMLElement).style.setProperty('--rotate-enter', enterRotateValue + 'deg')"
      @before-leave="(el: Element) => (el as HTMLElement).style.setProperty('--rotate-leave', leaveRotateValue + 'deg')"
    >
      <svg v-if="lightMode" t="1752212592100" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="3550" width="200" height="200"><path d="M501.48 493.55m-233.03 0a233.03 233.03 0 1 0 466.06 0 233.03 233.03 0 1 0-466.06 0Z" fill="#F9C626" p-id="3551"></path><path d="M501.52 185.35H478.9c-8.28 0-15-6.72-15-15V87.59c0-8.28 6.72-15 15-15h22.62c8.28 0 15 6.72 15 15v82.76c0 8.28-6.72 15-15 15zM281.37 262.76l-16 16c-5.86 5.86-15.36 5.86-21.21 0l-58.52-58.52c-5.86-5.86-5.86-15.36 0-21.21l16-16c5.86-5.86 15.36-5.86 21.21 0l58.52 58.52c5.86 5.86 5.86 15.35 0 21.21zM185.76 478.48v22.62c0 8.28-6.72 15-15 15H88c-8.28 0-15-6.72-15-15v-22.62c0-8.28 6.72-15 15-15h82.76c8.28 0 15 6.72 15 15zM270.69 698.63l16 16c5.86 5.86 5.86 15.36 0 21.21l-58.52 58.52c-5.86 5.86-15.36 5.86-21.21 0l-16-16c-5.86-5.86-5.86-15.36 0-21.21l58.52-58.52c5.85-5.86 15.35-5.86 21.21 0zM486.41 794.24h22.62c8.28 0 15 6.72 15 15V892c0 8.28-6.72 15-15 15h-22.62c-8.28 0-15-6.72-15-15v-82.76c0-8.28 6.72-15 15-15zM706.56 709.31l16-16c5.86-5.86 15.36-5.86 21.21 0l58.52 58.52c5.86 5.86 5.86 15.36 0 21.21l-16 16c-5.86 5.86-15.36 5.86-21.21 0l-58.52-58.52c-5.86-5.85-5.86-15.35 0-21.21zM802.17 493.59v-22.62c0-8.28 6.72-15 15-15h82.76c8.28 0 15 6.72 15 15v22.62c0 8.28-6.72 15-15 15h-82.76c-8.28 0-15-6.72-15-15zM717.24 273.44l-16-16c-5.86-5.86-5.86-15.36 0-21.21l58.52-58.52c5.86-5.86 15.36-5.86 21.21 0l16 16c5.86 5.86 5.86 15.36 0 21.21l-58.52 58.52c-5.86 5.86-15.35 5.86-21.21 0z" fill="#F9C626" p-id="3552"></path></svg>
    </transition>
    <transition
      name="rotate"
      @before-enter="(el: Element) => (el as HTMLElement).style.setProperty('--rotate-enter', enterRotateValue + 'deg')"
      @before-leave="(el: Element) => (el as HTMLElement).style.setProperty('--rotate-leave', leaveRotateValue + 'deg')"
    >
      <svg v-if="!lightMode" t="1752212627643" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="4628" width="200" height="200"><path d="M505.4 878.6c-196.7 0-358-150.9-374.9-343.1 1-18.6 16.1-33.4 34.9-33.4 10.8 0 20.5 4.8 26.9 12.4 0.2 0.3 0.5 0.1 0.5-0.7 41.6 44.2 100.5 71.9 166.1 71.9 127.1 0 230.1-103 230.1-230.1 0-66.1-28-125.1-72.6-166.8 0.1-0.1 0.5-0.1 0.3-0.3-7-6.5-11.4-15.7-11.4-26.1 0-19 14.9-34.1 33.7-35.3 192.1 17.1 342.9 178.3 342.9 375 0 208-168.5 376.5-376.5 376.5z" fill="#f4ea2a" p-id="4629"></path></svg>
    </transition>
  </button>
</template>
<style scoped>
.rotate-enter-active,
.rotate-leave-active {
  transition: var(--transitionTimeSlow) ease;
}
.rotate-enter-from {
  opacity: 0;
  transform: rotate(var(--rotate-enter, 180deg));
}
.rotate-leave-to {
  opacity: 0;
  transform: rotate(calc(var(--rotate-leave, -180deg)));
}
button{
  --wh: 1.8rem;
  height: 100%;
  width: var(--wh);
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  background: transparent;
  svg{
    width: var(--wh);
    height: var(--wh);
    cursor: pointer;
    transition: var(--transition) ease;
    position: absolute;
  }
}
</style>
