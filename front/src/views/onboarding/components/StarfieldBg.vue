<template>
  <div class="starfield" ref="starfieldRef"></div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";

const starfieldRef = ref<HTMLElement | null>(null);

onMounted(() => {
  const field = starfieldRef.value;
  if (!field) return;
  for (let i = 0; i < 56; i++) {
    const s = document.createElement("i");
    s.className = "star";
    s.style.left = Math.random() * 100 + "%";
    s.style.top = Math.random() * 100 + "%";
    s.style.setProperty("--dur", 12 + Math.random() * 24 + "s");
    s.style.setProperty("--op", (0.08 + Math.random() * 0.25).toFixed(2));
    s.style.setProperty("--dx", -18 + Math.random() * 36 + "px");
    s.style.animationDelay = -Math.random() * 20 + "s";
    field.appendChild(s);
  }
});
</script>
