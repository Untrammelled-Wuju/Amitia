<script setup lang="ts">
import { ref } from 'vue'
import ChangeTheme from '@/utils/themeContral'

const lightMode = ref(false)
if (localStorage.getItem('theme') && localStorage.getItem('theme') === 'dark') {
  lightMode.value = true
} else {
  lightMode.value = false
}
const enterRotateValue = ref(180)
const leaveRotateValue = ref(-180)
const changeTheme = () => {
  lightMode.value = !lightMode.value
  enterRotateValue.value = lightMode.value ? 180 : -180
  leaveRotateValue.value = lightMode.value ? -180 : 180
  ChangeTheme()
}
</script>
<template>
  <button class="changeTheme" @click="changeTheme">
    <transition
      name="rotate"
      @before-enter="
        (el: Element) =>
          (el as HTMLElement).style.setProperty('--rotate-enter', enterRotateValue + 'deg')
      "
      @before-leave="
        (el: Element) =>
          (el as HTMLElement).style.setProperty('--rotate-leave', leaveRotateValue + 'deg')
      "
    >
      <svg
        v-if="lightMode"
        class="sun"
        xmlns="http://www.w3.org/2000/svg"
        xmlns:xlink="http://www.w3.org/1999/xlink"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
      >
        <path
          d="M12.75 2C12.75 1.58579 12.4142 1.25 12 1.25C11.5858 1.25 11.25 1.58579 11.25 2L11.25 4C11.25 4.41421 11.5858 4.75 12 4.75C12.4142 4.75 12.75 4.41421 12.75 4L12.75 2ZM4.72825 3.66865C4.42255 3.38915 3.94815 3.41039 3.66865 3.71609C3.38915 4.02179 3.41039 4.49619 3.71609 4.77569L5.9381 6.80727C6.2438 7.08677 6.7182 7.06553 6.9977 6.75983C7.2772 6.45413 7.25596 5.97974 6.95026 5.70024L4.72825 3.66865ZM20.2839 4.77569C20.5896 4.49619 20.6109 4.02179 20.3314 3.71609C20.0518 3.41039 19.5775 3.38915 19.2718 3.66865L17.0497 5.70024C16.744 5.97974 16.7228 6.45413 17.0023 6.75983C17.2818 7.06553 17.7562 7.08677 18.0619 6.80727L20.2839 4.77569ZM7.75 12C7.75 9.65279 9.65279 7.75 12 7.75C14.3472 7.75 16.25 9.65279 16.25 12C16.25 14.3472 14.3472 16.25 12 16.25C9.65279 16.25 7.75 14.3472 7.75 12ZM12 6.25C8.82436 6.25 6.25 8.82436 6.25 12C6.25 15.1756 8.82436 17.75 12 17.75C15.1756 17.75 17.75 15.1756 17.75 12C17.75 8.82436 15.1756 6.25 12 6.25ZM2 11.25C1.58579 11.25 1.25 11.5858 1.25 12C1.25 12.4142 1.58579 12.75 2 12.75L4 12.75C4.41421 12.75 4.75 12.4142 4.75 12C4.75 11.5858 4.41421 11.25 4 11.25L2 11.25ZM20 11.25C19.5858 11.25 19.25 11.5858 19.25 12C19.25 12.4142 19.5858 12.75 20 12.75L22 12.75C22.4142 12.75 22.75 12.4142 22.75 12C22.75 11.5858 22.4142 11.25 22 11.25L20 11.25ZM18.0862 17.0252C17.7933 16.7323 17.3184 16.7323 17.0255 17.0252C16.7326 17.3181 16.7326 17.7929 17.0255 18.0858L19.2475 20.3081C19.5403 20.601 20.0152 20.6011 20.3081 20.3082C20.601 20.0153 20.6011 19.5404 20.3082 19.2475L18.0862 17.0252ZM6.97467 18.086C7.26756 17.7931 7.26756 17.3182 6.97467 17.0253C6.68177 16.7324 6.2069 16.7324 5.91401 17.0253L3.69178 19.2476C3.39889 19.5404 3.39889 20.0153 3.69178 20.3082C3.98468 20.6011 4.45955 20.6011 4.75244 20.3082L6.97467 18.086ZM12.75 20C12.75 19.5858 12.4142 19.25 12 19.25C11.5858 19.25 11.25 19.5858 11.25 20L11.25 22C11.25 22.4142 11.5858 22.75 12 22.75C12.4142 22.75 12.75 22.4142 12.75 22L12.75 20Z"
          fill-rule="evenodd"
          fill="currentColor"
        />
      </svg>
    </transition>
    <transition
      name="rotate"
      @before-enter="
        (el: Element) =>
          (el as HTMLElement).style.setProperty('--rotate-enter', enterRotateValue + 'deg')
      "
      @before-leave="
        (el: Element) =>
          (el as HTMLElement).style.setProperty('--rotate-leave', leaveRotateValue + 'deg')
      "
    >
      <svg
        v-if="!lightMode"
        xmlns="http://www.w3.org/2000/svg"
        xmlns:xlink="http://www.w3.org/1999/xlink"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
      >
        <path
          d="M2.75 12C2.75 7.22328 6.37072 3.29221 11.0174 2.80157C9.33315 4.1283 8.25 6.18738 8.25 8.5C8.25 12.5041 11.4959 15.75 15.5 15.75C17.8126 15.75 19.8717 14.6669 21.1984 12.9826C20.7078 17.6293 16.7767 21.25 12 21.25C6.89137 21.25 2.75 17.1086 2.75 12ZM12 1.25C6.06294 1.25 1.25 6.06294 1.25 12C1.25 17.9371 6.06294 22.75 12 22.75C17.9371 22.75 22.75 17.9371 22.75 12C22.75 11.2834 22.1787 10.9246 21.7237 10.8632C21.286 10.804 20.7293 10.9658 20.4253 11.469C19.4172 13.1373 17.5882 14.25 15.5 14.25C12.3244 14.25 9.75 11.6756 9.75 8.5C9.75 6.41182 10.8627 4.5828 12.531 3.57467C13.0342 3.27065 13.196 2.71398 13.1368 2.27627C13.0754 1.82126 12.7166 1.25 12 1.25Z"
          fill-rule="evenodd"
          fill="currentColor"
        />
      </svg>
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
button {
  width: var(--OverviewMenuIconwh);
  height: var(--OverviewMenuIconwh);
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  background: transparent;
  margin-right: 20px;
  cursor: pointer;
  fill: var(--Main_font_SecondColor);
  * {
    fill: var(--Main_font_SecondColor);
  }
  svg {
    height: 100%;
    width: 100%;
    cursor: pointer;
    transition: var(--transitionTime) ease;
    position: absolute;
    &.sun {
      --wh: 110%;
      height: var(--wh);
      width: var(--wh);
    }
  }
}
</style>
