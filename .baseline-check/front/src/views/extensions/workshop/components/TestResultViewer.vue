<template>
  <section v-if="report" class="test-report" aria-live="polite">
    <div class="report-summary">
      <el-tag :type="report.status === 'passed' ? 'success' : 'danger'">{{
        report.status === "passed" ? "测试通过" : "测试失败"
      }}</el-tag
      ><span>{{ report.durationMs }} ms</span
      ><span>{{ report.workflowChecksum.slice(0, 12) }}</span>
    </div>
    <el-timeline>
      <el-timeline-item
        v-for="step in report.stepResults"
        :key="`${step.stepId}-${step.durationMs}`"
        :type="
          step.status === 'succeeded'
            ? 'success'
            : step.status === 'skipped'
              ? 'info'
              : 'danger'
        "
        :timestamp="`${step.durationMs} ms`"
      >
        <strong>{{ step.stepId }}</strong
        ><span class="step-type"
          >{{ step.type }}{{ step.mocked ? " · Mock" : "" }}</span
        >
        <p v-if="step.error">{{ step.error.detail || step.error.message }}</p>
      </el-timeline-item>
    </el-timeline>
    <el-alert
      v-if="report.error"
      :title="report.error.message"
      :description="report.error.detail"
      type="error"
      show-icon
      :closable="false"
    />
    <h4>预计或实际副作用</h4>
    <el-tag
      v-for="effect in report.sideEffects"
      :key="`${effect.type}-${effect.targetId}`"
      :type="effect.confirmed ? 'warning' : 'info'"
      >{{ effect.type
      }}{{ effect.targetId ? ` · ${effect.targetId}` : "" }}</el-tag
    >
  </section>
</template>
<script setup lang="ts">
import type { WorkshopTestReport } from "../../types";

defineProps<{ report?: WorkshopTestReport }>();
</script>
<style scoped>
.test-report {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.report-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--console-text-muted);
  font-variant-numeric: tabular-nums;
}
.step-type {
  margin-left: 8px;
  color: var(--console-text-muted);
}
.test-report p {
  color: var(--el-color-danger);
}
</style>
