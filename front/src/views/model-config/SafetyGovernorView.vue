<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="safety-governor-page">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      style="margin-bottom: 16px"
    >
      <template #title
        >安全调控器在决策前后独立校验每条回复，确保不违反安全、依赖、操控和隐私边界。规则不由角色人格配置覆盖。</template
      >
    </el-alert>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">安全规则</span>
          <el-button
            type="primary"
            size="small"
            @click="saveConfig"
            :loading="saving"
            >保存配置</el-button
          >
        </div>
      </template>

      <el-form label-position="top" size="small">
        <el-divider content-position="left">表达边界</el-divider>

        <el-form-item label="禁止情绪绑架">
          <div class="switch-row">
            <el-switch v-model="config.preventEmotionalBlackmail" />
            <span class="switch-hint"
              >不允许使用内疚、自责、牺牲式表达操控用户情绪</span
            >
          </div>
        </el-form-item>

        <el-form-item label="禁止排他依赖">
          <div class="switch-row">
            <el-switch v-model="config.preventExclusiveDependency" />
            <span class="switch-hint"
              >不允许暗示"只有我懂你"、"离开我你会..."等排他绑定</span
            >
          </div>
        </el-form-item>

        <el-form-item label="禁止现实关系隔离">
          <div class="switch-row">
            <el-switch v-model="config.preventRealityIsolation" />
            <span class="switch-hint"
              >不允许劝说用户疏远现实社交、家庭或伴侣</span
            >
          </div>
        </el-form-item>

        <el-form-item label="禁止惩罚性表达">
          <div class="switch-row">
            <el-switch v-model="config.preventPunitiveExpression" />
            <span class="switch-hint"
              >不允许沉默、冷漠、阴阳怪气作为对用户行为的回应策略</span
            >
          </div>
        </el-form-item>

        <el-divider content-position="left">内容过滤</el-divider>

        <el-form-item label="禁止冒充真人">
          <div class="switch-row">
            <el-switch v-model="config.preventPretendingHuman" />
            <span class="switch-hint"
              >不允许声称自己是真人、有真实身体或现实身份</span
            >
          </div>
        </el-form-item>

        <el-form-item label="禁止敏感主动提及">
          <div class="switch-row">
            <el-switch v-model="config.preventSensitiveProactiveMention" />
            <span class="switch-hint"
              >主动消息中不提及用户标记为敏感或禁止谈论的记忆</span
            >
          </div>
        </el-form-item>

        <el-form-item label="成人内容限制">
          <div class="switch-row">
            <el-switch v-model="config.restrictAdultContent" />
            <span class="switch-hint"
              >阻止色情、暴力、自残等成人化内容输出</span
            >
          </div>
        </el-form-item>

        <el-divider content-position="left">情绪表达上限</el-divider>

        <el-form-item label="负面情绪强度上限">
          <div class="slider-row">
            <el-slider
              v-model="config.negativeEmotionCap"
              :min="0"
              :max="10"
              :step="1"
              show-input
              style="width: 200px"
            />
            <span class="slider-hint"
              >0=完全禁止负面表达，10=不限制。建议3-5</span
            >
          </div>
        </el-form-item>

        <el-form-item label="亲密表达强度上限">
          <div class="slider-row">
            <el-slider
              v-model="config.intimacyExpressionCap"
              :min="0"
              :max="10"
              :step="1"
              show-input
              style="width: 200px"
            />
            <span class="slider-hint"
              >0=完全禁止亲密表达，10=不限制。建议5-7</span
            >
          </div>
        </el-form-item>

        <el-divider content-position="left">安全行为</el-divider>

        <el-form-item label="违规处理方式">
          <el-radio-group v-model="config.violationAction">
            <el-radio value="block">阻止并替换为安全回复</el-radio>
            <el-radio value="rewrite">改写违规内容</el-radio>
            <el-radio value="audit_only">仅记录不阻止</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="审核日志保留天数">
          <div class="slider-row">
            <el-input-number
              v-model="config.auditLogRetentionDays"
              :min="1"
              :max="365"
              size="small"
            />
            <span class="slider-hint">超过天数的审计日志自动清理</span>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="auditLogs.length > 0" shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">最近审计记录</span>
          <el-button size="small" @click="fetchAuditLogs">刷新</el-button>
        </div>
      </template>
      <el-table
        :data="auditLogs"
        stripe
        size="small"
        max-height="300"
        empty-text="暂无记录"
      >
        <el-table-column prop="time" label="时间" width="160" />
        <el-table-column prop="ruleId" label="规则" width="120" />
        <el-table-column prop="action" label="动作" width="80" />
        <el-table-column
          prop="description"
          label="描述"
          show-overflow-tooltip
        />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../composables/useApi";

const { get, put } = useApi();

const saving = ref(false);

const config = reactive({
  preventEmotionalBlackmail: true,
  preventExclusiveDependency: true,
  preventRealityIsolation: true,
  preventPunitiveExpression: true,
  preventPretendingHuman: true,
  preventSensitiveProactiveMention: true,
  restrictAdultContent: true,
  negativeEmotionCap: 5,
  intimacyExpressionCap: 7,
  violationAction: "block",
  auditLogRetentionDays: 30,
});

const auditLogs = ref<any[]>([]);

async function fetchConfig() {
  try {
    const data = await get<any>("/api/safety/config");
    if (data) {
      Object.assign(config, {
        preventEmotionalBlackmail: data.preventEmotionalBlackmail ?? true,
        preventExclusiveDependency: data.preventExclusiveDependency ?? true,
        preventRealityIsolation: data.preventRealityIsolation ?? true,
        preventPunitiveExpression: data.preventPunitiveExpression ?? true,
        preventPretendingHuman: data.preventPretendingHuman ?? true,
        preventSensitiveProactiveMention:
          data.preventSensitiveProactiveMention ?? true,
        restrictAdultContent: data.restrictAdultContent ?? true,
        negativeEmotionCap: data.negativeEmotionCap ?? 5,
        intimacyExpressionCap: data.intimacyExpressionCap ?? 7,
        violationAction: data.violationAction ?? "block",
        auditLogRetentionDays: data.auditLogRetentionDays ?? 30,
      });
    }
  } catch {
    // 使用默认值
  }
}

async function saveConfig() {
  saving.value = true;
  try {
    await put("/api/safety/config", { ...config });
    ElMessage.success("安全配置已保存");
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function fetchAuditLogs() {
  try {
    const data = await get<any[]>("/api/safety/audit-logs");
    auditLogs.value = data || [];
  } catch {
    auditLogs.value = [];
  }
}

onMounted(() => {
  fetchConfig();
  fetchAuditLogs();
});
</script>

<style scoped>
.safety-governor-page {
  padding: 0;
}
.section-card {
  margin-bottom: 16px;
}
.section-title {
  font-weight: 600;
  font-size: 15px;
}
.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.switch-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.switch-hint {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
.slider-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.slider-hint {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
</style>
