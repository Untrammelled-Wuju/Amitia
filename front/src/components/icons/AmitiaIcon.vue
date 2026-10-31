<template>
  <svg
    class="amitia-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path v-for="path in shape.paths" :key="`path-${path}`" :d="path" />
    <line
      v-for="line in shape.lines"
      :key="`line-${line.join('-')}`"
      :x1="line[0]"
      :y1="line[1]"
      :x2="line[2]"
      :y2="line[3]"
    />
    <circle
      v-for="circle in shape.circles"
      :key="`circle-${circle.cx}-${circle.cy}-${circle.r}`"
      :cx="circle.cx"
      :cy="circle.cy"
      :r="circle.r"
      :fill="circle.fill ? 'currentColor' : 'none'"
      :stroke="circle.fill ? 'none' : 'currentColor'"
    />
    <rect
      v-for="rect in shape.rects"
      :key="`rect-${rect.x}-${rect.y}-${rect.width}-${rect.height}`"
      :x="rect.x"
      :y="rect.y"
      :width="rect.width"
      :height="rect.height"
      :rx="rect.rx"
      fill="none"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from "vue";
import {
  amitiaIconShapes,
  type AmitiaIconName,
  type AmitiaIconShape,
} from "./amitiaIconRegistry";

const props = withDefaults(
  defineProps<{
    name: AmitiaIconName;
    strokeWidth?: number;
  }>(),
  {
    strokeWidth: 1.8,
  },
);

const shape = computed<AmitiaIconShape>(() => amitiaIconShapes[props.name]);
</script>

<style scoped>
.amitia-icon {
  width: 1em;
  height: 1em;
  display: block;
  flex: none;
  overflow: visible;
}
</style>
