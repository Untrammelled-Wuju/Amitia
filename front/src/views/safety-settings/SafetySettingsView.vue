<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="safety-page">
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
              >主动任务中不提及用户标记为敏感或禁止谈论的记忆</span
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
    <el-card shadow="never" class="section-card">
      <template #header
        ><span class="section-title">BDI 硬约束过滤规则</span></template
      >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 10px"
      >
        规则在候选行为仲裁阶段生效：阻断会直接移除该候选，优先会保留候选并排到最前；阻断原因同时作为本轮禁止话题写入提示词。当前 {{ blockRuleCount }} 条阻断、{{ overrideRuleCount }} 条优先。
      </el-alert>
      <el-table
        :data="hardConstraints"
        stripe
        size="small"
        empty-text="暂无约束规则"
      >
        <el-table-column label="规则 ID" width="150">
          <template #default="{ row }">
            <el-input v-model="row.ruleId" size="small" maxlength="40" />
          </template>
        </el-table-column>
        <el-table-column label="候选行为" width="170">
          <template #default="{ row }">
            <el-select v-model="row.candidateKey" size="small" filterable placeholder="选择候选">
              <el-option
                v-for="item in bdiOptions.candidates"
                :key="item.id"
                :label="item.label"
                :value="item.id"
              />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="原因 / 禁止话题">
          <template #default="{ row }">
            <el-input v-model="row.reason" size="small" maxlength="60" />
          </template>
        </el-table-column>
        <el-table-column label="生效方式" width="140">
          <template #default="{ row }">
            <el-select v-model="row.severity" size="small">
              <el-option
                v-for="item in bdiOptions.severities"
                :key="item.id"
                :label="item.label"
                :value="item.id"
              />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="70">
          <template #default="{ $index }">
            <el-button link type="danger" size="small" @click="removeHardConstraint($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 10px; display: flex; gap: 8px">
        <el-button size="small" @click="addHardConstraint">添加规则</el-button>
        <el-button size="small" @click="fetchBdiConfig">刷新</el-button>
        <el-button size="small" type="primary" @click="saveBdiConfig">保存约束</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header
        ><span class="section-title">BDI 软偏好加权</span></template
      >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 10px"
      >
        软偏好只做加权，不阻断候选：贡献值 =（原始分 - 5）/ 5 × 权重，写入该候选的偏好分（默认权重 0.1），最终影响仲裁排序。
      </el-alert>
      <el-table
        :data="softPreferences"
        stripe
        size="small"
        empty-text="暂无偏好维度"
      >
        <el-table-column label="维度" width="200">
          <template #default="{ row }">
            <el-select v-model="row.dimension" size="small" filterable placeholder="选择候选或类别">
              <el-option-group label="具体候选">
                <el-option
                  v-for="item in bdiOptions.dimensions.filter((entry) => entry.group === 'candidate')"
                  :key="item.id"
                  :label="item.label"
                  :value="item.id"
                />
              </el-option-group>
              <el-option-group label="行为类别">
                <el-option
                  v-for="item in bdiOptions.dimensions.filter((entry) => entry.group === 'tag')"
                  :key="item.id"
                  :label="item.label"
                  :value="item.id"
                />
              </el-option-group>
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="原始分" width="110">
          <template #default="{ row }">
            <el-input-number
              v-model="row.rawScore"
              :min="0"
              :max="10"
              size="small"
              controls-position="right"
              style="width: 100px"
            />
          </template>
        </el-table-column>
        <el-table-column label="权重" width="120">
          <template #default="{ row }">
            <el-input-number
              v-model="row.normalizedWeight"
              :min="-1"
              :max="1"
              :step="0.1"
              size="small"
              controls-position="right"
              style="width: 110px"
            />
          </template>
        </el-table-column>
        <el-table-column label="贡献值" width="90">
          <template #default="{ row }">
            <span style="font-size: 13px">{{ preferenceContribution(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="70">
          <template #default="{ $index }">
            <el-button link type="danger" size="small" @click="removeSoftPreference($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 10px; display: flex; gap: 8px">
        <el-button size="small" @click="addSoftPreference">添加维度</el-button>
        <el-button size="small" type="primary" @click="saveBdiConfig"
          >保存偏好设置</el-button
        >
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header
        ><span class="section-title">应对策略选择</span></template
      >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 10px"
      >
        选择的策略直接进入本轮表达计划并写入提示词；未选择时按当前压力与情绪自动选择。
      </el-alert>
      <div class="coping-section">
        <div class="coping-row">
          <span class="coping-label">当前策略</span>
          <el-select v-model="copingSelected" style="width: 200px" size="small">
            <el-option
              v-for="item in bdiOptions.copingStrategies"
              :key="item.id"
              :label="item.label"
              :value="item.id"
            />
          </el-select>
        </div>
        <div class="coping-row">
          <span class="coping-label">备选策略</span>
          <el-select
            v-model="copingAlternatives"
            multiple
            style="width: 320px"
            size="small"
            placeholder="选择备选策略"
          >
            <el-option
              v-for="item in bdiOptions.copingStrategies"
              :key="item.id"
              :label="item.label"
              :value="item.id"
            />
          </el-select>
        </div>
        <div class="coping-row">
          <span class="coping-label">选择理由</span>
          <el-input
            v-model="copingReason"
            type="textarea"
            :rows="2"
            placeholder="描述当前策略的选择理由"
            maxlength="200"
            show-word-limit
            style="flex: 1"
          />
        </div>
      </div>
      <div style="margin-top: 10px">
        <el-button size="small" type="primary" @click="saveBdiConfig"
          >保存应对策略</el-button
        >
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header
        ><span class="section-title">情绪表达控制</span></template
      >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 10px"
      >
        控制 AI 在输出中如何显露、抑制或重构情绪表达；下面的显示强度是本轮情绪强度的整体上限。
      </el-alert>
      <div class="effective-row">
        <el-tag size="small" type="info">当前生效</el-tag>
        <span class="effective-text">{{ effectiveEmotionSummary }}</span>
      </div>
      <div class="emotion-section">
        <div class="emotion-row">
          <span class="emotion-label">表达模式</span>
          <el-radio-group v-model="emotionDisplayMode" size="small">
            <el-radio value="show">正常显露</el-radio>
            <el-radio value="suppress">抑制表达</el-radio>
            <el-radio value="reframe">重构表达</el-radio>
          </el-radio-group>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">内部强度</span>
          <div class="emotion-slider-group">
            <el-slider
              v-model="emotionInternalIntensity"
              :min="0"
              :max="10"
              :step="0.5"
              style="width: 200px"
            />
            <span class="slider-value">{{ emotionInternalIntensity }}</span>
          </div>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">显示强度</span>
          <div class="emotion-slider-group">
            <el-slider
              v-model="emotionDisplayIntensity"
              :min="0"
              :max="10"
              :step="0.5"
              style="width: 200px"
            />
            <span class="slider-value">{{ emotionDisplayIntensity }}</span>
          </div>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">覆盖理由</span>
          <el-input
            v-model="emotionOverrideReason"
            type="textarea"
            :rows="2"
            placeholder="当表达模式非正常显露时，说明原因"
            maxlength="200"
            show-word-limit
            style="flex: 1"
          />
        </div>
      </div>
      <div style="margin-top: 10px">
        <el-button size="small" type="primary" @click="saveBdiConfig"
          >保存情绪配置</el-button
        >
      </div>
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

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">安全事件日志</span>
          <el-button
            text
            size="small"
            @click="clearEvents"
            :disabled="evTotal === 0"
            >清除</el-button
          >
        </div>
      </template>
      <el-table :data="events" stripe size="small" max-height="360">
        <el-table-column
          prop="eventType"
          label="事件类型"
          width="140"
          show-overflow-tooltip
        />
        <el-table-column
          prop="description"
          label="描述"
          show-overflow-tooltip
        />
        <el-table-column prop="direction" label="方向" width="80" />
        <el-table-column label="处理" width="96">
          <template #default="{ row }">
            <el-button
              v-if="row.status !== 'resolved'"
              link
              type="primary"
              size="small"
              @click="handleEvent(row)"
              >确认处理</el-button
            >
            <el-tag v-else type="success" size="small">已处理</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="150">
          <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-if="evTotal > 20"
        v-model:current-page="evPage"
        :page-size="20"
        :total="evTotal"
        layout="prev,next"
        size="small"
        @current-change="fetchEvents"
        style="margin-top: 10px; justify-content: center"
      />
    </el-card>

  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "../../composables/useApi";

const { get, put, del } = useApi();


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
  revision: 0,
});

const auditLogs = ref<any[]>([]);
const events = ref<any[]>([]);
const evPage = ref(1);
const evTotal = ref(0);

const hardConstraints = ref<
  Array<{
    ruleId: string;
    candidateKey: string;
    reason: string;
    severity: "block" | "override";
  }>
>([]);
const softPreferences = ref<
  Array<{
    dimension: string;
    rawScore: number;
    normalizedWeight: number;
    contribution: number;
  }>
>([]);
const copingSelected = ref("active");
const copingAlternatives = ref<string[]>([]);
const copingReason = ref("");
const emotionDisplayMode = ref<"show" | "suppress" | "reframe">("show");
const emotionInternalIntensity = ref(5);
const emotionDisplayIntensity = ref(5);
const emotionOverrideReason = ref("");
const bdiRevision = ref(0);

interface BdiOptionItem {
  id: string;
  label: string;
  group?: string;
}

const bdiOptions = ref<{
  candidates: BdiOptionItem[];
  dimensions: BdiOptionItem[];
  severities: BdiOptionItem[];
  copingStrategies: BdiOptionItem[];
  displayModes: BdiOptionItem[];
}>({
  candidates: [],
  dimensions: [],
  severities: [],
  copingStrategies: [],
  displayModes: [],
});

const legacyCoping: Record<string, string> = {
  active: "problem_solving",
  support: "seek_support",
  reframe: "reappraisal",
  avoid: "avoidance",
  suppress: "suppression",
  accept: "acceptance",
};

const blockRuleCount = computed(
  () => hardConstraints.value.filter((rule) => rule.severity === "block").length,
);
const overrideRuleCount = computed(
  () =>
    hardConstraints.value.filter((rule) => rule.severity === "override").length,
);
const effectiveEmotionSummary = computed(() => {
  const mode =
    bdiOptions.value.displayModes.find(
      (item) => item.id === emotionDisplayMode.value,
    )?.label || "如实表露";
  return `${mode}；显示强度上限 ${emotionDisplayIntensity.value}/10；负面情绪上限 ${config.negativeEmotionCap}/10（关切语气）；亲密表达上限 ${config.intimacyExpressionCap}/10（温柔或柔和语气）`;
});

function preferenceContribution(row: {
  rawScore: number;
  normalizedWeight: number;
}) {
  const raw = Number(row.rawScore ?? 5);
  const weight = Number(row.normalizedWeight ?? 0);
  const value = weight === 0 ? (raw - 5) / 5 : weight * ((raw - 5) / 5);
  return value.toFixed(2);
}

function addHardConstraint() {
  hardConstraints.value.push({
    ruleId: "",
    candidateKey: "",
    reason: "",
    severity: "block",
  });
}

function removeHardConstraint(index: number) {
  hardConstraints.value.splice(index, 1);
}

function removeSoftPreference(index: number) {
  softPreferences.value.splice(index, 1);
}

function addSoftPreference() {
  softPreferences.value.push({
    dimension: "",
    rawScore: 5,
    normalizedWeight: 0,
    contribution: 0,
  });
}

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
        revision: Number(data.revision || 0),
      });
    }
  } catch (error) {
  console.error("[original-error]", error);
}
}

async function saveConfig() {
  saving.value = true;
  try {
    const updated = await put<any>("/api/safety/config", {
      ...config,
      expectedRevision: config.revision,
    });
    config.revision = Number(updated?.revision || config.revision + 1);
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
  } catch (error) {
  console.error("[original-error]", error);
auditLogs.value = [];
  }
}

async function fetchBdiConfig() {
  try {
    const r = await get<any>("/api/safety/bdi-config");
    if (r?.hardConstraints) hardConstraints.value = r.hardConstraints;
    if (r?.softPreferences) softPreferences.value = r.softPreferences;
    if (r?.copingStrategy) {
      const selected = String(r.copingStrategy.selected || "problem_solving");
      copingSelected.value = legacyCoping[selected] || selected;
      copingAlternatives.value = (r.copingStrategy.alternatives || []).map(
        (value: string) => legacyCoping[value] || value,
      );
      copingReason.value = r.copingStrategy.selectionReason || "";
    }
    if (r?.emotionExpression) {
      emotionDisplayMode.value = r.emotionExpression.displayMode || "show";
      emotionInternalIntensity.value =
        r.emotionExpression.internalIntensity ?? 5;
      emotionDisplayIntensity.value = r.emotionExpression.displayIntensity ?? 5;
      emotionOverrideReason.value = r.emotionExpression.overrideReason || "";
    }
    bdiRevision.value = Number(r?.revision || 0);
  } catch (error) {
  console.error("[original-error]", error);
}
}

async function fetchBdiOptions() {
  try {
    const data = await get<any>("/api/safety/bdi-options");
    if (data) {
      bdiOptions.value = {
        candidates: data.candidates || [],
        dimensions: data.dimensions || [],
        severities: data.severities || [],
        copingStrategies: data.copingStrategies || [],
        displayModes: data.displayModes || [],
      };
    }
  } catch (error) {
  console.error("[original-error]", error);
}
}

async function saveBdiConfig() {
  for (const [index, rule] of hardConstraints.value.entries()) {
    if (!rule.ruleId.trim()) {
      ElMessage.error(`第 ${index + 1} 条规则缺少规则 ID`);
      return;
    }
    if (!rule.candidateKey) {
      ElMessage.error(`第 ${index + 1} 条规则未选择候选行为`);
      return;
    }
    if (rule.severity === "block" && !rule.reason.trim()) {
      ElMessage.error(`第 ${index + 1} 条阻断规则必须填写原因`);
      return;
    }
  }
  if (softPreferences.value.some((item) => !item.dimension)) {
    ElMessage.error("软偏好存在未选择维度的行");
    return;
  }
  try {
    const updated = await put<any>("/api/safety/bdi-config", {
      hardConstraints: hardConstraints.value.map((rule) => ({
        ...rule,
        ruleId: rule.ruleId.trim(),
        reason: rule.reason.trim(),
      })),
      softPreferences: softPreferences.value.map((sp) => ({
        dimension: sp.dimension,
        rawScore: Number(sp.rawScore),
        normalizedWeight: Number(sp.normalizedWeight),
        contribution: Number(preferenceContribution(sp)),
      })),
      copingStrategy: {
        selected: copingSelected.value,
        alternatives: copingAlternatives.value,
        selectionReason: copingReason.value,
      },
      emotionExpression: {
        displayMode: emotionDisplayMode.value,
        internalIntensity: Number(emotionInternalIntensity.value),
        displayIntensity: Number(emotionDisplayIntensity.value),
        overrideReason: emotionOverrideReason.value,
      },
      expectedRevision: bdiRevision.value,
    });
    bdiRevision.value = Number(updated?.revision || bdiRevision.value + 1);
    ElMessage.success("BDI 配置已保存");
  } catch (error: any) {
    ElMessage.error(error?.message || "BDI 配置保存失败");
  }
}

async function fetchEvents() {
  try {
    const r = await get<any>("/api/safety/events", {
      page: evPage.value,
      pageSize: 20,
    });
    events.value = r?.items || [];
    evTotal.value = r?.total || 0;
  } catch (error) {
  console.error("[original-error]", error);
}
}

async function handleEvent(event: any) {
  if (!event?.id || !Number.isSafeInteger(event.revision)) return;
  try {
    await put(`/api/safety/events/${encodeURIComponent(event.id)}/handle`, {
      expectedRevision: event.revision,
    });
    ElMessage.success("安全事件已确认处理");
    await fetchEvents();
  } catch (error: any) {
    ElMessage.error(error?.message || "安全事件处理失败");
  }
}

async function clearEvents() {
  await ElMessageBox.confirm("确定清除所有安全事件日志？", "提示", {
    type: "warning",
  });
  try {
    await del("/api/safety/events");
    ElMessage.success("已清除");
    fetchEvents();
  } catch (error) {
  console.error("[original-error]", error);
}
}

function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch (error) {
  console.error("[original-error]", error);
return d;
  }
}


onMounted(() => {
  fetchConfig();
  fetchBdiOptions();
  fetchBdiConfig();
  fetchEvents();
  fetchAuditLogs();
});
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}
.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}


.coping-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.coping-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.coping-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  min-width: 80px;
  padding-top: 4px;
}

.emotion-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.emotion-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.emotion-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  min-width: 80px;
}
.emotion-slider-group {
  display: flex;
  align-items: center;
  gap: 8px;
}
.slider-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  min-width: 24px;
  text-align: right;
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
.effective-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.effective-text {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
}
</style>
