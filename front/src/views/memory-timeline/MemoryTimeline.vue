<template><DeviceOwnedDataView title="记忆时间线" initial-kind="memory" mode="timeline" :kinds="TIMELINE_KINDS" /></template>
<script setup lang="ts">
import DeviceOwnedDataView from "@/components/DeviceOwnedDataView.vue";

const TIMELINE_KINDS = ["memory", "working", "episodic", "fact", "profile", "graph", "summary"];
</script>
