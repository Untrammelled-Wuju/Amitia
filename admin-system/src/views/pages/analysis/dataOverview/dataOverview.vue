<script setup lang="ts">
import DataOverview from './DataOverview/DataOverview.vue';
import chartOverView from './chartOverView/chartOverView.vue';
</script>
<template>
  <div id="dataOverview">
    <DataOverview />
    <chartOverView />
  </div>
</template>

<style scoped>
#dataOverview{
  width: 100%;
}
</style>
