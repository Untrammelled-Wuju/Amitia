<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="knowledge-page">
    <ExtensionPageHeader
      title="知识库"
      description="当前账户或空间内所有角色共享。知识库与角色世界书、长期记忆相互独立。"
      parent-title="记忆"
      parent-path="/memory"
      breadcrumb-label="记忆页面层级"
    >
      <template #actions>
        <div class="header-actions">
        <el-button size="small" :loading="collecting" @click="handleGarbageCollect">清理过期版本</el-button>
        <el-button size="small" :loading="reconciling" @click="handleReconcile">一致性修复</el-button>
        <el-button size="small" type="primary" :loading="uploading" @click="pickFile">上传文件</el-button>
        <input ref="fileInput" class="hidden-input" type="file" :accept="acceptTypes" @change="onFilePicked" />
        <input ref="revisionFileInput" class="hidden-input" type="file" :accept="acceptTypes" @change="onRevisionFilePicked" />
        </div>
      </template>
    </ExtensionPageHeader>

    <el-alert
      title="知识库内容按数据处理，检索结果不会被当成系统指令执行。向量服务不可用时会自动退化到 Exact + Sparse 检索。"
      type="info"
      :closable="false"
      show-icon
      class="boundary-alert"
    />

    <el-alert
      v-if="indexOutboxHealth?.status === 'blocked'"
      :title="`向量索引清理受阻：${indexOutboxHealth.exhaustedDeletes} 个删除任务已达到重试上限，请检查 Qdrant 后执行一致性修复。`"
      type="error"
      :closable="false"
      show-icon
      class="boundary-alert"
    />

    <div class="upload-strip" @dragover.prevent @drop.prevent="onDrop">
      <div>
        <strong>拖入知识文件</strong>
        <span>支持 TXT / Markdown / CSV / JSON / JSONL / DOC / DOCX / XLSX / PDF</span>
      </div>
      <el-input v-model="logicalName" size="small" placeholder="可选：知识源名称" clearable />
    </div>

    <section class="panel">
      <div class="section-title-row">
        <h3>知识源</h3>
        <el-button size="small" :loading="loadingSources" @click="refreshAll">刷新</el-button>
      </div>
      <el-table :data="sources" v-loading="loadingSources" empty-text="暂无知识源">
        <el-table-column prop="logicalName" label="名称" min-width="180">
          <template #default="{ row }">
            <div class="source-name">{{ row.logicalName || row.filename }}</div>
            <div class="source-file">{{ row.filename }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="解析进度" width="180">
          <template #default="{ row }">
            <span>{{ row.processedUnits || 0 }} / {{ row.expectedUnits || "?" }}</span>
            <span class="muted"> · 索引 {{ row.indexedChunks || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Revision" width="95">
          <template #default="{ row }">r{{ row.revision }}</template>
        </el-table-column>
        <el-table-column label="大小" width="100">
          <template #default="{ row }">{{ formatBytes(row.sizeBytes) }}</template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="160">
          <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="330" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="showSourceDetail(row)">详情</el-button>
            <el-button size="small" text type="primary" @click="pickRevision(row)">新版本</el-button>
            <el-button size="small" text :disabled="row.status !== 'ready' && row.status !== 'ready_degraded'" @click="reindex(row)">重建索引</el-button>
            <el-button size="small" text @click="showJobs(row)">任务</el-button>
            <el-button size="small" text type="danger" @click="removeSource(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-for="row in sources.filter((item) => item.lastError)" :key="`${row.id}:error`" class="source-error">
        <strong>{{ row.logicalName || row.filename }}</strong>：{{ row.lastError }}
      </div>
    </section>

    <section class="panel search-panel">
      <div class="section-title-row">
        <div>
          <h3>检索验证</h3>
          <div class="muted">直接调用与聊天自动召回相同的 Knowledge Retrieval Runtime。</div>
        </div>
      </div>
      <div class="search-row">
        <el-input v-model="query" placeholder="输入检索问题；允许返回 0 条证据" clearable @keyup.enter="runSearch" />
        <el-select v-model="retrievalLevel" class="temporal-select" aria-label="检索级别">
          <el-option label="自动级别" value="auto" />
          <el-option label="NONE" value="none" />
          <el-option label="FAST" value="fast" />
          <el-option label="DEEP" value="deep" />
          <el-option label="AGENTIC" value="agentic" />
        </el-select>
        <el-select v-model="temporalMode" class="temporal-select" aria-label="时间语义">
          <el-option label="当前有效" value="current" />
          <el-option label="历史时点" value="historical" />
          <el-option label="全部时段" value="all" />
        </el-select>
        <el-date-picker
          v-if="temporalMode === 'historical'"
          v-model="historicalAsOf"
          type="datetime"
          placeholder="选择历史时点"
          :clearable="false"
        />
        <el-button type="primary" :loading="searching" :disabled="!query.trim() || (temporalMode === 'historical' && !historicalAsOf)" @click="runSearch">检索</el-button>
      </div>
      <template v-if="searchResult">
        <div class="search-summary">
          <el-tag size="small" :type="searchResult.answerState === 'ambiguous' ? 'danger' : (searchResult.sufficient ? 'success' : 'info')">
            {{ searchResult.answerState === "ambiguous" ? "证据冲突" : (searchResult.sufficient ? "证据充分" : "证据不足 / 无证据") }}
          </el-tag>
          <el-tag v-if="searchResult.degraded" size="small" type="warning">降级检索</el-tag>
          <el-tag v-if="searchResult.ambiguous" size="small" type="danger">存在冲突证据</el-tag>
          <el-tag size="small" type="info">{{ (searchResult.retrievalLevel || "-").toUpperCase() }}</el-tag>
          <el-tag size="small" type="info">{{ searchResult.queryComplexity || "simple" }}</el-tag>
          <el-tag size="small" type="info">{{ temporalModeLabel(searchResult.temporalMode) }}</el-tag>
          <span v-if="searchResult.asOf">asOf {{ formatTime(searchResult.asOf) }}</span>
          <span>返回 {{ searchResult.evidence.length }} 条</span>
          <span v-if="searchResult.sufficiencyReason">gate={{ searchResult.sufficiencyReason }}</span>
        </div>
        <el-alert
          v-for="warning in searchResult.warnings || []"
          :key="warning"
          :title="warning"
          type="warning"
          :closable="false"
          class="search-warning"
        />
        <div v-for="(item, index) in searchResult.evidence" :key="item.id" class="evidence-card">
          <div class="evidence-head">
            <div class="evidence-title">
              <strong>K{{ index + 1 }} · {{ item.title || "未命名片段" }}</strong>
              <el-tag v-if="item.evidenceKind" size="small" type="info">{{ item.evidenceKind }}</el-tag>
            </div>
            <div class="evidence-actions">
              <span>fusion {{ formatScore(item.fusionScore) }}</span>
              <el-button size="small" text type="primary" :loading="readingEvidenceId === item.id" @click="showEvidenceRead(item)">读取原文</el-button>
            </div>
          </div>
          <div class="evidence-meta">
            source={{ item.sourceId }} · revision={{ item.sourceRevision }}
            <template v-if="item.unitId"> · unit={{ item.unitId }}</template>
            <template v-if="item.conflictGroup"> · conflict={{ item.conflictGroup }}</template>
            <template v-if="item.validFrom"> · validFrom={{ formatTime(item.validFrom) }}</template>
            <template v-if="item.validTo"> · validTo={{ formatTime(item.validTo) }}</template>
            · locator={{ compactJson(item.locator) }}
          </div>
          <div v-if="item.coverageKeys?.length" class="coverage-row">coverage={{ item.coverageKeys.join(", ") }}</div>
          <pre>{{ item.content }}</pre>
        </div>
      </template>
    </section>

    <section class="panel diagnostics-panel">
      <div class="section-title-row">
        <div>
          <h3>检索诊断</h3>
          <div class="muted">仅显示脱敏 Trace：查询哈希、候选/证据 ID、score、locator hash 与 SLO，不包含原始 query 或文档正文。</div>
        </div>
        <div class="diagnostic-actions">
          <el-button v-if="diagnosticsExpanded" size="small" :loading="loadingDiagnostics" @click="refreshDiagnostics">刷新</el-button>
          <el-button size="small" @click="toggleDiagnostics">{{ diagnosticsExpanded ? "收起诊断" : "打开开发者诊断" }}</el-button>
        </div>
      </div>
      <template v-if="diagnosticsExpanded">
      <div v-if="runtimeStatus" class="runtime-status">
        <div class="diagnostic-subtitle">运行时灰度</div>
        <div class="runtime-tags">
          <el-tag size="small" :type="runtimeStatus.retrievalPolicyEnabled ? 'success' : 'warning'">Policy {{ runtimeStatus.retrievalPolicyEnabled ? 'ACTIVE' : 'LEGACY' }}</el-tag>
          <el-tag v-if="runtimeStatus.retrievalPolicyShadow" size="small" type="info">SHADOW</el-tag>
          <el-tag size="small" :type="runtimeStatus.canonicalReaderEnabled ? 'success' : 'warning'">Reader {{ runtimeStatus.canonicalReaderEnabled ? 'ON' : 'OFF' }}</el-tag>
          <el-tag size="small" :type="runtimeStatus.agenticInvestigationEnabled ? 'success' : 'warning'">Agentic {{ runtimeStatus.agenticInvestigationEnabled ? 'ON' : 'OFF' }}</el-tag>
        </div>
        <div class="muted">readerWorkers={{ runtimeStatus.readerWorkers }} · trace={{ runtimeStatus.retrievalTraceRetentionHours }}h · benchmark={{ runtimeStatus.benchmarkRetentionHours }}h · cleanupBatch={{ runtimeStatus.diagnosticCleanupBatchSize }} · config={{ shortHash(runtimeStatus.configHash) }}</div>
        <div v-if="policyDistribution.samples" class="muted">最近 {{ policyDistribution.samples }} 条：actual={{ compactJson(policyDistribution.actual) }}<template v-if="Object.keys(policyDistribution.shadow).length"> · shadow={{ compactJson(policyDistribution.shadow) }}</template></div>
      </div>
      <div v-if="indexOutboxHealth" class="runtime-status">
        <div class="diagnostic-subtitle">向量索引任务健康度</div>
        <div class="runtime-tags">
          <el-tag size="small" :type="indexOutboxHealth.status === 'blocked' ? 'danger' : (indexOutboxHealth.status === 'degraded' ? 'warning' : 'success')">
            {{ indexOutboxHealth.status === 'blocked' ? '删除清理阻塞' : (indexOutboxHealth.status === 'degraded' ? '待恢复' : '正常') }}
          </el-tag>
          <el-tag v-if="indexOutboxHealth.exhaustedUpserts" size="small" type="danger">Upsert 重试耗尽 {{ indexOutboxHealth.exhaustedUpserts }}</el-tag>
        </div>
        <div class="muted">等待写入 {{ indexOutboxHealth.pendingUpserts }} · 等待删除 {{ indexOutboxHealth.pendingDeletes }} · 删除重试 {{ indexOutboxHealth.retryingDeletes }} · 删除耗尽 {{ indexOutboxHealth.exhaustedDeletes }} · 持有租约 {{ indexOutboxHealth.activeLeases }} · 逾期 {{ indexOutboxHealth.overdueOperations }} · 最久积压 {{ indexOutboxHealth.oldestOutstandingSeconds }} 秒</div>
        <div class="muted">统计依据：当前空间的 SQLite outbox；不代表已完成真实 Qdrant 向量清点。</div>
      </div>
      <div class="runtime-status">
        <div class="diagnostic-subtitle">Qdrant 向量抽样核验</div>
        <el-button size="small" :loading="auditingVectors" @click="handleVectorAudit">核验最多 50 条</el-button>
        <div v-if="vectorAudit" class="runtime-tags">
          <el-tag size="small" :type="vectorAudit.status === 'healthy' ? 'success' : vectorAudit.status === 'degraded' ? 'danger' : 'info'">
            {{ vectorAudit.status === 'healthy' ? '样本匹配' : vectorAudit.status === 'degraded' ? '发现不一致' : '无可检查样本' }}
          </el-tag>
          <span class="muted">已检 {{ vectorAudit.sampled }} · 缺失 {{ vectorAudit.missing }} · 元数据不符 {{ vectorAudit.mismatched }} · 涉及集合 {{ vectorAudit.collectionsChecked }}</span>
        </div>
        <div class="muted">按天轮换 Canonical 抽样窗口，不扫描整个 Qdrant；结果不代表全量一致性，不自动修复。</div>
      </div>
      <div class="runtime-status">
        <div class="diagnostic-subtitle">Qdrant 反向孤儿向量检查</div>
        <el-button size="small" :loading="auditingOrphans" @click="handleOrphanAudit">只读检查最多 50 个点</el-button>
        <div v-if="orphanVectorAudit" class="runtime-tags">
          <el-tag size="small" :type="orphanVectorAudit.status === 'suspected' ? 'warning' : 'info'">
            {{ orphanVectorAudit.status === 'suspected' ? '发现待核实异常' : orphanVectorAudit.status === 'empty' ? '无样本' : '本次样本未发现异常' }}
          </el-tag>
          <span class="muted">已检 {{ orphanVectorAudit.sampled }} · 疑似孤儿 {{ orphanVectorAudit.suspectedOrphans }} · 元数据差异 {{ orphanVectorAudit.metadataMismatches }} · 在途跳过 {{ orphanVectorAudit.inFlight }} · 集合 {{ orphanVectorAudit.collectionsChecked }}/{{ orphanVectorAudit.collectionsKnown }}</span>
        </div>
        <div class="runtime-tags">
          <el-button size="small" :loading="pagingOrphans" @click="handleOrphanPage(false)">开始分页巡检</el-button>
          <el-button v-if="orphanVectorPage?.nextCursor" size="small" :loading="pagingOrphans" @click="handleOrphanPage(true)">继续下一页</el-button>
        </div>
        <div v-if="orphanVectorPage" class="muted">
          巡检 {{ orphanVectorPage.auditId.slice(0, 8) }} · 第 {{ orphanVectorPage.page }} 页 · 已查 {{ orphanVectorPage.sampled }} 个点 · 疑似孤儿 {{ orphanVectorPage.suspectedOrphans }} · 元数据不符 {{ orphanVectorPage.metadataMismatches }} · 在途 {{ orphanVectorPage.inFlight }} · 本页扫描集合 {{ orphanVectorPage.collectionsChecked }} · {{ orphanVectorPage.finished ? '已到达末页' : (orphanVectorPage.truncated ? '达到分页安全上限' : '仍有未覆盖数据') }}
        </div>
        <div v-if="orphanPageTotals" class="muted">会话累计：{{ orphanPageTotals.pages }} 页 · {{ orphanPageTotals.sampled }} 点 · 疑似孤儿 {{ orphanPageTotals.suspectedOrphans }} · 元数据不符 {{ orphanPageTotals.metadataMismatches }} · 在途跳过 {{ orphanPageTotals.inFlight }}</div>
        <div class="runtime-tags">
          <el-button size="small" @click="fetchOrphanAuditSessions">查看巡检历史</el-button>
        </div>
        <div v-for="item in orphanAuditSessions" :key="item.auditId" class="muted">
          {{ item.auditId.slice(0, 8) }} · {{ item.status === 'finished' ? '已完成' : item.status === 'truncated' ? '达到上限' : item.status === 'expired' ? '已过期' : '进行中' }} · {{ item.pages }} 页 · {{ item.sampled }} 点 · 疑似孤儿 {{ item.suspectedOrphans }} · 元数据差异 {{ item.metadataMismatches }}
          <el-button text size="small" :loading="fetchingOrphanReceipts" @click="handleAuditReceipts(item.auditId)">核验页回执</el-button>
          <el-button text size="small" :loading="recheckingOrphans" @click="handleOrphanRecheck(item.auditId)">二次核验最近一页</el-button>
          <el-button text size="small" :loading="fetchingOrphanRechecks" @click="handleRecheckHistory(item.auditId)">复核历史</el-button>
          <el-button text size="small" :loading="reviewingRecovery" @click="handleRecoveryReviewHistory(item.auditId)">人工审查记录</el-button>
        </div>
        <div v-if="orphanRecheckHistory.length" class="muted">
          <div v-for="entry in orphanRecheckHistory" :key="entry.id">
            复核 {{ entry.id.slice(0, 8) }} · 第 {{ entry.page }} 页 · {{ entry.status }} · {{ entry.observedAt }} · 摘要 {{ entry.resultHash.slice(0, 12) }} · 仅供审计，不授权自动恢复
          </div>
        </div>
        <div v-if="orphanRecheck" class="muted">
          巡检 {{ orphanRecheck.auditId.slice(0, 8) }} · 第 {{ orphanRecheck.page }} 页二次核验：
          {{ orphanRecheck.status === 'matched' ? '再次扫描的样本摘要与异常计数相符（仍非全库一致性证明）' :
             orphanRecheck.status === 'changed' ? '二次扫描发生变化，禁止据此修复' :
             orphanRecheck.status === 'legacy_no_digest' ? '历史页缺少观测摘要，无法充分复核' :
             orphanRecheck.status === 'unanchored' ? '审计摘要链不完整，拒绝复核' :
             orphanRecheck.status === 'expired' ? '巡检游标已过期，请重新巡检' :
             orphanRecheck.status === 'collections_changed' ? '集合范围已变化，请重新巡检' : '检查点异常，拒绝复核' }}
          · 原样本 {{ orphanRecheck.previousSampled }} · 当前样本 {{ orphanRecheck.currentSampled }} · 原疑似孤儿 {{ orphanRecheck.previousSuspected }} · 当前 {{ orphanRecheck.currentSuspected }}
          <el-button v-if="orphanRecheck.status === 'matched' && orphanRecheck.currentSuspected > 0 && orphanRecheck.recheckId" text size="small" type="primary" :loading="reviewingRecovery" @click="handleRecoveryReviewRequest(orphanRecheck.auditId, orphanRecheck.recheckId)">提交人工核查申请</el-button>
        </div>
        <div v-if="recoveryReviews.length" class="muted">
          <div v-for="review in recoveryReviews" :key="review.id">
            审查 {{ review.id.slice(0, 8) }} · {{ review.displayStatus === 'history_inconsistent' ? '操作轨迹异常，禁止审批' : review.displayStatus === 'revoked' ? '已撤销' : review.displayStatus === 'expired' ? '申请已过期' : review.displayStatus === 'identity_unverified' ? '设备身份无法验证' : review.displayStatus === 'needs_recheck' ? '待核验审批条件' : review.displayStatus === 'approved_unverified' ? '曾人工批准，当前授权需重新核验' : '状态不可用' }} · 证据到期 {{ review.expiresAt }} · 不具备自动执行权限
            <el-button v-if="review.displayStatus === 'needs_recheck'" text size="small" :loading="reviewingRecovery" @click="handleRecoveryReviewTransition(review.id, review.auditId, 'approve')">检查证据后人工通过</el-button>
            <el-button v-if="review.status !== 'revoked'" text size="small" :loading="reviewingRecovery" @click="handleRecoveryReviewTransition(review.id, review.auditId, 'revoke')">撤销</el-button>
            <el-button text size="small" @click="handleRecoveryReviewEvents(review.id)">操作轨迹</el-button>
            <el-button text size="small" @click="handleRecoveryReviewPlan(review.id)">只读计划检查</el-button>
          </div>
        </div>
        <div v-if="recoveryReviewPlan" class="muted">
          恢复审查计划 {{ recoveryReviewPlan.reviewId.slice(0, 8) }} ·
          {{ recoveryReviewPlan.evidenceStatus === 'valid' ? '当前证据有效' : recoveryReviewPlan.evidenceStatus === 'expired' ? '审查已过期' : recoveryReviewPlan.evidenceStatus === 'revoked' ? '审查已撤销' : recoveryReviewPlan.evidenceStatus === 'stale' ? '证据已变化或不可用' : recoveryReviewPlan.evidenceStatus === 'identity_unverified' ? '审查设备身份无法验证' : recoveryReviewPlan.evidenceStatus === 'history_inconsistent' ? '审查操作轨迹不一致' : '无法核验证据' }} ·
          {{ recoveryReviewPlan.historyStatus === 'verified' ? '操作轨迹一致' : recoveryReviewPlan.historyStatus === 'inconsistent' ? '操作轨迹异常，禁止继续审批' : '操作轨迹不可用' }} · {{ recoveryReviewPlan.canApprove ? '当前设备具备人工审批资格（提交时再次验证）' : recoveryReviewPlan.manualReviewReady ? '当前设备不能批准此申请' : '禁止推进人工审查' }} · 无执行权限
        </div>
        <div v-if="recoveryReviewEvents.length || recoveryReviewEventHistoryStatus === 'inconsistent'" class="muted">
          {{ recoveryReviewEventHistoryStatus === 'verified' ? '事件轨迹完整' : '事件轨迹异常或不完整，不能作为审批依据' }}
          <div v-for="event in recoveryReviewEvents" :key="event.id">
            {{ event.action === 'requested' ? '提交核查申请' : event.action === 'approved' ? '人工确认' : '人工撤销' }} · {{ event.createdAt }} · 证据摘要 {{ event.receiptHash.slice(0, 12) }}
          </div>
        </div>
        <div v-if="orphanAuditReceipts" class="muted">
          回执 {{ orphanAuditReceipts.auditId.slice(0, 8) }} · {{ orphanAuditReceipts.items.length }} 页 · {{ orphanAuditReceipts.chainAnchored ? '已从首页验证摘要链' : (orphanAuditReceipts.items.some((item) => !item.verified) ? '回执校验异常或历史缺口' : '仅核验了当前片段，非全链证明') }}
          <div v-for="receipt in orphanAuditReceipts.items" :key="receipt.page">
            第 {{ receipt.page }} 页 · {{ receipt.verified ? '摘要与链路校验通过' : '记录缺失或摘要校验失败' }} · digest {{ receipt.receiptHash.slice(0, 16) }}
          </div>
          <el-button v-if="orphanAuditReceipts.hasMore" text size="small" :loading="fetchingOrphanReceipts" @click="handleAuditReceipts(orphanAuditReceipts.auditId, orphanAuditReceipts.nextPage)">查看后续回执</el-button>
        </div>
        <div class="muted">分页会话由 SQLite 持久化，续页游标与租户、集合、页码、请求限制严格匹配；30 分钟有效，历史摘要保留最多 7 天。每页最多 50 个点、4 个集合。分页非快照式，不自动修复。</div>
      </div>
      <div v-if="sloSummary" class="slo-grid">
        <div v-for="item in sloSummary.levels" :key="item.level" class="slo-card">
          <div class="slo-head">
            <strong>{{ item.level.toUpperCase() }}</strong>
            <el-tag size="small" :type="!item.enoughSamples ? 'info' : (item.targetMet ? 'success' : 'warning')">
              {{ !item.enoughSamples ? '样本不足' : (item.targetMet ? 'SLO 达标' : 'SLO 超标') }}
            </el-tag>
          </div>
          <div>P50 {{ item.p50Ms }}ms · P95 {{ item.p95Ms }}ms · target {{ item.targetMs }}ms</div>
          <div class="muted">samples={{ item.samples }} · errors={{ item.errorCount }} · exceeded={{ item.exceededCount }}</div>
        </div>
      </div>
      <div class="benchmark-history" v-if="benchmarkRuns.length">
        <div class="diagnostic-subtitle">Benchmark 趋势</div>
        <el-table :data="benchmarkRuns" size="small" empty-text="暂无 Benchmark 历史">
          <el-table-column label="时间" width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
          <el-table-column label="结果" width="90"><template #default="{ row }"><el-tag size="small" :type="row.passed ? 'success' : 'danger'">{{ row.passed ? 'PASS' : 'FAIL' }}</el-tag></template></el-table-column>
          <el-table-column label="Recall@K" width="105"><template #default="{ row }">{{ formatScore(row.recallAtK) }}</template></el-table-column>
          <el-table-column label="MRR" width="90"><template #default="{ row }">{{ formatScore(row.mrr) }}</template></el-table-column>
          <el-table-column label="NDCG" width="90"><template #default="{ row }">{{ formatScore(row.ndcgAtK) }}</template></el-table-column>
          <el-table-column label="NoAnswer" width="105"><template #default="{ row }">{{ formatScore(row.noAnswerAccuracy) }}</template></el-table-column>
          <el-table-column label="P50/P95" width="135"><template #default="{ row }">{{ row.p50LatencyMs }}/{{ row.p95LatencyMs }}ms</template></el-table-column>
          <el-table-column label="错误率" width="90"><template #default="{ row }">{{ formatScore(row.errorRate) }}</template></el-table-column>
          <el-table-column label="场景" min-width="220"><template #default="{ row }">{{ compactJson(row.scenarioCounts) }}</template></el-table-column>
          <el-table-column label="Gate" min-width="190"><template #default="{ row }">{{ row.violations?.length ? row.violations.join('；') : '-' }}</template></el-table-column>
        </el-table>
      </div>
      <el-table :data="retrievalTraces" v-loading="loadingDiagnostics" empty-text="暂无检索 Trace">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="trace-detail">
              <div><strong>Query Plan</strong> · level={{ row.queryPlan?.level || '-' }} · complexity={{ row.queryPlan?.complexity || '-' }} · timeout={{ row.queryPlan?.timeoutMs || 0 }}ms · candidates={{ row.queryPlan?.candidateLimit || 0 }}</div>
              <div><strong>Policy</strong> · mode={{ row.policyMode || '-' }} · reason={{ row.routeReason || '-' }}<template v-if="row.shadowLevel || row.shadowReason"> · shadow={{ row.shadowLevel || '-' }} / {{ row.shadowReason || '-' }}</template></div>
              <div class="muted">queryHash={{ shortHash(row.queryHash) }} · original={{ shortHash(row.queryPlan?.originalHash) }} · effective={{ shortHash(row.queryPlan?.effectiveHash) }} · config={{ shortHash(row.configHash) }}</div>
              <div v-if="row.warnings?.length" class="trace-warnings">{{ row.warnings.join("；") }}</div>
              <div class="trace-columns">
                <div>
                  <strong>Selected Evidence</strong>
                  <div v-for="item in row.evidence || []" :key="`${row.id}:ev:${item.rank}`" class="trace-item">
                    #{{ item.rank }} {{ item.sourceId }}@r{{ item.sourceRevision }} · fusion={{ formatScore(item.fusionScore) }} · rerank={{ formatScore(item.rerankScore) }} · locator={{ shortHash(item.locatorHash) }}
                  </div>
                </div>
                <div>
                  <strong>Top Candidates</strong>
                  <div v-for="item in (row.candidates || []).slice(0, 12)" :key="`${row.id}:ca:${item.rank}`" class="trace-item">
                    #{{ item.rank }} {{ item.sourceId }}@r{{ item.sourceRevision }} · dense={{ formatScore(item.denseScore) }} · sparse={{ formatScore(item.sparseScore) }} · fusion={{ formatScore(item.fusionScore) }}
                  </div>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="级别" width="95"><template #default="{ row }">{{ (row.queryPlan?.level || '-').toUpperCase() }}</template></el-table-column>
        <el-table-column label="策略" width="125"><template #default="{ row }">{{ row.policyMode || '-' }}<span v-if="row.shadowLevel"> · {{ row.shadowLevel.toUpperCase() }}</span></template></el-table-column>
        <el-table-column label="耗时" width="100"><template #default="{ row }">{{ row.durationMs }}ms</template></el-table-column>
        <el-table-column label="候选/选中" width="115"><template #default="{ row }">{{ row.candidateCount }}/{{ row.selectedCount }}</template></el-table-column>
        <el-table-column label="状态" min-width="180">
          <template #default="{ row }">
            <el-tag size="small" :type="row.errorCode ? 'danger' : (row.sloExceeded ? 'warning' : 'success')">{{ row.errorCode || (row.sufficient ? 'sufficient' : 'insufficient') }}</el-tag>
            <el-tag v-if="row.degraded" size="small" type="warning" class="trace-tag">degraded</el-tag>
            <el-tag v-if="row.ambiguous" size="small" type="danger" class="trace-tag">ambiguous</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="SLO" width="130"><template #default="{ row }">{{ row.sloTargetMs ? `${row.sloTargetMs}ms` : '-' }}<span v-if="row.sloExceeded"> · exceeded</span></template></el-table-column>
      </el-table>
      </template>
    </section>

    <el-dialog v-model="readVisible" :title="readDialogTitle" width="900px" destroy-on-close>
      <div v-loading="readingEvidenceId !== ''">
        <template v-if="readResult">
          <div class="detail-summary">
            <span>source={{ readResult.sourceId }}</span>
            <span>revision={{ readResult.sourceRevision }}</span>
            <span>anchor={{ readResult.anchorUnitId || '-' }}</span>
            <span>{{ readResult.totalRunes }} runes</span>
            <el-tag v-if="readResult.truncated" size="small" type="warning">已按预算截断</el-tag>
          </div>
          <div v-for="unit in readResult.units" :key="unit.id" class="read-unit" :class="{ 'read-unit-anchor': unit.id === readResult.anchorUnitId }">
            <div class="read-unit-head">
              <strong>{{ unit.title || unit.unitType || '原文单元' }}</strong>
              <span>#{{ unit.ordinal }} · {{ compactJson(unit.locator) }}</span>
            </div>
            <pre>{{ unit.content }}</pre>
          </div>
        </template>
      </div>
    </el-dialog>

    <el-dialog v-model="detailVisible" :title="detailDialogTitle" width="860px" destroy-on-close>
      <div v-loading="loadingSourceDetail">
        <template v-if="sourceDetail">
          <div class="detail-summary">
            <span>活动版本 r{{ sourceDetail.source.revision }}</span>
            <span>Units {{ sourceDetail.unitCount }}</span>
            <span>Chunks {{ sourceDetail.chunkCount }}</span>
            <el-tag size="small" :type="sourceDetail.indexHealth === 'healthy' ? 'success' : sourceDetail.indexHealth === 'degraded' ? 'warning' : 'danger'">
              {{ sourceDetail.indexHealth }}
            </el-tag>
          </div>
          <el-table :data="sourceDetail.revisions" empty-text="暂无版本记录">
            <el-table-column label="版本" width="90">
              <template #default="{ row }">
                r{{ row.revision }} <el-tag v-if="row.revision === sourceDetail?.source.revision" size="small" type="success">活动</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="filename" label="文件" min-width="180" />
            <el-table-column label="状态" width="125">
              <template #default="{ row }"><el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="解析/索引" width="170">
              <template #default="{ row }">{{ row.processedUnits }}/{{ row.expectedUnits || '?' }} · {{ row.indexedChunks }}</template>
            </el-table-column>
            <el-table-column label="大小" width="100">
              <template #default="{ row }">{{ formatBytes(row.sizeBytes) }}</template>
            </el-table-column>
            <el-table-column label="保留/激活" min-width="185">
              <template #default="{ row }">
                <div>{{ row.revision === sourceDetail?.source.revision ? `激活 ${formatTime(row.activatedAt || '')}` : retentionLabel(row) }}</div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="165" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="!row.purgedAt"
                  size="small"
                  text
                  @click="loadSourceOutline(sourceDetail.source.id, row.revision, true)"
                >结构</el-button>
                <el-button
                  v-if="canRollback(row)"
                  size="small"
                  text
                  type="primary"
                  @click="rollback(row)"
                >回滚</el-button>
                <el-button
                  v-if="row.revision !== sourceDetail?.source.revision && !row.purgedAt"
                  size="small"
                  text
                  type="danger"
                  :disabled="!canPurge(row)"
                  @click="purge(row)"
                >清理</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="outline-section">
            <div class="section-title-row outline-title-row">
              <div>
                <strong>Canonical Outline · r{{ outlineRevision || sourceDetail.source.revision }}</strong>
                <div class="muted">仅显示 unit 结构、locator 与 checksum，不返回正文；点击读取时才走受限 Canonical Reader。</div>
              </div>
            </div>
            <el-table :data="sourceOutline" v-loading="loadingOutline" max-height="340" empty-text="该版本暂无 canonical unit">
              <el-table-column prop="ordinal" label="#" width="70" />
              <el-table-column prop="unitType" label="类型" width="110" />
              <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
              <el-table-column label="Locator" min-width="260">
                <template #default="{ row }"><span class="outline-locator">{{ compactJson(row.locator) }}</span></template>
              </el-table-column>
              <el-table-column label="Checksum" width="145"><template #default="{ row }">{{ shortHash(row.checksum) }}</template></el-table-column>
              <el-table-column label="操作" width="90" fixed="right">
                <template #default="{ row }"><el-button size="small" text type="primary" :loading="readingEvidenceId === row.id" @click="showOutlineRead(row)">读取</el-button></template>
              </el-table-column>
            </el-table>
            <div v-if="outlineNextAfterOrdinal !== undefined" class="outline-more">
              <el-button size="small" :loading="loadingOutline" @click="loadSourceOutline(sourceDetail.source.id, outlineRevision || sourceDetail.source.revision, false)">加载更多</el-button>
            </div>
          </div>
          <div v-if="sourceDetail.latestJob" class="latest-job">
            最近任务：{{ sourceDetail.latestJob.kind || 'ingest' }} · r{{ sourceDetail.latestJob.sourceRevision }} · {{ sourceDetail.latestJob.status }} / {{ sourceDetail.latestJob.stage }}
          </div>
        </template>
      </div>
    </el-dialog>

    <el-dialog v-model="jobsVisible" :title="jobDialogTitle" width="760px" destroy-on-close>
      <el-table :data="jobs" empty-text="暂无任务">
        <el-table-column prop="kind" label="类型" width="90" />
        <el-table-column prop="stage" label="阶段" width="110" />
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column label="进度" min-width="150">
          <template #default="{ row }">{{ row.processedUnits }} / {{ row.expectedUnits || "?" }} · 索引 {{ row.indexedChunks }}</template>
        </el-table-column>
        <el-table-column prop="retryCount" label="重试" width="80" />
        <el-table-column label="错误" min-width="180">
          <template #default="{ row }"><span class="job-error">{{ row.lastError || "-" }}</span></template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button v-if="row.status === 'failed' || row.status === 'cancelled'" size="small" text type="primary" @click="retry(row)">重试</el-button>
            <el-button v-if="row.status === 'queued' || row.status === 'running'" size="small" text type="danger" @click="cancel(row)">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import {
  useKnowledge,
  type KnowledgeEvidence,
  type KnowledgeJob,
  type KnowledgeReadResponse,
  type KnowledgeSource,
  type KnowledgeSourceRevision,
  type KnowledgeUnitOutline,
} from "@/composables/useKnowledge";

const {
  sources,
  jobs,
  searchResult,
  loadingSources,
  searching,
  uploading,
  reconciling,
  collecting,
  sourceDetail,
  loadingSourceDetail,
  runtimeStatus,
  indexOutboxHealth,
  vectorAudit,
  auditingVectors,
  orphanVectorAudit,
  auditingOrphans,
  orphanVectorPage,
  orphanPageTotals,
  orphanAuditSessions,
  orphanAuditReceipts,
  fetchingOrphanReceipts,
  orphanRecheck,
  recheckingOrphans,
  orphanRecheckHistory,
  recoveryReviews,
  recoveryReviewEvents,
  recoveryReviewEventHistoryStatus,
  recoveryReviewPlan,
  reviewingRecovery,
  fetchingOrphanRechecks,
  pagingOrphans,
  sloSummary,
  retrievalTraces,
  benchmarkRuns,
  loadingDiagnostics,
  fetchSources,
  fetchSourceDetail,
  fetchSourceOutline,
  uploadSource,
  uploadRevision,
  reindexSource,
  rollbackRevision,
  purgeRevision,
  deleteSource,
  fetchJobs,
  retryJob,
  cancelJob,
  search,
  read,
  fetchDiagnostics,
  fetchIndexOutboxHealth,
  auditVectorSample,
  auditOrphanVectors,
  auditOrphanPage,
  fetchOrphanAuditSessions,
  fetchOrphanAuditReceipts,
  recheckOrphanLatest,
  fetchOrphanRecheckHistory,
  fetchRecoveryReviews,
  fetchRecoveryReviewEvents,
  fetchRecoveryReviewPlan,
  requestRecoveryReview,
  transitionRecoveryReview,
  reconcile,
  garbageCollect,
} = useKnowledge();

const fileInput = ref<HTMLInputElement | null>(null);
const revisionFileInput = ref<HTMLInputElement | null>(null);
const revisionTarget = ref<KnowledgeSource | null>(null);
const logicalName = ref("");
const query = ref("");
const temporalMode = ref<"current" | "historical" | "all">("current");
const retrievalLevel = ref<"auto" | "none" | "fast" | "deep" | "agentic">("auto");
const historicalAsOf = ref<Date | null>(null);
const jobsVisible = ref(false);
const detailVisible = ref(false);
const currentJobSource = ref<KnowledgeSource | null>(null);
const currentDetailSource = ref<KnowledgeSource | null>(null);
const readVisible = ref(false);
const readResult = ref<KnowledgeReadResponse | null>(null);
const readEvidence = ref<KnowledgeEvidence | null>(null);
const readingEvidenceId = ref("");
const readTitle = ref("");
const sourceOutline = ref<KnowledgeUnitOutline[]>([]);
const outlineRevision = ref(0);
const outlineNextAfterOrdinal = ref<number | undefined>(undefined);
const loadingOutline = ref(false);
const diagnosticsExpanded = ref(false);
const acceptTypes = ".txt,.md,.markdown,.csv,.json,.jsonl,.ndjson,.doc,.docx,.xlsx,.pdf";
const jobDialogTitle = computed(() => currentJobSource.value ? `知识库任务 · ${currentJobSource.value.logicalName || currentJobSource.value.filename}` : "知识库任务");
const detailDialogTitle = computed(() => currentDetailSource.value ? `知识源详情 · ${currentDetailSource.value.logicalName || currentDetailSource.value.filename}` : "知识源详情");
const readDialogTitle = computed(() => readTitle.value || (readEvidence.value ? `原文读取 · ${readEvidence.value.title || readEvidence.value.sourceId}` : "原文读取"));
const policyDistribution = computed(() => {
  const actual: Record<string, number> = {};
  const shadow: Record<string, number> = {};
  for (const row of retrievalTraces.value) {
    const actualLevel = String(row.queryPlan?.level || "none").toLowerCase();
    actual[actualLevel] = (actual[actualLevel] || 0) + 1;
    const shadowLevel = String(row.shadowLevel || "").toLowerCase();
    if (shadowLevel) shadow[shadowLevel] = (shadow[shadowLevel] || 0) + 1;
  }
  return { samples: retrievalTraces.value.length, actual, shadow };
});

function pickFile() { fileInput.value?.click(); }
async function onFilePicked(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) await doUpload(file);
}
async function onDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0];
  if (file) await doUpload(file);
}
async function doUpload(file: File) {
  try {
    await uploadSource(file, logicalName.value);
    logicalName.value = "";
    ElMessage.success("知识源已进入解析队列");
  } catch (error: any) {
    ElMessage.error(error?.message || "上传失败");
  }
}
function pickRevision(row: KnowledgeSource) {
  revisionTarget.value = row;
  revisionFileInput.value?.click();
}
async function onRevisionFilePicked(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  const target = revisionTarget.value;
  revisionTarget.value = null;
  if (!file || !target) return;
  try {
    await uploadRevision(target.id, file);
    ElMessage.success("新版本已进入解析队列，旧活动版本会保持可用直到切换完成");
  } catch (error: any) {
    ElMessage.error(error?.message || "上传新版本失败");
  }
}
async function reindex(row: KnowledgeSource) {
  try {
    await ElMessageBox.confirm(`确定重建“${row.logicalName || row.filename}”当前活动版本 r${row.revision} 的向量索引吗？`, "重建索引", { type: "warning" });
    await reindexSource(row.id);
    ElMessage.success("重建索引任务已入队");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error?.message || "重建索引失败");
  }
}
async function showSourceDetail(row: KnowledgeSource) {
  currentDetailSource.value = row;
  detailVisible.value = true;
  try {
    await fetchSourceDetail(row.id);
    await loadSourceOutline(row.id, row.revision, true);
  } catch (error: any) {
    ElMessage.error(error?.message || "加载知识源详情失败");
  }
}
async function rollback(row: KnowledgeSourceRevision) {
  const source = sourceDetail.value?.source;
  if (!source || !canRollback(row)) return;
  try {
    await ElMessageBox.confirm(
      `确定将“${source.logicalName || source.filename}”回滚到 r${row.revision} 吗？回滚目标会先重建/校验索引，切换完成前 r${source.revision} 仍继续提供检索。`,
      "回滚知识源版本",
      { type: "warning" },
    );
    await rollbackRevision(source.id, row.revision);
    ElMessage.success(`回滚 r${row.revision} 任务已入队`);
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error?.message || "回滚失败");
  }
}
async function purge(row: KnowledgeSourceRevision) {
  const source = sourceDetail.value?.source;
  if (!source || !canPurge(row)) return;
  try {
    await ElMessageBox.confirm(
      `确定永久清理 r${row.revision} 的解析数据、文件与二级索引吗？该操作完成后此版本不能再回滚。`,
      "清理历史版本",
      { type: "warning", confirmButtonText: "永久清理" },
    );
    await purgeRevision(source.id, row.revision);
    ElMessage.success(`r${row.revision} 已进入清理流程`);
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error?.message || "清理历史版本失败");
  }
}

async function refreshAll() { await Promise.all([fetchSources(), fetchJobs(), fetchIndexOutboxHealth().catch(() => null)]); }
async function handleVectorAudit() {
  try {
    await auditVectorSample(50);
  } catch (error: any) {
    ElMessage.error(error?.message || "Qdrant 向量核验失败");
  }
}

async function handleOrphanAudit() {
  try {
    await auditOrphanVectors(50);
  } catch (error: any) {
    ElMessage.error(error?.message || "Qdrant 孤儿向量检查失败");
  }
}

async function handleOrphanPage(next = false) {
  try {
    await auditOrphanPage(next, 25);
  } catch (error: any) {
    ElMessage.error(error?.message || "Qdrant 分页巡检失败，请重新开始");
  }
}

async function handleAuditReceipts(auditId: string, after = 0) {
  try {
    await fetchOrphanAuditReceipts(auditId, after);
  } catch (error: any) {
    ElMessage.error(error?.message || "巡检页回执读取失败");
  }
}

async function handleRecheckHistory(auditId: string) {
  try {
    await fetchOrphanRecheckHistory(auditId);
  } catch (error: any) {
    ElMessage.error(error?.message || "复核历史读取失败");
  }
}

async function handleRecoveryReviewHistory(auditId: string) {
  try {
    await fetchRecoveryReviews(auditId);
  } catch (error: any) {
    ElMessage.error(error?.message || "人工审查记录读取失败或管理员权限不足");
  }
}

async function handleRecoveryReviewEvents(id: string) {
  try {
    await fetchRecoveryReviewEvents(id);
  } catch (error: any) {
    ElMessage.error(error?.message || "操作轨迹读取失败");
  }
}

async function handleRecoveryReviewPlan(id: string) {
  try {
    await fetchRecoveryReviewPlan(id);
  } catch (error: any) {
    ElMessage.error(error?.message || "只读计划检查失败或管理员权限不足");
  }
}

async function handleRecoveryReviewRequest(auditId: string, recheckId: string) {
  try {
    await ElMessageBox.confirm("仅生成可撤销的人工核查申请，不执行向量删除或重建。是否继续？", "人工核查申请", { type: "warning" });
    await requestRecoveryReview(auditId, recheckId);
    ElMessage.success("人工核查申请已登记，不授予自动执行权限");
  } catch (error: any) {
    if (error !== "cancel") ElMessage.error(error?.message || "提交失败，请核实证据时效与权限");
  }
}

async function handleRecoveryReviewTransition(id: string, auditId: string, action: "approve" | "revoke") {
  try {
    await ElMessageBox.confirm(action === "approve" ? "仅记录人工审查通过，不执行任何向量操作。" : "撤销该人工审查记录？", "恢复审查", { type: "warning" });
    await transitionRecoveryReview(id, auditId, action);
    ElMessage.success(action === "approve" ? "人工审查已记录（不可执行）" : "审查已撤销");
  } catch (error: any) {
    if (error !== "cancel") ElMessage.error(error?.message || "状态更新失败，证据可能已失效");
  }
}

async function handleOrphanRecheck(auditId: string) {
  try {
    await recheckOrphanLatest(auditId);
  } catch (error: any) {
    ElMessage.error(error?.message || "二次核验失败，请检查 Qdrant 状态");
  }
}

async function handleReconcile() {
  try {
    await reconcile();
    ElMessage.success("索引任务对账已执行；如需核对外部向量，请运行抽样核验");
  } catch (error: any) {
    ElMessage.error(error?.message || "一致性检查失败");
  } finally {
    await fetchIndexOutboxHealth().catch(() => null);
  }
}
async function handleGarbageCollect() {
  try {
    const result = await garbageCollect();
    const purged = Number(result?.purged || 0);
    if (detailVisible.value && currentDetailSource.value) {
      await fetchSourceDetail(currentDetailSource.value.id);
    }
    ElMessage.success(purged > 0 ? `已清理 ${purged} 个过期版本` : "没有达到清理条件的历史版本");
  } catch (error: any) {
    ElMessage.error(error?.message || "版本清理失败");
  }
}
async function removeSource(row: KnowledgeSource) {
  try {
    await ElMessageBox.confirm(`确定删除知识源“${row.logicalName || row.filename}”吗？删除后不会再进入检索。`, "删除知识源", { type: "warning" });
    await deleteSource(row.id);
    ElMessage.success("已删除");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    if (error?.message) ElMessage.error(error.message);
  }
}
async function showJobs(row: KnowledgeSource) {
  currentJobSource.value = row;
  await fetchJobs(row.id);
  jobsVisible.value = true;
}
async function retry(row: KnowledgeJob) {
  try {
    await retryJob(row.id);
    if (currentJobSource.value) await fetchJobs(currentJobSource.value.id);
    ElMessage.success("任务已重新入队");
  } catch (error: any) {
    ElMessage.error(error?.message || "重试失败");
  }
}
async function cancel(row: KnowledgeJob) {
  try {
    await cancelJob(row.id);
    if (currentJobSource.value) await fetchJobs(currentJobSource.value.id);
    ElMessage.success("任务已取消");
  } catch (error: any) {
    ElMessage.error(error?.message || "取消失败");
  }
}
async function runSearch() {
  if (!query.value.trim()) return;
  if (temporalMode.value === "historical" && !historicalAsOf.value) {
    ElMessage.warning("请选择历史查询时点");
    return;
  }
  try {
    await search(query.value, {
      retrievalLevel: retrievalLevel.value,
      temporalMode: temporalMode.value,
      asOf: temporalMode.value === "historical" ? historicalAsOf.value?.toISOString() : undefined,
    });
  } catch (error: any) {
    ElMessage.error(error?.message || "检索失败");
  }
}
async function executeCanonicalRead(title: string, request: Parameters<typeof read>[0], loadingID: string) {
  readingEvidenceId.value = loadingID;
  readTitle.value = title;
  readVisible.value = true;
  readResult.value = null;
  try {
    readResult.value = await read(request);
  } catch (error: any) {
    readVisible.value = false;
    ElMessage.error(error?.message || "读取知识原文失败");
  } finally {
    readingEvidenceId.value = "";
  }
}

async function showEvidenceRead(item: KnowledgeEvidence) {
  if (!item?.id) return;
  readEvidence.value = item;
  await executeCanonicalRead(`原文读取 · ${item.title || item.sourceId}`, {
    evidenceId: item.id,
    unitId: item.unitId,
    sourceId: item.sourceId,
    sourceRevision: item.sourceRevision,
    beforeUnits: 2,
    afterUnits: 2,
    maxUnits: 8,
    maxRunes: 18000,
  }, item.id);
}

async function showOutlineRead(item: KnowledgeUnitOutline) {
  readEvidence.value = null;
  await executeCanonicalRead(`原文读取 · ${item.title || item.unitType || item.id}`, {
    unitId: item.id,
    sourceId: item.sourceId,
    sourceRevision: item.sourceRevision,
    beforeUnits: 2,
    afterUnits: 2,
    maxUnits: 8,
    maxRunes: 18000,
  }, item.id);
}

async function loadSourceOutline(sourceId: string, revision: number, reset = false) {
  if (!sourceId || revision <= 0 || loadingOutline.value) return;
  if (reset) {
    sourceOutline.value = [];
    outlineNextAfterOrdinal.value = undefined;
    outlineRevision.value = revision;
  }
  loadingOutline.value = true;
  try {
    const afterOrdinal = reset ? undefined : outlineNextAfterOrdinal.value;
    const result = await fetchSourceOutline(sourceId, { revision, afterOrdinal, limit: 100 });
    outlineRevision.value = result.sourceRevision;
    sourceOutline.value = reset ? (result.items || []) : [...sourceOutline.value, ...(result.items || [])];
    outlineNextAfterOrdinal.value = result.nextAfterOrdinal;
  } catch (error: any) {
    ElMessage.error(error?.message || "加载 canonical outline 失败");
  } finally {
    loadingOutline.value = false;
  }
}

async function toggleDiagnostics() {
  diagnosticsExpanded.value = !diagnosticsExpanded.value;
  if (diagnosticsExpanded.value && retrievalTraces.value.length === 0 && !sloSummary.value) {
    await refreshDiagnostics();
  }
}

async function refreshDiagnostics() {
  try {
    await fetchDiagnostics(30);
  } catch (error: any) {
    ElMessage.error(error?.message || "加载检索诊断失败");
  }
}

function shortHash(value?: string) {
  const text = String(value || "");
  if (!text) return "-";
  return text.length <= 16 ? text : `${text.slice(0, 8)}…${text.slice(-6)}`;
}

function statusLabel(status: string) {
  return ({
    pending: "等待解析", processing: "处理中", ready: "就绪", ready_degraded: "降级就绪",
    failed: "失败", cancelled: "已取消", deleted: "已删除", purged: "已清理",
  } as Record<string, string>)[status] || status;
}
function statusTag(status: string): "success" | "warning" | "danger" | "info" | "primary" {
  if (status === "ready") return "success";
  if (status === "ready_degraded") return "warning";
  if (status === "failed" || status === "purged") return "danger";
  if (status === "processing") return "primary";
  return "info";
}
function canRollback(row: KnowledgeSourceRevision) {
  return Boolean(row.canRollback);
}
function canPurge(row: KnowledgeSourceRevision) {
  return Boolean(row.canPurge);
}
function retentionLabel(row: KnowledgeSourceRevision) {
  if (row.purgedAt || row.status === "purged") return `已清理 ${formatTime(row.purgedAt || row.updatedAt)}`;
  if (row.retainUntil) {
    const expired = new Date(row.retainUntil).getTime() <= Date.now();
    return `${expired ? "可清理" : "保留至"} ${formatTime(row.retainUntil)}`;
  }
  return "保留中";
}

function temporalModeLabel(mode: string) {
  return ({ current: "当前有效", historical: "历史时点", all: "全部时段" } as Record<string, string>)[mode] || mode || "当前有效";
}

function formatBytes(value: number) {
  const n = Number(value || 0);
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`;
}
function formatTime(value: string) { return value ? new Date(value).toLocaleString() : "-"; }
function formatScore(value: number) { return Number(value || 0).toFixed(4); }
function compactJson(value: unknown) {
  try { return JSON.stringify(value ?? {}); } catch { return "{}"; }
}

let pollTimer: number | undefined;
let indexHealthTimer: number | undefined;
onMounted(async () => {
  await refreshAll();
  indexHealthTimer = window.setInterval(() => { void fetchIndexOutboxHealth().catch(() => null); }, 30000);
  pollTimer = window.setInterval(() => {
    if (jobs.value.some((job) => job.status === "queued" || job.status === "running")) {
      void refreshAll();
      if (detailVisible.value && currentDetailSource.value) {
        void fetchSourceDetail(currentDetailSource.value.id);
      }
    }
  }, 5000);
});
onUnmounted(() => {
  if (pollTimer !== undefined) window.clearInterval(pollTimer);
  if (indexHealthTimer !== undefined) window.clearInterval(indexHealthTimer);
});
</script>

<style scoped>
.knowledge-page { padding: 0; max-width: 1440px; margin: 0 auto; }
.section-title-row, .search-row, .evidence-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.temporal-select { width: 128px; flex: 0 0 128px; }
.panel h3 { margin: 0; }
.muted, .source-file, .evidence-meta { color: var(--el-text-color-secondary); font-size: 12px; }
.header-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.boundary-alert { margin: 14px 0; }
.upload-strip { display: grid; grid-template-columns: minmax(260px, 1fr) minmax(220px, 360px); gap: 18px; align-items: center; border: 1px dashed var(--el-border-color); border-radius: 10px; padding: 16px; margin-bottom: 16px; background: var(--el-fill-color-lighter); }
.upload-strip div { display: flex; flex-direction: column; gap: 4px; }
.panel { border: 1px solid var(--el-border-color-lighter); border-radius: 10px; padding: 16px; margin-bottom: 16px; background: var(--el-bg-color); }
.section-title-row { margin-bottom: 12px; }
.source-name { font-weight: 600; }
.source-error, .search-warning { margin-top: 8px; }
.source-error { color: var(--el-color-warning-dark-2); font-size: 12px; }
.search-row { justify-content: flex-start; }
.search-row .el-input { max-width: 900px; }
.search-summary { display: flex; gap: 8px; align-items: center; margin: 14px 0; color: var(--el-text-color-secondary); font-size: 13px; }
.evidence-card { margin-top: 10px; border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 12px; }
.evidence-head span { color: var(--el-text-color-secondary); font-size: 12px; }
.evidence-meta { margin: 6px 0; word-break: break-all; }
.evidence-card pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-family: inherit; line-height: 1.6; }
.job-error { color: var(--el-color-danger); font-size: 12px; }
.detail-summary { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; color: var(--el-text-color-secondary); font-size: 13px; }
.outline-section { margin-top: 18px; padding-top: 14px; border-top: 1px solid var(--el-border-color-lighter); }
.outline-title-row { align-items: flex-start; }
.outline-locator { font-size: 12px; color: var(--el-text-color-secondary); word-break: break-all; }
.outline-more { display: flex; justify-content: center; margin-top: 10px; }
.latest-job { margin-top: 12px; color: var(--el-text-color-secondary); font-size: 12px; }
.hidden-input { display: none; }
.evidence-title, .evidence-actions, .slo-head, .read-unit-head { display: flex; align-items: center; gap: 8px; }
.evidence-actions { justify-content: flex-end; }
.coverage-row { margin: 4px 0 8px; color: var(--el-text-color-secondary); font-size: 12px; word-break: break-all; }
.diagnostic-actions { display: flex; gap: 8px; }
.runtime-status { margin-bottom: 14px; }
.runtime-tags { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.diagnostic-subtitle { margin: 14px 0 8px; font-size: 13px; font-weight: 600; }
.benchmark-history { margin-bottom: 14px; }
.slo-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; margin-bottom: 12px; }
.slo-card { border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 10px 12px; font-size: 13px; line-height: 1.7; }
.slo-head { justify-content: space-between; }
.trace-detail { padding: 10px 18px 16px; font-size: 12px; line-height: 1.7; }
.trace-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; margin-top: 10px; }
.trace-item { color: var(--el-text-color-secondary); word-break: break-all; }
.trace-warnings { margin-top: 6px; color: var(--el-color-warning-dark-2); }
.trace-tag { margin-left: 4px; }
.read-unit { border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 10px 12px; margin-top: 10px; }
.read-unit-anchor { border-color: var(--el-color-primary-light-5); background: var(--el-color-primary-light-9); }
.read-unit-head { justify-content: space-between; color: var(--el-text-color-secondary); font-size: 12px; }
.read-unit-head strong { color: var(--el-text-color-primary); }
.read-unit pre { white-space: pre-wrap; word-break: break-word; font-family: inherit; line-height: 1.65; }
@media (max-width: 760px) {
  .knowledge-page { padding: 0; }
  .upload-strip { grid-template-columns: 1fr; }
  .trace-columns { grid-template-columns: 1fr; }
}
</style>
