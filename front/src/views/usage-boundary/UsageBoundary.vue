<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="legal-page">
    <div class="page-header">
      <el-button text class="back-btn" @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <div>
        <h2>用户协议与使用边界</h2>
        <p>覆盖服务使用、账号设备、扩展工具、AI 安全边界和开源许可。</p>
      </div>
    </div>

    <el-card shadow="never" class="content-card">
      <section class="legal-section">
        <h3>服务使用</h3>
        <p>你可以在本地模式或自己配置的 Cloud Core 中使用 Amitia。具体可用能力取决于当前部署、模型、扩展、权限和设备运行状态。</p>
        <p>客户端界面显示某项能力并不代表服务端一定可用；实际执行以当前 Core、Device Agent 和相关服务返回结果为准。</p>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>账号与设备</h3>
        <ul>
          <li>账号用于身份鉴权、会话安全和云端设备归属。</li>
          <li>设备绑定、登录会话、设备凭据和解除绑定应通过应用提供的真实接口完成。</li>
          <li>云端模式下业务 WebSocket 连接 Cloud Core，但设备本地 Runtime/Device Agent 仍可以承担设备侧能力。</li>
        </ul>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>扩展与工具</h3>
        <p>扩展包、MCP、Agent Skills、工作流以及其它工具可能访问你授权的数据、网络资源或设备能力。启用前应确认来源、权限范围和运行位置。</p>
        <el-alert type="warning" :closable="false" show-icon>
          <template #title>不要把高权限扩展、工作流或设备控制能力授予不可信来源。</template>
        </el-alert>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>AI 使用边界</h3>
        <el-table :data="limitations" :show-header="false" size="small" class="limit-table">
          <el-table-column width="38">
            <template #default><el-icon color="var(--ac-color-danger)"><CircleCloseFilled /></el-icon></template>
          </el-table-column>
          <el-table-column prop="title" width="180" />
          <el-table-column prop="desc" />
        </el-table>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>安全使用建议</h3>
        <ul>
          <li>不要发送密码、验证码、支付凭据等高敏感信息。</li>
          <li>重要医疗、法律、财务和安全决策应由合格专业人士或可靠来源复核。</li>
          <li>不要利用自动化、设备控制或生成能力实施违法、欺诈、骚扰或未经授权的操作。</li>
          <li>工具调用和设备操作可能产生真实副作用，应根据任务风险保留必要的权限确认与审计。</li>
        </ul>
      </section>

      <el-divider />

      <section class="legal-section">
        <h3>开源许可</h3>
        <p>当前应用报告的许可：<strong>{{ license }}</strong>。第三方依赖分别遵循各自许可证。</p>
        <p>继续使用 Amitia 表示你理解当前部署能力、数据处理边界及以上使用条件。</p>
        <p class="update-note">最后更新：2026 年 9 月</p>
      </section>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ArrowLeft, CircleCloseFilled } from "@element-plus/icons-vue";
import { useApi } from "@/composables/useApi";

const { get } = useApi();
const license = ref("AGPL-3.0-only");
const limitations = [
  { title: "不是真人", desc: "AI 是软件系统，不应被当作真人身份或现实关系的替代。" },
  { title: "输出可能出错", desc: "模型可能产生不完整、过时或错误的内容，关键事实需要复核。" },
  { title: "不替代专业服务", desc: "AI 不能替代医生、律师、财务顾问或其它受监管专业服务。" },
  { title: "工具有真实副作用", desc: "Agent、工作流、扩展和设备操作可能修改文件、调用服务或控制设备，应按权限和审计策略执行。" },
];

onMounted(async () => {
  try {
    const about = await get<Record<string, unknown>>("/api/about");
    if (about?.license) license.value = String(about.license);
  } catch {}
});
</script>

<style scoped>
.legal-page { padding: 20px; }
.page-header { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 16px; }
.page-header h2 { margin: 0; font-size: 18px; font-weight: 600; }
.page-header p { margin: 4px 0 0; color: var(--ac-color-text-secondary); font-size: 13px; }
.back-btn { padding: 4px; }
.content-card { line-height: 1.75; }
.legal-section h3 { margin: 0 0 10px; font-size: 15px; font-weight: 600; }
.legal-section p, .legal-section li { font-size: 14px; color: var(--el-text-color-regular); }
.legal-section p { margin: 0 0 8px; }
.legal-section ul { margin: 4px 0; padding-left: 20px; }
.legal-section li { margin-bottom: 7px; }
.limit-table { margin-top: 8px; }
.limit-table :deep(.el-table__cell) { font-size: 14px; }
.update-note { margin-top: 14px !important; color: var(--el-text-color-secondary) !important; font-size: 12px !important; }
@media (max-width: 768px) { .legal-page { padding: 12px; } }
</style>
