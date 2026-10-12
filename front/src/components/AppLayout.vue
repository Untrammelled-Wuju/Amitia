<!-- Root Slot boundary: every replaceable provider surface hangs from this tree. -->
<script setup lang="ts">
import BuiltinAppShell from "./BuiltinAppShell.vue";
import ExtensionSlot from "./extension/ExtensionSlot.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
</script>

<template>
  <div class="app-shell-target" data-amitia-ui="app.shell">
    <ExtensionSlot slot-id="root" fallback="default" layout="stack" surface-role="main" bare>
      <template #default>
        <UIProviderHost capability="app.shell" :fallback="BuiltinAppShell">
          <slot />
        </UIProviderHost>
      </template>
    </ExtensionSlot>
  </div>
</template>

<style scoped>
.app-shell-target {
  height: 100%;
}
</style>
