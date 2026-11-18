<script setup lang="ts">
import { Teleport } from "vue";
import UIPortalLayer from "./UIPortalLayer.vue";
</script>

<template>
  <Teleport to="#amitia-overlay-root">
    <UIPortalLayer layer="overlay" />
    <UIPortalLayer layer="modal" />
    <UIPortalLayer layer="drawer" />
    <UIPortalLayer layer="toast" />
  </Teleport>
</template>
