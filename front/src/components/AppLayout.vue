<!-- UI Provider boundary: the original Amitia shell lives in BuiltinAppShell.vue. -->
<script setup lang="ts">
import BuiltinAppShell from "./BuiltinAppShell.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
</script>

<template>
  <UIProviderHost capability="app.shell" :fallback="BuiltinAppShell">
    <slot />
  </UIProviderHost>
</template>
