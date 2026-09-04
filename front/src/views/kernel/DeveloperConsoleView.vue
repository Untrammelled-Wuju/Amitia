<template>
  <div class="developer-console">
    <div class="console-header">
      <div class="header-left">
        <h2>开发者诊断控制台</h2>
        <p class="subtitle">Developer Console — 扩展运行时诊断、调用追踪、事件监控与导出</p>
      </div>
      <div class="header-right">
        <el-button @click="handleExport" :icon="Download" :loading="exporting">导出诊断包</el-button>
        <el-button @click="refreshAll" :icon="Refresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <div class="overview-cards">
      <el-card v-for="metric in metrics" :key="metric.label" class="metric-card" shadow="hover">
        <div class="metric-value" :class="metric.class">{{ metric.value }}</div>
        <div class="metric-label">{{ metric.label }}</div>
      </el-card>
    </div>

    <el-tabs v-model="activeTab" class="console-tabs" @tab-change="onTabChange">
      <el-tab-pane label="总览" name="overview">
        <el-card>
          <template #header>Top 扩展</template>
          <el-table :data="overview?.topExtensions || []" border size="small" v-loading="loading">
            <el-table-column prop="extensionId" label="扩展 ID" min-width="200" show-overflow-tooltip />
            <el-table-column prop="publisher" label="发布者" width="140" show-overflow-tooltip />
            <el-table-column prop="version" label="版本" width="100" />
            <el-table-column prop="moduleCount" label="模块数" width="80" />
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="errorCount" label="错误数" width="80" />
            <el-table-column prop="invocationCount" label="调用数" width="80" />
          </el-table>
          <el-empty v-if="!loading && (overview?.topExtensions || []).length === 0" description="暂无扩展数据" />
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="调用" name="invocations">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadInvocations">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-select v-model="invocationSeverity" placeholder="按状态过滤" clearable style="width: 160px" @change="loadInvocations">
            <el-option label="运行中" value="running" />
            <el-option label="成功" value="succeeded" />
            <el-option label="失败" value="failed" />
            <el-option label="超时" value="timeout" />
            <el-option label="取消" value="cancelled" />
          </el-select>
          <el-button @click="loadInvocations" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="invocations" border size="small" v-loading="loading">
          <el-table-column prop="id" label="调用 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展 ID" min-width="160" show-overflow-tooltip />
          <el-table-column prop="moduleId" label="模块 ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="toolId" label="工具 ID" min-width="140" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="invocationStatusType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="durationMs" label="耗时(ms)" width="100" />
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column label="完成时间" width="180">
            <template #default="{ row }">{{ row.completedAt ? formatDate(row.completedAt) : '-' }}</template>
          </el-table-column>
          <el-table-column prop="error" label="错误" min-width="200" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!loading && invocations.length === 0" description="暂无调用记录" />
      </el-tab-pane>

      <el-tab-pane label="Host API" name="hostApiAudits">
        <div class="tab-toolbar host-audit-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 220px" @change="loadHostAPIAudits">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-input v-model="hostMethod" placeholder="Method" clearable style="width: 200px" @keyup.enter="loadHostAPIAudits" />
          <el-select v-model="hostResult" placeholder="结果" clearable style="width: 130px" @change="loadHostAPIAudits">
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
            <el-option label="已取消" value="cancelled" />
            <el-option label="超时" value="timed_out" />
            <el-option label="执行中" value="started" />
          </el-select>
          <el-input v-model="hostTraceId" placeholder="Trace ID" clearable style="width: 220px" @keyup.enter="loadHostAPIAudits" />
          <el-button @click="loadHostAPIAudits" :icon="Refresh">刷新</el-button>
          <span class="audit-total">共 {{ hostApiAuditTotal }} 条</span>
        </div>
        <el-table :data="hostApiAudits" border size="small" v-loading="loading">
          <el-table-column prop="startedAt" label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column prop="extensionId" label="扩展 ID" min-width="170" show-overflow-tooltip />
          <el-table-column prop="moduleId" label="模块 ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="method" label="Method" min-width="190" show-overflow-tooltip />
          <el-table-column prop="phase" label="阶段" width="110" />
          <el-table-column label="结果" width="100">
            <template #default="{ row }">
              <el-tag :type="hostResultType(row.result)" size="small">{{ row.result || "unknown" }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sideEffect" label="副作用" width="130" show-overflow-tooltip />
          <el-table-column prop="traceId" label="Trace ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="callId" label="Call ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="inputMasked" label="脱敏输入" min-width="240" show-overflow-tooltip />
          <el-table-column prop="errorMessage" label="错误" min-width="220" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!loading && hostApiAudits.length === 0" description="暂无 Host API 审计记录" />
      </el-tab-pane>

      <el-tab-pane label="事件" name="events">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadEvents">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadEvents" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="events" border size="small" v-loading="loading">
          <el-table-column prop="id" label="事件 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="type" label="类型" min-width="160" show-overflow-tooltip />
          <el-table-column prop="source" label="来源" min-width="140" show-overflow-tooltip />
          <el-table-column label="已消费" width="80">
            <template #default="{ row }">
              <el-tag :type="row.consumed ? 'success' : 'warning'" size="small">{{ row.consumed ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="consumer" label="消费者" min-width="140" show-overflow-tooltip />
          <el-table-column label="触发时间" width="180">
            <template #default="{ row }">{{ formatDate(row.emittedAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && events.length === 0" description="暂无事件记录" />
      </el-tab-pane>

      <el-tab-pane label="钩子" name="hooks">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadHooks">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadHooks" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="hooks" border size="small" v-loading="loading">
          <el-table-column prop="id" label="钩子 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="pipeline" label="管道" min-width="140" show-overflow-tooltip />
          <el-table-column prop="stage" label="阶段" width="120" />
          <el-table-column prop="phase" label="相位" width="100" />
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="durationMs" label="耗时(ms)" width="100" />
          <el-table-column label="否决" width="80">
            <template #default="{ row }">
              <el-tag :type="row.vetoed ? 'danger' : 'success'" size="small">{{ row.vetoed ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="调用时间" width="180">
            <template #default="{ row }">{{ formatDate(row.invokedAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && hooks.length === 0" description="暂无钩子记录" />
      </el-tab-pane>

      <el-tab-pane label="任务" name="tasks">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadTasks">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadTasks" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="tasks" border size="small" v-loading="loading">
          <el-table-column prop="id" label="记录 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="taskId" label="任务 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="taskStatusType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="进度" width="140">
            <template #default="{ row }">
              <el-progress :percentage="Math.round(row.progress * 100)" :stroke-width="10" />
            </template>
          </el-table-column>
          <el-table-column prop="attempt" label="尝试" width="60" />
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column label="完成时间" width="180">
            <template #default="{ row }">{{ row.completedAt ? formatDate(row.completedAt) : '-' }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && tasks.length === 0" description="暂无任务记录" />
      </el-tab-pane>

      <el-tab-pane label="日志" name="logs">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadLogs">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-select v-model="logSeverity" placeholder="按级别过滤" clearable style="width: 160px" @change="loadLogs">
            <el-option label="Debug" value="debug" />
            <el-option label="Info" value="info" />
            <el-option label="Warn" value="warn" />
            <el-option label="Error" value="error" />
            <el-option label="Fatal" value="fatal" />
          </el-select>
          <el-button @click="loadLogs" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="logs" border size="small" v-loading="loading">
          <el-table-column label="级别" width="80">
            <template #default="{ row }">
              <el-tag :type="logLevelType(row.level)" size="small">{{ row.level }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="message" label="消息" min-width="300" show-overflow-tooltip />
          <el-table-column label="字段" width="80">
            <template #default="{ row }">
              <el-button v-if="row.fields && Object.keys(row.fields).length > 0" size="small" link @click="showFields(row)">查看</el-button>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.at) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && logs.length === 0" description="暂无日志记录" />
      </el-tab-pane>

      <el-tab-pane label="生命周期" name="lifecycle">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadLifecycle">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadLifecycle" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="lifecycle" border size="small" v-loading="loading">
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="stage" label="阶段" width="140" />
          <el-table-column label="成功" width="80">
            <template #default="{ row }">
              <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="200" show-overflow-tooltip />
          <el-table-column label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.at) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && lifecycle.length === 0" description="暂无生命周期事件" />
      </el-tab-pane>

      <el-tab-pane label="性能" name="performance">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadPerformance">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadPerformance" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="performance" border size="small" v-loading="loading">
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="metric" label="指标" min-width="200" show-overflow-tooltip />
          <el-table-column prop="value" label="值" width="120" />
          <el-table-column label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.at) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && performance.length === 0" description="暂无性能指标" />
      </el-tab-pane>

      <el-tab-pane label="权限" name="permissions">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadPermissions">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadPermissions" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="permissions" border size="small" v-loading="loading">
          <el-table-column prop="permission" label="权限" min-width="200" show-overflow-tooltip />
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="scope" label="作用域" min-width="160" show-overflow-tooltip />
          <el-table-column label="已授予" width="80">
            <template #default="{ row }">
              <el-tag :type="row.granted ? 'success' : 'danger'" size="small">{{ row.granted ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="200" show-overflow-tooltip />
          <el-table-column label="授予时间" width="180">
            <template #default="{ row }">{{ formatDate(row.grantedAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && permissions.length === 0" description="暂无权限授予记录" />
      </el-tab-pane>

      <el-tab-pane label="作用域" name="scopes">
        <div class="tab-toolbar">
          <el-button @click="loadScopes" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="scopes" border size="small" v-loading="loading">
          <el-table-column prop="scope" label="作用域" min-width="200" show-overflow-tooltip />
          <el-table-column prop="characterId" label="角色 ID" min-width="160" show-overflow-tooltip />
          <el-table-column prop="conversationId" label="会话 ID" min-width="160" show-overflow-tooltip />
          <el-table-column prop="userId" label="用户 ID" min-width="160" show-overflow-tooltip />
          <el-table-column label="活跃" width="80">
            <template #default="{ row }">
              <el-tag :type="row.active ? 'success' : 'info'" size="small">{{ row.active ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && scopes.length === 0" description="暂无作用域记录" />
      </el-tab-pane>

      <el-tab-pane label="资源" name="resources">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadResources">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadResources" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="resources" border size="small" v-loading="loading">
          <el-table-column prop="handle" label="句柄" min-width="200" show-overflow-tooltip />
          <el-table-column prop="kind" label="类型" width="120" />
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column label="大小" width="120">
            <template #default="{ row }">{{ formatBytes(row.size) }}</template>
          </el-table-column>
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">{{ formatDate(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && resources.length === 0" description="暂无资源记录" />
      </el-tab-pane>

      <el-tab-pane label="存储" name="storage">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadStorage">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadStorage" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="storage" border size="small" v-loading="loading">
          <el-table-column prop="namespace" label="命名空间" min-width="200" show-overflow-tooltip />
          <el-table-column prop="key" label="键" min-width="200" show-overflow-tooltip />
          <el-table-column prop="version" label="版本" width="80" />
          <el-table-column prop="scope" label="作用域" min-width="140" show-overflow-tooltip />
          <el-table-column label="更新时间" width="180">
            <template #default="{ row }">{{ formatDate(row.updatedAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && storage.length === 0" description="暂无存储条目" />
      </el-tab-pane>

      <el-tab-pane label="UI 会话" name="uiSessions">
        <div class="tab-toolbar">
          <el-select v-model="filterExtension" placeholder="按扩展过滤" clearable filterable style="width: 240px" @change="loadUISessions">
            <el-option v-for="ext in extensionOptions" :key="ext" :label="ext" :value="ext" />
          </el-select>
          <el-button @click="loadUISessions" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="uiSessions" border size="small" v-loading="loading">
          <el-table-column prop="id" label="会话 ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="contribution" label="贡献" min-width="160" show-overflow-tooltip />
          <el-table-column prop="origin" label="来源" width="120" />
          <el-table-column prop="cspViolations" label="CSP 违规" width="100" />
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column label="最后活跃" width="180">
            <template #default="{ row }">{{ formatDate(row.lastActive) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && uiSessions.length === 0" description="暂无 UI 会话" />
      </el-tab-pane>

      <el-tab-pane label="迁移" name="migration">
        <div class="tab-toolbar">
          <el-button @click="loadMigration" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="migration" border size="small" v-loading="loading">
          <el-table-column prop="stage" label="阶段" min-width="160" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="migrationStatusType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="details" label="详情" min-width="300" show-overflow-tooltip />
          <el-table-column label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.at) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && migration.length === 0" description="暂无迁移记录" />
      </el-tab-pane>

      <el-tab-pane label="兼容性" name="compatibility">
        <div class="tab-toolbar">
          <el-button @click="loadCompatibility" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="compatibility" border size="small" v-loading="loading">
          <el-table-column prop="extension" label="扩展" min-width="160" show-overflow-tooltip />
          <el-table-column prop="required" label="要求版本" width="120" />
          <el-table-column prop="host" label="宿主版本" width="120" />
          <el-table-column label="兼容" width="80">
            <template #default="{ row }">
              <el-tag :type="row.ok ? 'success' : 'danger'" size="small">{{ row.ok ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="240" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!loading && compatibility.length === 0" description="暂无兼容性记录" />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="fieldsVisible" title="日志字段" width="600px">
      <el-descriptions :column="1" border v-if="currentFields">
        <el-descriptions-item v-for="(value, key) in currentFields" :key="key" :label="String(key)">
          {{ typeof value === 'object' ? JSON.stringify(value) : String(value) }}
        </el-descriptions-item>
      </el-descriptions>
      <el-empty v-else description="无字段数据" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { ElMessage } from "element-plus";
import { Refresh, Download } from "@element-plus/icons-vue";
import {
  fetchOverview,
  fetchInvocations,
  fetchEvents,
  fetchHooks,
  fetchTasks,
  fetchLogs,
  fetchLifecycle,
  fetchPerformance,
  fetchPermissions,
  fetchScopes,
  fetchResources,
  fetchStorage,
  fetchUISessions,
  fetchMigration,
  fetchCompatibility,
  fetchHostAPIAudits,
  exportDiagnostics,
  type DevConsoleOverview,
  type InvocationRecord,
  type EventRecord,
  type HookRecord,
  type TaskRecord,
  type LogEntry,
  type LifecycleEventRecord,
  type PerformanceRecord,
  type PermissionGrantRecord,
  type ScopeRecord,
  type ResourceRecord,
  type StorageEntryRecord,
  type UISessionRecord,
  type MigrationRecord,
  type CompatibilityRecord,
  type HostAPIAuditEntry,
} from "./dev-console-api";

const loading = ref(false);
const exporting = ref(false);
const activeTab = ref("overview");

const overview = ref<DevConsoleOverview | null>(null);
const invocations = ref<InvocationRecord[]>([]);
const events = ref<EventRecord[]>([]);
const hooks = ref<HookRecord[]>([]);
const tasks = ref<TaskRecord[]>([]);
const logs = ref<LogEntry[]>([]);
const lifecycle = ref<LifecycleEventRecord[]>([]);
const performance = ref<PerformanceRecord[]>([]);
const permissions = ref<PermissionGrantRecord[]>([]);
const scopes = ref<ScopeRecord[]>([]);
const resources = ref<ResourceRecord[]>([]);
const storage = ref<StorageEntryRecord[]>([]);
const uiSessions = ref<UISessionRecord[]>([]);
const migration = ref<MigrationRecord[]>([]);
const compatibility = ref<CompatibilityRecord[]>([]);
const hostApiAudits = ref<HostAPIAuditEntry[]>([]);
const hostApiAuditTotal = ref(0);

const filterExtension = ref("");
const invocationSeverity = ref("");
const logSeverity = ref("");
const hostMethod = ref("");
const hostResult = ref("");
const hostTraceId = ref("");

const fieldsVisible = ref(false);
const currentFields = ref<Record<string, unknown> | null>(null);

let refreshTimer: ReturnType<typeof setInterval> | null = null;

const extensionOptions = computed(() => {
  const set = new Set<string>();
  for (const ext of overview.value?.topExtensions || []) {
    if (ext.extensionId) set.add(ext.extensionId);
  }
  for (const rec of invocations.value) {
    if (rec.extensionId) set.add(rec.extensionId);
  }
  for (const rec of hooks.value) {
    if (rec.extension) set.add(rec.extension);
  }
  for (const rec of tasks.value) {
    if (rec.extension) set.add(rec.extension);
  }
  for (const rec of logs.value) {
    if (rec.extension) set.add(rec.extension);
  }
  for (const rec of permissions.value) {
    if (rec.extension) set.add(rec.extension);
  }
  for (const rec of hostApiAudits.value) {
    if (rec.extensionId) set.add(rec.extensionId);
  }
  return Array.from(set).sort();
});

const metrics = computed(() => {
  const o = overview.value;
  if (!o) return [];
  return [
    { label: "扩展数", value: o.extensions, class: "" },
    { label: "模块数", value: o.modules, class: "" },
    { label: "活跃调用", value: o.activeInvocations, class: "" },
    { label: "Host API 调用", value: o.hostApiCalls, class: "" },
    { label: "近5分钟事件", value: o.eventsLast5Min, class: "" },
    { label: "钩子调用", value: o.hookInvocations, class: "" },
    { label: "活跃任务", value: o.activeTasks, class: "" },
    { label: "UI 会话", value: o.activeUiSessions, class: "" },
    { label: "存储条目", value: o.storageEntries, class: "" },
    { label: "权限授予", value: o.permissionGrants, class: "" },
    { label: "活跃作用域", value: o.activeScopes, class: "" },
    { label: "资源", value: o.resources, class: "" },
    { label: "错误", value: o.errors, class: "danger" },
    { label: "警告", value: o.warnings, class: "warn" },
    { label: "生命周期事件", value: o.lifecycleEvents, class: "" },
    { label: "兼容性问题", value: o.compatibilityIssues, class: o.compatibilityIssues > 0 ? "danger" : "" },
  ];
});

function statusTagType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "active" || status === "running") return "success";
  if (status === "disabled" || status === "error") return "danger";
  if (status === "pending" || status === "idle") return "warning";
  return "info";
}

function invocationStatusType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "succeeded") return "success";
  if (status === "running") return "warning";
  if (status === "failed" || status === "timeout") return "danger";
  return "info";
}

function hostResultType(result: string): "success" | "warning" | "danger" | "info" {
  const value = String(result || "").toLowerCase();
  if (["success", "succeeded", "ok", "allowed"].includes(value)) return "success";
  if (["denied", "blocked", "cancelled", "started"].includes(value)) return "warning";
  if (["error", "failed", "timeout", "timed_out"].includes(value)) return "danger";
  return "info";
}

function taskStatusType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "completed" || status === "succeeded") return "success";
  if (status === "running" || status === "pending") return "warning";
  if (status === "failed" || status === "cancelled") return "danger";
  return "info";
}

function logLevelType(level: string): "success" | "warning" | "danger" | "info" {
  const l = level.toLowerCase();
  if (l === "error" || l === "fatal") return "danger";
  if (l === "warn" || l === "warning") return "warning";
  if (l === "info") return "success";
  return "info";
}

function migrationStatusType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "completed" || status === "done") return "success";
  if (status === "running" || status === "pending") return "warning";
  if (status === "failed") return "danger";
  return "info";
}

function formatDate(s: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

function formatBytes(bytes: number): string {
  if (!bytes) return "0";
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}GB`;
}

function buildFilter() {
  const filter: Record<string, string> = {};
  if (filterExtension.value) filter.extension = filterExtension.value;
  return filter;
}

function buildInvocationFilter() {
  const filter: Record<string, string> = {};
  if (filterExtension.value) filter.extension = filterExtension.value;
  if (invocationSeverity.value) filter.severity = invocationSeverity.value;
  return filter;
}

function buildLogFilter() {
  const filter: Record<string, string> = {};
  if (filterExtension.value) filter.extension = filterExtension.value;
  if (logSeverity.value) filter.severity = logSeverity.value;
  return filter;
}

async function loadOverview() {
  try {
    overview.value = await fetchOverview();
  } catch (e: any) {
    console.error("加载总览失败:", e);
  }
}

async function loadInvocations() {
  try {
    invocations.value = await fetchInvocations(buildInvocationFilter());
  } catch (e: any) {
    ElMessage.error("加载调用记录失败: " + (e?.message || e));
  }
}

async function loadHostAPIAudits() {
  try {
    const response = await fetchHostAPIAudits({
      extension: filterExtension.value || undefined,
      method: hostMethod.value.trim() || undefined,
      result: hostResult.value || undefined,
      traceId: hostTraceId.value.trim() || undefined,
      limit: 500,
      offset: 0,
    });
    hostApiAudits.value = response.entries || [];
    hostApiAuditTotal.value = Number(response.total || 0);
  } catch (e: any) {
    hostApiAudits.value = [];
    hostApiAuditTotal.value = 0;
    ElMessage.error("加载 Host API 审计失败: " + (e?.message || e));
  }
}

async function loadEvents() {
  try {
    events.value = await fetchEvents(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载事件记录失败: " + (e?.message || e));
  }
}

async function loadHooks() {
  try {
    hooks.value = await fetchHooks(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载钩子记录失败: " + (e?.message || e));
  }
}

async function loadTasks() {
  try {
    tasks.value = await fetchTasks(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载任务记录失败: " + (e?.message || e));
  }
}

async function loadLogs() {
  try {
    logs.value = await fetchLogs(buildLogFilter());
  } catch (e: any) {
    ElMessage.error("加载日志失败: " + (e?.message || e));
  }
}

async function loadLifecycle() {
  try {
    lifecycle.value = await fetchLifecycle(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载生命周期失败: " + (e?.message || e));
  }
}

async function loadPerformance() {
  try {
    performance.value = await fetchPerformance(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载性能指标失败: " + (e?.message || e));
  }
}

async function loadPermissions() {
  try {
    permissions.value = await fetchPermissions(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载权限记录失败: " + (e?.message || e));
  }
}

async function loadScopes() {
  try {
    scopes.value = await fetchScopes();
  } catch (e: any) {
    ElMessage.error("加载作用域失败: " + (e?.message || e));
  }
}

async function loadResources() {
  try {
    resources.value = await fetchResources(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载资源记录失败: " + (e?.message || e));
  }
}

async function loadStorage() {
  try {
    storage.value = await fetchStorage(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载存储记录失败: " + (e?.message || e));
  }
}

async function loadUISessions() {
  try {
    uiSessions.value = await fetchUISessions(buildFilter());
  } catch (e: any) {
    ElMessage.error("加载 UI 会话失败: " + (e?.message || e));
  }
}

async function loadMigration() {
  try {
    migration.value = await fetchMigration();
  } catch (e: any) {
    ElMessage.error("加载迁移记录失败: " + (e?.message || e));
  }
}

async function loadCompatibility() {
  try {
    compatibility.value = await fetchCompatibility();
  } catch (e: any) {
    ElMessage.error("加载兼容性记录失败: " + (e?.message || e));
  }
}

async function loadActiveTab() {
  switch (activeTab.value) {
    case "overview":
      await loadOverview();
      break;
    case "invocations":
      await loadInvocations();
      break;
    case "hostApiAudits":
      await loadHostAPIAudits();
      break;
    case "events":
      await loadEvents();
      break;
    case "hooks":
      await loadHooks();
      break;
    case "tasks":
      await loadTasks();
      break;
    case "logs":
      await loadLogs();
      break;
    case "lifecycle":
      await loadLifecycle();
      break;
    case "performance":
      await loadPerformance();
      break;
    case "permissions":
      await loadPermissions();
      break;
    case "scopes":
      await loadScopes();
      break;
    case "resources":
      await loadResources();
      break;
    case "storage":
      await loadStorage();
      break;
    case "uiSessions":
      await loadUISessions();
      break;
    case "migration":
      await loadMigration();
      break;
    case "compatibility":
      await loadCompatibility();
      break;
  }
}

async function refreshAll() {
  loading.value = true;
  try {
    await loadOverview();
    await loadActiveTab();
  } finally {
    loading.value = false;
  }
}

function onTabChange() {
  loadActiveTab();
}

function showFields(row: LogEntry) {
  currentFields.value = row.fields || null;
  fieldsVisible.value = true;
}

async function handleExport() {
  exporting.value = true;
  try {
    const filter: Record<string, string> = {};
    if (filterExtension.value) filter.extension = filterExtension.value;
    if (logSeverity.value) filter.severity = logSeverity.value;
    const blob = await exportDiagnostics(filter);
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `dev-console-diagnostics-${new Date().toISOString().replace(/[:.]/g, "-")}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    ElMessage.success("诊断包已导出");
  } catch (e: any) {
    ElMessage.error("导出失败: " + (e?.message || e));
  } finally {
    exporting.value = false;
  }
}

onMounted(() => {
  refreshAll();
  refreshTimer = setInterval(loadOverview, 15000);
});

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<style scoped>
.audit-total { color: var(--el-text-color-secondary); font-size: 12px; white-space: nowrap; }
.host-audit-toolbar { flex-wrap: wrap; }
.developer-console {
  padding: 20px;
}

.console-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.console-header h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.overview-cards {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}

.metric-card {
  text-align: center;
}

.metric-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--el-color-primary);
  line-height: 1.4;
}

.metric-value.warn {
  color: var(--el-color-warning);
}

.metric-value.danger {
  color: var(--el-color-danger);
}

.metric-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.console-tabs {
  margin-top: 8px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
  align-items: center;
}

.phase-tag {
  margin-right: 4px;
}

@media (max-width: 1200px) {
  .overview-cards {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (max-width: 768px) {
  .overview-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
