<template>
  <div class="agent-os">
    <header class="agent-os__header">
      <div>
        <p class="agent-os__eyebrow">AMITIA AGENT OS</p>
        <h1>智能体工作台</h1>
        <p>创建、运行、协作和治理拥有独立电脑与授权的 Agent。</p>
      </div>
      <div class="agent-os__header-actions">
        <el-button v-if="platformSessionState" @click="logoutMember">{{ platformSessionState.user.displayName || platformSessionState.user.email || "成员" }}</el-button>
        <el-button v-else @click="openMemberDialog">成员登录</el-button>
        <el-button @click="agentImportInput?.click()">导入 Agent 包</el-button>
        <input ref="agentImportInput" class="sr-only" type="file" accept=".json,.zip,.amitia-agent" @change="importSelectedAgent" />
        <el-button :loading="loading" @click="refreshAll">刷新</el-button>
        <el-button type="primary" @click="openAgentDialog">新建 Agent</el-button>
      </div>
    </header>

    <nav class="agent-os__tabs">
      <button
        v-for="item in tabs"
        :key="item.id"
        :class="{ active: activeTab === item.id }"
        @click="activeTab = item.id"
      >
        {{ item.label }}
      </button>
    </nav>

    <main class="agent-os__content" v-loading="loading">
      <section v-if="activeTab === 'overview'" class="overview">
        <div class="metrics">
          <article>
            <span>Agent</span>
            <strong>{{ agents.length }}</strong>
            <small>可用身份与工作单元</small>
          </article>
          <article>
            <span>运行中</span>
            <strong>{{ overview.summary.running }}</strong>
            <small>正在执行的动作</small>
          </article>
          <article>
            <span>等待处理</span>
            <strong>{{ overview.pendingApprovals.length + overview.reconciliations.length }}</strong>
            <small>审批与副作用对账</small>
          </article>
          <article>
            <span>今日成本</span>
            <strong>{{ money(overview.summary.costMicroUsd) }}</strong>
            <small>按已报告用量计算</small>
          </article>
        </div>
        <div class="overview-grid">
          <article class="panel">
            <div class="panel__title">
              <div>
                <h2>活跃 Agent</h2>
                <p>每个 Agent 拥有独立版本、电脑、记忆和授权。</p>
              </div>
              <el-button text @click="activeTab = 'agents'">全部 Agent</el-button>
            </div>
            <div class="agent-cards">
              <button
                v-for="agent in agents.slice(0, 6)"
                :key="agent.id"
                class="agent-card"
                @click="selectAgent(agent.id)"
              >
                <span class="agent-card__avatar">{{ agent.name.slice(0, 1) }}</span>
                <span class="agent-card__body">
                  <strong>{{ agent.name }}</strong>
                  <small>{{ archetypeLabel(agent.archetype) }} · {{ statusLabel(agent.status) }}</small>
                </span>
                <span class="agent-card__revision">r{{ agent.revision }}</span>
              </button>
              <el-empty v-if="agents.length === 0" description="尚未创建 Agent" :image-size="72" />
            </div>
          </article>
          <article class="panel">
            <div class="panel__title">
              <div>
                <h2>活动流</h2>
                <p>实时显示等待原因、成本和执行状态。</p>
              </div>
              <el-button text @click="activeTab = 'activity'">查看全部</el-button>
            </div>
            <div class="timeline">
              <div v-for="event in overview.activity" :key="event.id" class="timeline__item">
                <span class="timeline__dot" :class="event.state"></span>
                <div>
                  <strong>{{ event.title }}</strong>
                  <p>{{ event.detail || event.waitingReason || event.kind }}</p>
                  <small>{{ dateTime(event.occurredAt) }} · {{ event.state }}<template v-if="event.actorType"> · {{ event.actorType }} {{ event.actorId }}</template></small>
                </div>
              </div>
              <el-empty v-if="overview.activity.length === 0" description="暂无活动" :image-size="64" />
            </div>
          </article>
        </div>
      </section>

      <section v-else-if="activeTab === 'agents'" class="agents-layout">
        <aside class="list-panel">
          <div class="list-panel__header">
            <h2>Agent</h2>
            <el-button circle :icon="Plus" @click="openAgentDialog" />
          </div>
          <button
            v-for="agent in agents"
            :key="agent.id"
            class="agent-row"
            :class="{ active: selectedAgent?.profile.id === agent.id }"
            @click="selectAgent(agent.id)"
          >
            <span class="agent-row__avatar">{{ agent.name.slice(0, 1) }}</span>
            <span>
              <strong>{{ agent.name }}</strong>
              <small>{{ archetypeLabel(agent.archetype) }}</small>
            </span>
            <el-tag size="small" :type="agent.status === 'active' ? 'success' : 'info'">{{ statusLabel(agent.status) }}</el-tag>
          </button>
        </aside>

        <article v-if="selectedAgent" class="detail-panel">
          <div class="detail-panel__hero">
            <div class="detail-panel__identity">
              <span class="detail-panel__avatar">{{ selectedAgent.profile.name.slice(0, 1) }}</span>
              <div>
                <h2>{{ selectedAgent.profile.name }}</h2>
                <p>{{ selectedAgent.profile.description || "暂无描述" }}</p>
                <div class="chips">
                  <span>{{ archetypeLabel(selectedAgent.profile.archetype) }}</span>
                  <span>revision {{ selectedAgent.profile.revision }}</span>
                  <span>{{ selectedAgent.release?.version || "未发布" }}</span>
                </div>
              </div>
            </div>
            <div class="detail-panel__actions">
              <el-button @click="router.push({ path: '/chat', query: { characterId: selectedAgent.profile.characterId } })">开始对话</el-button>
              <el-button @click="testSelectedAgent">测试当前版本</el-button>
              <el-button type="primary" @click="publishSelectedAgent">发布</el-button>
              <el-dropdown trigger="click">
                <el-button>更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item @click="openAgentEditor">编辑当前版本</el-dropdown-item>
                    <el-dropdown-item @click="exportSelectedAgent">导出 Agent 包</el-dropdown-item>
                    <el-dropdown-item @click="disableSelectedAgent">停用 Agent</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>

          <div class="detail-grid">
            <section class="detail-card">
              <h3>运行配置</h3>
              <pre>{{ pretty(selectedAgent.revision.configJson) }}</pre>
            </section>
            <section class="detail-card">
              <h3>版本历史</h3>
              <div v-for="revision in selectedAgent.history" :key="revision.id" class="history-row">
                <div>
                  <strong>r{{ revision.revision }}</strong>
                  <p>{{ revision.changeNote || "未填写变更说明" }}</p>
                </div>
                <div class="history-row__meta">
                  <el-tag size="small" :type="revision.testStatus === 'passed' ? 'success' : 'info'">{{ revision.testStatus }}</el-tag>
                  <small>{{ dateTime(revision.createdAt) }}</small>
                  <el-button link @click="compareRevision(revision.revision)">对比</el-button>
                  <el-button
                    v-if="releaseForRevision(revision.id)"
                    link
                    type="warning"
                    @click="rollbackToRelease(releaseForRevision(revision.id)!.id)"
                  >回滚</el-button>
                </div>
              </div>
            </section>
          </div>
        </article>
        <el-empty v-else description="选择左侧 Agent 查看详情" />
      </section>

      <section v-else-if="activeTab === 'teams'" class="teams-layout">
        <div class="panel">
          <div class="panel__title">
            <div>
              <h2>Agent 团队</h2>
              <p>负责人、共享上下文、任务交接和团队级资源治理。</p>
            </div>
            <el-button type="primary" @click="teamDialog = true">新建团队</el-button>
          </div>
          <div class="human-members">
            <div>
              <h3>人类成员</h3>
              <p>邀请成员并授予最小范围权限。</p>
            </div>
            <el-button @click="inviteDialog = true">邀请成员</el-button>
          </div>
          <div v-for="user in platformUsers" :key="user.id" class="simple-row">
            <span>{{ user.displayName }} · {{ user.email || "未设置邮箱" }}</span>
            <el-tag size="small">{{ user.isLocalOwner ? "Owner" : "Member" }}</el-tag>
          </div>
          <el-empty v-if="platformUsers.length === 0" description="暂无人类成员" :image-size="56" />
          <div class="team-grid">
            <article v-for="team in teams" :key="team.id" class="team-card" @click="selectTeam(team.id)">
              <span class="team-card__mark">{{ team.name.slice(0, 1) }}</span>
              <div>
                <h3>{{ team.name }}</h3>
                <p>{{ team.description || "暂无团队说明" }}</p>
                <div class="chips">
                  <span>{{ team.sharedMemory ? "共享记忆" : "独立记忆" }}</span>
                  <span>{{ team.sharedWorkspace ? "共享文件" : "独立文件" }}</span>
                  <span>rev {{ team.revision }}</span>
                </div>
              </div>
            </article>
            <el-empty v-if="teams.length === 0" description="尚未创建团队" />
          </div>
        </div>
        <aside v-if="selectedTeam" class="detail-card">
          <h3>{{ selectedTeam.team?.name }}</h3>
          <p class="muted">{{ selectedTeam.team?.description }}</p>
          <h4>成员</h4>
          <el-button size="small" @click="addSelectedAgentToTeam">加入当前 Agent</el-button>
          <el-button size="small" @click="configureTeamBudget">配置团队预算</el-button>
          <el-button size="small" @click="testTeamRoute">测试消息路由</el-button>
          <div v-for="member in selectedTeam.members || []" :key="member.id" class="simple-row">
            <span>{{ member.memberType }} · {{ member.memberId }}</span>
            <el-tag size="small">{{ member.role }}</el-tag>
          </div>
          <el-empty v-if="(selectedTeam.members || []).length === 0" description="暂无成员" :image-size="56" />
          <h4>共享上下文</h4>
          <div class="policy-form">
            <el-input v-model="teamContextText" placeholder="添加团队规则、决策或共享知识" />
            <el-button @click="addTeamContext">添加</el-button>
          </div>
          <div v-for="context in selectedTeam.contexts || []" :key="context.id" class="context-row">
            <strong>{{ context.kind }} {{ context.key }}</strong>
            <p>{{ context.content }}</p>
          </div>
        </aside>
      </section>

      <section v-else-if="activeTab === 'inbox'" class="inbox-layout">
        <div class="panel">
          <div class="panel__title">
            <div>
              <h2>待审批</h2>
              <p>审批记录持久保存，重启和多端切换后仍可继续。</p>
            </div>
          </div>
          <div v-for="approval in approvals" :key="approval.id" class="inbox-card">
            <div>
              <span class="risk" :class="approval.riskLevel">{{ approval.riskLevel }}</span>
              <h3>{{ approval.actionSummary }}</h3>
              <p>{{ approval.resourceSummary || approval.toolName }}</p>
              <small>到期 {{ dateTime(approval.expiresAt) }}</small>
            </div>
            <div class="inbox-card__actions">
              <el-button @click="decide(approval, 'denied')">拒绝</el-button>
              <el-button type="primary" @click="decide(approval, 'approved')">允许一次</el-button>
            </div>
          </div>
          <el-empty v-if="approvals.length === 0" description="没有待审批操作" />
        </div>
        <div class="panel">
          <div class="panel__title">
            <div>
              <h2>待对账</h2>
              <p>外部副作用结果不明确时不会自动重复执行。</p>
            </div>
          </div>
          <div v-for="item in reconciliations" :key="item.id" class="inbox-card">
            <div>
              <h3>{{ item.reason }}</h3>
              <p class="error-text">{{ item.originalError || "没有附加原始错误" }}</p>
            </div>
            <el-button @click="resolveCase(item)">标记已核对</el-button>
          </div>
          <el-empty v-if="reconciliations.length === 0" description="没有待对账记录" />
        </div>
      </section>

      <section v-else-if="activeTab === 'runtimes'" class="panel">
        <div class="panel__title">
          <div>
            <h2>Agent 电脑</h2>
            <p>本机、OCI、远程和 Android 运行时使用统一生命周期。</p>
          </div>
          <el-button type="primary" @click="runtimeDialog = true">创建电脑</el-button>
        </div>
        <div class="runtime-grid">
          <article v-for="runtime in runtimes" :key="runtime.id" class="runtime-card">
            <div class="runtime-card__head">
              <div>
                <span class="runtime-provider">{{ runtime.provider }}</span>
                <h3>{{ agentName(runtime.agentId) }}</h3>
              </div>
              <el-tag :type="runtime.state === 'running' ? 'success' : runtime.state === 'paused' ? 'warning' : 'info'">{{ runtime.state }}</el-tag>
            </div>
            <dl>
              <div><dt>CPU</dt><dd>{{ runtime.cpuQuota }}</dd></div>
              <div><dt>内存</dt><dd>{{ bytes(runtime.memoryBytes) }}</dd></div>
              <div><dt>Endpoint</dt><dd>{{ runtime.endpoint || (runtime.provider === "local" ? "本机工作区" : "由 Provider 管理") }}</dd></div>
            </dl>
            <div class="runtime-actions">
              <el-button v-if="!['running', 'paused'].includes(runtime.state)" size="small" @click="runtimeAct(runtime, 'start')">启动</el-button>
              <el-button v-if="runtime.state === 'running'" size="small" @click="runtimeAct(runtime, 'pause')">暂停</el-button>
              <el-button v-if="runtime.state === 'paused'" size="small" @click="runtimeAct(runtime, 'resume')">恢复</el-button>
              <el-button size="small" @click="runtimeAct(runtime, 'stop')">停止</el-button>
              <el-button size="small" @click="createSnapshot(runtime)">快照</el-button>
              <el-button size="small" :disabled="runtime.state !== 'running'" @click="openRuntimeTerminal(runtime)">终端</el-button>
              <el-button size="small" @click="takeOverDesktop(runtime)">接管桌面</el-button>
            </div>
          </article>
          <el-empty v-if="runtimes.length === 0" description="尚未创建 Agent 电脑" />
        </div>
      </section>

      <section v-else-if="activeTab === 'governance'" class="governance-layout">
        <article class="panel">
          <div class="panel__title">
            <div>
              <h2>自治与预算</h2>
              <p>控制 Agent 能自主执行的范围、频率和成本。</p>
            </div>
            <el-button type="primary" @click="saveAutonomy">保存</el-button>
          </div>
          <el-form v-if="selectedAgent" label-position="top">
            <div class="form-grid">
              <el-form-item label="自治等级">
                <el-select v-model="autonomy.level">
                  <el-option label="0 · 仅观察" :value="0" />
                  <el-option label="1 · 建议" :value="1" />
                  <el-option label="2 · 自动低风险" :value="2" />
                  <el-option label="3 · 外部操作审批" :value="3" />
                  <el-option label="4 · 长期托管" :value="4" />
                </el-select>
              </el-form-item>
              <el-form-item label="每日最大动作">
                <el-input-number v-model="autonomy.maxActionsPerDay" :min="1" :max="1000" />
              </el-form-item>
            </div>
            <el-form-item label="安静时段"><el-input v-model="autonomy.quietHours" placeholder="23:00-07:00" /></el-form-item>
            <el-form-item label="失败升级给"><el-input v-model="autonomy.escalationTarget" placeholder="团队负责人或用户" /></el-form-item>
            <el-checkbox v-model="autonomy.autoLowRisk">允许自动执行低风险动作</el-checkbox>
            <el-checkbox v-model="autonomy.externalApproval">外部副作用必须审批</el-checkbox>
          </el-form>
        </article>
        <article class="panel">
          <div class="panel__title">
            <div>
              <h2>网络与凭据</h2>
              <p>按 Agent 限制域名、外传和应用授权。</p>
            </div>
          </div>
          <div class="policy-form">
            <el-input v-model="networkForm.domain" placeholder="允许访问的域名，例如 api.example.com" />
            <el-button @click="addNetworkPolicy">添加域名策略</el-button>
          </div>
          <div v-for="policy in networkPolicies" :key="policy.id" class="simple-row">
            <span>{{ policy.name }} · {{ policy.defaultAction }}</span>
            <el-tag size="small">v{{ policy.revision }}</el-tag>
          </div>
          <h4>Agent 应用授权</h4>
          <div class="credential-form">
            <el-input v-model="credentialForm.provider" placeholder="Provider，例如 gmail" />
            <el-input v-model="credentialForm.accountId" placeholder="账号 ID" />
            <el-input v-model="credentialForm.secretValue" type="password" show-password placeholder="OAuth Token 或应用密钥" />
            <el-button @click="addCredentialGrant">授权</el-button>
          </div>
          <div v-for="grant in credentialGrants" :key="grant.id" class="simple-row">
            <span>
              {{ grant.provider }} · {{ grant.accountId || grant.connectionId }}
              <small class="credential-ref">{{ grant.secretRef }}</small>
            </span>
            <el-button size="small" text type="danger" @click="revokeGrant(grant.id)">撤销</el-button>
          </div>
          <el-empty v-if="credentialGrants.length === 0" description="暂无应用授权" :image-size="54" />
        </article>
        <article class="panel governance-budget">
          <div class="panel__title">
            <div>
              <h2>Agent 预算</h2>
              <p>Token、费用、工具次数和并发上限。</p>
            </div>
            <el-button @click="addBudget">新增预算</el-button>
          </div>
          <div v-for="budget in budgetPolicies" :key="budget.id" class="simple-row">
            <span>{{ budget.period }} · Token {{ budget.tokenLimit }} · 并发 {{ budget.concurrencyLimit }}</span>
            <el-tag size="small">{{ budget.hardStop ? "硬停止" : "预警" }}</el-tag>
          </div>
          <el-empty v-if="budgetPolicies.length === 0" description="暂无预算策略" :image-size="54" />
        </article>
        <article class="panel governance-a2a">
          <div class="panel__title">
            <div>
              <h2>外部 Agent 互联</h2>
              <p>使用 A2A Agent Card 发现和调用远程 Agent。</p>
            </div>
            <el-button @click="registerCurrentA2ACard">注册当前 Agent</el-button>
            <el-button @click="callRemoteAgent">调用远程 Agent</el-button>
          </div>
          <div v-for="card in a2aCards" :key="card.id" class="simple-row">
            <span>{{ card.name }} · {{ card.url }}</span>
            <el-tag size="small">{{ card.status }}</el-tag>
          </div>
          <el-empty v-if="a2aCards.length === 0" description="暂无 A2A Agent Card" :image-size="54" />
          <h4>最近远程任务</h4>
          <div v-for="task in a2aTasks" :key="task.id" class="simple-row">
            <span>{{ task.remoteAgentUrl || task.agentId }}</span>
            <el-tag size="small">{{ task.status }}</el-tag>
          </div>
        </article>
        <article class="panel governance-eval">
          <div class="panel__title">
            <div>
              <h2>评估门禁</h2>
              <p>发布前验证任务成功率、工具准确率、安全违规、成本与恢复能力。</p>
            </div>
            <el-button @click="createDefaultEvaluationSuite">创建评估套件</el-button>
          </div>
          <div v-for="suite in evaluationSuites" :key="suite.id" class="simple-row">
            <span>{{ suite.name }} · {{ suite.description || "未填写说明" }}</span>
            <div class="inline-actions">
              <el-tag size="small">v{{ suite.revision || 1 }}</el-tag>
              <el-button link type="primary" @click="runSuite(suite.id)">运行门禁</el-button>
            </div>
          </div>
          <el-empty v-if="evaluationSuites.length === 0" description="暂无评估套件" :image-size="54" />
        </article>
      </section>

      <section v-else-if="activeTab === 'activity'" class="panel">
        <div class="panel__title">
          <div>
            <h2>执行活动</h2>
            <p>等待原因、动作时间、成本和结果统一展示。</p>
          </div>
          <el-select v-model="activityState" clearable placeholder="全部状态" @change="loadActivity">
            <el-option label="运行中" value="running" />
            <el-option label="等待" value="pending" />
            <el-option label="完成" value="completed" />
            <el-option label="失败" value="failed" />
          </el-select>
        </div>
        <div class="activity-table">
          <div v-for="event in activity" :key="event.id" class="activity-row">
            <span class="activity-row__state" :class="event.state"></span>
            <div class="activity-row__main">
              <strong>{{ event.title }}</strong>
              <p>{{ event.detail || event.waitingReason || event.kind }}</p>
            </div>
            <span>{{ agentName(event.agentId) }}</span>
            <span>{{ event.actorType ? `${event.actorType} ${event.actorId}` : "系统" }}</span>
            <span>{{ money(event.costMicroUsd) }}</span>
            <span>{{ dateTime(event.occurredAt) }}</span>
          </div>
        </div>
        <el-empty v-if="activity.length === 0" description="暂无活动" />
      </section>
    </main>

    <el-dialog v-model="agentDialog" title="新建 Agent" width="680px">
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item label="名称">
            <el-input v-model="agentForm.name" placeholder="例如：研究助理" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="agentForm.archetype">
              <el-option label="陪伴型" value="companion" />
              <el-option label="执行型" value="worker" />
              <el-option label="团队负责人" value="team_lead" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="描述">
          <el-input v-model="agentForm.description" type="textarea" :rows="2" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="默认模型">
            <el-input v-model="agentForm.model" placeholder="例如：deepseek-chat" />
          </el-form-item>
          <el-form-item label="运行时">
            <el-select v-model="agentForm.runtimeProvider">
              <el-option label="本机工作区" value="local" />
              <el-option label="隔离容器（推荐）" value="isolated" />
              <el-option label="OCI 容器" value="oci" />
              <el-option label="远程电脑" value="remote" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="系统职责">
          <el-input v-model="agentForm.instructions" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="记忆范围">
          <el-radio-group v-model="agentForm.memoryScope">
            <el-radio-button value="agent">独立</el-radio-button>
            <el-radio-button value="team">团队共享</el-radio-button>
            <el-radio-button value="user">用户上下文</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="agentDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitAgent">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="teamDialog" title="新建团队" width="520px">
      <el-form label-position="top">
        <el-form-item label="团队模板">
          <el-select v-model="teamForm.templateId" clearable placeholder="从空白团队开始" @change="applyTeamTemplate">
            <el-option v-for="template in teamTemplates" :key="template.id" :label="template.name" :value="template.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="团队名称"><el-input v-model="teamForm.name" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="teamForm.description" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="负责人">
          <el-select v-model="teamForm.leadAgentId" clearable placeholder="稍后指定">
            <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
          </el-select>
        </el-form-item>
        <el-checkbox v-model="teamForm.sharedMemory">共享团队记忆</el-checkbox>
        <el-checkbox v-model="teamForm.sharedWorkspace">共享团队文件</el-checkbox>
      </el-form>
      <template #footer>
        <el-button @click="teamDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitTeam">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="runtimeDialog" title="创建 Agent 电脑" width="560px">
      <el-form label-position="top">
        <el-form-item label="Agent">
          <el-select v-model="runtimeForm.agentId">
            <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
          </el-select>
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="Provider">
            <el-select v-model="runtimeForm.provider">
              <el-option label="本机工作区" value="local" />
              <el-option label="隔离容器（推荐）" value="isolated" />
              <el-option label="OCI 容器" value="oci" />
              <el-option label="Android" value="android" />
            </el-select>
          </el-form-item>
          <el-form-item label="镜像">
            <el-input v-model="runtimeForm.image" :disabled="!['oci', 'isolated'].includes(runtimeForm.provider)" placeholder="ubuntu:24.04" />
          </el-form-item>
        </div>
        <el-form-item v-if="runtimeForm.provider === 'remote'" label="远程 Endpoint">
          <el-input v-model="runtimeForm.endpoint" placeholder="https://agent.example.com/runtime" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="CPU"><el-input-number v-model="runtimeForm.cpuQuota" :min="0.25" :step="0.25" /></el-form-item>
          <el-form-item label="内存 MB"><el-input-number v-model="runtimeForm.memoryMB" :min="256" :step="256" /></el-form-item>
        </div>
        <el-form-item label="磁盘 GB"><el-input-number v-model="runtimeForm.diskGB" :min="1" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="runtimeDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitRuntime">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="terminalDialog" :title="`${terminalRuntime ? agentName(terminalRuntime.agentId) : 'Agent'} 终端`" width="760px">
      <el-form label-position="top">
        <el-form-item label="命令">
          <el-input v-model="terminalForm.command" type="textarea" :rows="4" class="terminal-input" placeholder="pwd" @keydown.meta.enter="executeRuntimeCommand" @keydown.ctrl.enter="executeRuntimeCommand" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="工作目录"><el-input v-model="terminalForm.workingDir" placeholder="workspace 内的相对路径" /></el-form-item>
          <el-form-item label="超时秒数"><el-input-number v-model="terminalForm.timeoutSeconds" :min="1" :max="600" /></el-form-item>
        </div>
      </el-form>
      <div class="terminal-result">
        <div class="terminal-result__meta">
          <span v-if="terminalResult">退出码 {{ terminalResult.exitCode }} · {{ terminalResult.durationMs }} ms · {{ terminalResult.provider }}</span>
          <span v-else>命令输出将在这里显示</span>
        </div>
        <pre>{{ terminalResult?.output || "尚无输出" }}</pre>
      </div>
      <template #footer>
        <el-button @click="terminalDialog = false">关闭</el-button>
        <el-button type="primary" :loading="terminalRunning" :disabled="!terminalForm.command.trim()" @click="executeRuntimeCommand">执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="inviteDialog" title="邀请人类成员" width="460px">
      <el-form label-position="top">
        <el-form-item label="邮箱"><el-input v-model="inviteForm.email" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="inviteForm.role">
            <el-option label="成员" value="member" />
            <el-option label="操作者" value="operator" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="inviteDialog = false">取消</el-button>
        <el-button type="primary" @click="inviteMember">发送邀请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="agentEditorVisible" title="编辑 Agent 当前版本" width="760px">
      <el-form label-position="top">
        <el-form-item label="配置 JSON">
          <el-input v-model="agentConfigDraft" type="textarea" :rows="18" class="config-editor" />
        </el-form-item>
        <el-form-item label="变更说明">
          <el-input v-model="agentChangeNote" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="agentEditorVisible = false">取消</el-button>
        <el-button type="primary" @click="saveAgentRevision">保存为新草稿</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="diffVisible" title="版本配置差异" width="760px">
      <pre class="diff-view">{{ pretty(agentDiff) }}</pre>
    </el-dialog>

    <el-dialog v-model="memberDialog" title="人类成员账户" width="520px">
      <el-tabs v-model="memberTab">
        <el-tab-pane label="成员登录" name="login">
          <el-form label-position="top">
            <el-form-item label="邮箱"><el-input v-model="memberForm.email" /></el-form-item>
            <el-form-item label="密码"><el-input v-model="memberForm.password" type="password" show-password /></el-form-item>
          </el-form>
          <el-button type="primary" @click="loginMember">登录</el-button>
        </el-tab-pane>
        <el-tab-pane label="接受邀请" name="invite">
          <el-form label-position="top">
            <el-form-item label="邀请令牌"><el-input v-model="memberForm.inviteToken" /></el-form-item>
            <el-form-item label="显示名称"><el-input v-model="memberForm.displayName" /></el-form-item>
            <el-form-item label="登录密码"><el-input v-model="memberForm.password" type="password" show-password /></el-form-item>
          </el-form>
          <el-button type="primary" @click="acceptInvitation">接受邀请</el-button>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <el-dialog v-model="hostControlDialog" title="桌面实时画面" width="min(1100px, 92vw)" @closed="stopHostPreview">
      <div class="host-preview">
        <img v-if="hostPreview" :src="hostPreview" alt="桌面实时画面" @click="handleHostClick" />
      </div>
      <div class="host-keyboard">
        <el-input v-model="hostKeyboardText" placeholder="向接管桌面发送文字" @keyup.enter="sendHostKeyboard" />
        <el-button @click="sendHostKeyboard">发送</el-button>
      </div>
      <template #footer>
        <el-button @click="hostControlDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus } from "@element-plus/icons-vue";
import {
  addTeamMember,
  acceptPlatformInvitation,
  approveHostControl,
  createAgent,
  createBudgetPolicy,
  createCredentialGrant,
  createEvaluationCase,
  createEvaluationSuite,
  createHostControl,
  createNetworkPolicy,
  createRuntime,
  createTeam,
  createTeamFromTemplate,
  createTeamContext,
  decideApproval,
  diffAgentRevisions,
  disableAgent,
  endHostControl,
  execRuntime,
  exportAgentArchive,
  getAgentAutonomy,
  executeHostControl,
  getAgent,
  getAgentPlatformOverview,
  getTeam,
  invitePlatformUser,
  importAgentArchive,
  importAgentPackage,
  listA2ACards,
  listA2ATasks,
  listActivity,
  listAgentReleases,
  listAgents,
  listApprovals,
  listBudgetPolicies,
  listCredentialGrants,
  listEvaluationSuites,
  listNetworkPolicies,
  listPlatformUsers,
  listReconciliations,
  listRuntimes,
  listTeams,
  listTeamTemplates,
  loginPlatformUser,
  logoutPlatformUser,
  platformSession,
  registerA2ACard,
  publishAgent,
  registerRemoteRuntime,
  rollbackAgent,
  resolveReconciliation,
  revokeCredentialGrant,
  runtimeAction,
  runEvaluation,
  routeTeamMessage,
  saveAgentAutonomy,
  snapshotRuntime,
  submitA2ATask,
  updateAgent,
  updateNetworkPolicy,
  testAgent,
  type ActivityEvent,
  type AgentDetail,
  type AgentProfile,
  type AgentRelease,
  type ApprovalRecord,
  type OverviewData,
  type ReconciliationCase,
  type RuntimeInstance,
  type RuntimeExecResult,
  type Team,
} from "@/api/agent-platform";
import { originalErrorMessage } from "@/runtime/original-error";

const router = useRouter();

const tabs = [
  { id: "overview", label: "概览" },
  { id: "agents", label: "Agent" },
  { id: "teams", label: "团队" },
  { id: "inbox", label: "待处理" },
  { id: "runtimes", label: "电脑" },
  { id: "governance", label: "治理" },
  { id: "activity", label: "活动" },
];

const activeTab = ref("overview");
const loading = ref(false);
const saving = ref(false);
const agents = ref<AgentProfile[]>([]);
const selectedAgent = ref<AgentDetail | null>(null);
const teams = ref<Team[]>([]);
const teamTemplates = ref<Record<string, any>[]>([]);
const platformUsers = ref<Record<string, any>[]>([]);
const selectedTeam = ref<Record<string, any> | null>(null);
const approvals = ref<ApprovalRecord[]>([]);
const reconciliations = ref<ReconciliationCase[]>([]);
const runtimes = ref<RuntimeInstance[]>([]);
const credentialGrants = ref<Record<string, any>[]>([]);
const networkPolicies = ref<Record<string, any>[]>([]);
const budgetPolicies = ref<Record<string, any>[]>([]);
const a2aCards = ref<Record<string, any>[]>([]);
const a2aTasks = ref<Record<string, any>[]>([]);
const evaluationSuites = ref<Record<string, any>[]>([]);
const agentReleases = ref<AgentRelease[]>([]);
const activity = ref<ActivityEvent[]>([]);
const activityState = ref("");
const agentDialog = ref(false);
const teamDialog = ref(false);
const runtimeDialog = ref(false);
const terminalDialog = ref(false);
const terminalRuntime = ref<RuntimeInstance | null>(null);
const terminalRunning = ref(false);
const terminalResult = ref<RuntimeExecResult | null>(null);
const hostPreview = ref("");
const hostControlDialog = ref(false);
const hostControlSessionID = ref("");
const hostKeyboardText = ref("");
const hostPreviewBusy = ref(false);
let hostPreviewTimer: ReturnType<typeof setInterval> | null = null;
const agentImportInput = ref<HTMLInputElement | null>(null);
const inviteDialog = ref(false);
const memberDialog = ref(false);
const memberTab = ref("login");
const platformSessionState = ref(platformSession());
const memberForm = reactive({ email: "", password: "", inviteToken: "", displayName: "" });
const agentEditorVisible = ref(false);
const agentConfigDraft = ref("");
const agentChangeNote = ref("");
const diffVisible = ref(false);
const agentDiff = ref<Record<string, unknown>>({});
const inviteForm = reactive({ email: "", role: "member" });
const overview = reactive<OverviewData>({
  agents: [],
  activity: [],
  summary: { running: 0, waiting: 0, failed: 0, completed: 0, costMicroUsd: 0, averageDurationMs: 0 },
  pendingApprovals: [],
  reconciliations: [],
});

const agentForm = reactive({
  name: "",
  description: "",
  archetype: "worker",
  model: "deepseek-chat",
  runtimeProvider: "oci",
  instructions: "你是一个可靠、谨慎并主动验证结果的执行型 Agent。",
  memoryScope: "agent",
});
const teamForm = reactive({
  templateId: "",
  name: "",
  description: "",
  leadAgentId: "",
  sharedMemory: true,
  sharedWorkspace: true,
});
const runtimeForm = reactive({
  agentId: "",
  provider: "oci",
  image: "ubuntu:24.04",
  cpuQuota: 1,
  memoryMB: 1024,
  diskGB: 10,
  endpoint: "",
});
const terminalForm = reactive({
  command: "pwd",
  workingDir: "",
  timeoutSeconds: 60,
});
const autonomy = reactive({
  level: 1,
  watchOnly: true,
  autoLowRisk: false,
  externalApproval: true,
  maxActionsPerDay: 20,
  maxCostMicroUsd: 0,
  quietHours: "",
  escalationTarget: "",
  triggers: [] as string[],
  expectedRevision: 0,
});
const networkForm = reactive({ domain: "" });
const credentialForm = reactive({ provider: "", accountId: "", secretValue: "" });
const teamContextText = ref("");

const agentNames = computed(() => new Map(agents.value.map((agent) => [agent.id, agent.name])));

async function refreshAll() {
  loading.value = true;
  try {
    await Promise.all([loadOverview(), loadAgents(), loadTeams(), loadInbox(), loadRuntimes(), loadActivity()]);
  } finally {
    loading.value = false;
  }
}

async function loadOverview() {
  const value = await getAgentPlatformOverview();
  Object.assign(overview, value);
  if (!selectedAgent.value && value.agents.length > 0) {
    await selectAgent(value.agents[0].id);
  }
}

async function loadAgents() {
  agents.value = await listAgents();
}

async function selectAgent(id: string) {
  const [detail, releases] = await Promise.all([getAgent(id), listAgentReleases(id)]);
  selectedAgent.value = detail;
  agentReleases.value = releases;
  await loadGovernance(id);
  activeTab.value = "agents";
}

async function loadGovernance(agentId: string) {
  const [policy, grants, network, budgets, cards, tasks, evaluations] = await Promise.all([
    getAgentAutonomy(agentId),
    listCredentialGrants(agentId),
    listNetworkPolicies(agentId),
    listBudgetPolicies("agent", agentId),
    listA2ACards(),
    listA2ATasks(agentId),
    listEvaluationSuites(agentId),
  ]);
  Object.assign(autonomy, policy);
  credentialGrants.value = grants;
  networkPolicies.value = network;
  budgetPolicies.value = budgets;
  a2aCards.value = cards;
  a2aTasks.value = tasks;
  evaluationSuites.value = evaluations;
}

async function loadTeams() {
  [teams.value, platformUsers.value, teamTemplates.value] = await Promise.all([listTeams(), listPlatformUsers(), listTeamTemplates()]);
}

function applyTeamTemplate(templateID: string) {
  const template = teamTemplates.value.find((item) => item.id === templateID);
  if (!template) return;
  teamForm.name = String(template.name || "");
  teamForm.description = String(template.description || "");
  teamForm.sharedMemory = Boolean(template.sharedMemory);
  teamForm.sharedWorkspace = Boolean(template.sharedWorkspace);
}

async function selectTeam(id: string) {
  selectedTeam.value = await getTeam(id);
}

async function loadInbox() {
  [approvals.value, reconciliations.value] = await Promise.all([listApprovals(), listReconciliations()]);
}

async function loadRuntimes() {
  runtimes.value = await listRuntimes();
}

async function loadActivity() {
  const value = await listActivity({ state: activityState.value, limit: 200 });
  activity.value = value.items;
}

function openAgentDialog() {
  agentDialog.value = true;
}

async function submitAgent() {
  if (!agentForm.name.trim()) {
    ElMessage.warning("请输入 Agent 名称");
    return;
  }
  saving.value = true;
  try {
    const detail = await createAgent({
      name: agentForm.name,
      description: agentForm.description,
      archetype: agentForm.archetype,
      config: {
        identity: { kind: agentForm.archetype },
        instructions: agentForm.instructions,
        model: { provider: "custom", name: agentForm.model },
        memory: { scope: agentForm.memoryScope },
        permissions: { mode: "request_approval" },
        runtime: { provider: agentForm.runtimeProvider },
      },
    });
    agentDialog.value = false;
    selectedAgent.value = detail;
    await loadAgents();
    activeTab.value = "agents";
    ElMessage.success("Agent 草稿已创建");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function testSelectedAgent() {
  if (!selectedAgent.value) return;
  try {
    const revision = await testAgent(selectedAgent.value.profile.id, selectedAgent.value.revision.id);
    await selectAgent(selectedAgent.value.profile.id);
    ElMessage.success(`版本 r${revision.revision} 测试通过`);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

function openAgentEditor() {
  if (!selectedAgent.value) return;
  try {
    agentConfigDraft.value = JSON.stringify(JSON.parse(selectedAgent.value.revision.configJson), null, 2);
    agentChangeNote.value = "";
    agentEditorVisible.value = true;
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function saveAgentRevision() {
  if (!selectedAgent.value) return;
  try {
    const config = JSON.parse(agentConfigDraft.value);
    await updateAgent(selectedAgent.value.profile.id, {
      expectedRevision: selectedAgent.value.profile.revision,
      changeNote: agentChangeNote.value,
      config,
    });
    agentEditorVisible.value = false;
    await loadAgents();
    await selectAgent(selectedAgent.value.profile.id);
    ElMessage.success("新草稿版本已保存");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function compareRevision(revision: number) {
  if (!selectedAgent.value) return;
  try {
    agentDiff.value = await diffAgentRevisions(
      selectedAgent.value.profile.id,
      revision,
      selectedAgent.value.revision.revision,
    );
    diffVisible.value = true;
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

function releaseForRevision(revisionID: string) {
  return agentReleases.value.find((release) => release.revisionId === revisionID);
}

async function rollbackToRelease(releaseID: string) {
  if (!selectedAgent.value) return;
  try {
    await ElMessageBox.confirm("回滚会把该发布版本重新设为 Agent 的当前版本。", "回滚 Agent", { type: "warning" });
    await rollbackAgent(selectedAgent.value.profile.id, {
      releaseId: releaseID,
      expectedRevision: selectedAgent.value.profile.revision,
    });
    await loadAgents();
    await selectAgent(selectedAgent.value.profile.id);
    ElMessage.success("Agent 已回滚");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

async function publishSelectedAgent() {
  if (!selectedAgent.value) return;
  try {
    const { value } = await ElMessageBox.prompt("输入发布版本", "发布 Agent", {
      inputValue: "1.0.0",
      confirmButtonText: "发布",
      cancelButtonText: "取消",
    });
    await publishAgent(selectedAgent.value.profile.id, {
      expectedRevision: selectedAgent.value.profile.revision,
      version: value,
      releaseNotes: "",
    });
    await selectAgent(selectedAgent.value.profile.id);
    await loadAgents();
    ElMessage.success("Agent 已发布");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

async function exportSelectedAgent() {
  if (!selectedAgent.value) return;
  try {
    const exported = await exportAgentArchive(selectedAgent.value.profile.id, selectedAgent.value.release?.version || "draft");
    const blob = exported.blob;
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = exported.fileName;
    link.click();
    URL.revokeObjectURL(url);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function importSelectedAgent(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  try {
    const detail = file.name.toLowerCase().endsWith(".zip")
      ? await importAgentArchive(file)
      : await importAgentPackage(JSON.parse(await file.text()));
    await loadAgents();
    selectedAgent.value = detail;
    activeTab.value = "agents";
    ElMessage.success("Agent 包已导入");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  } finally {
    input.value = "";
  }
}

async function disableSelectedAgent() {
  if (!selectedAgent.value) return;
  try {
    await ElMessageBox.confirm("停用后 Agent 不能开始新执行，已有记录会保留。", "停用 Agent", { type: "warning" });
    await disableAgent(selectedAgent.value.profile.id, selectedAgent.value.profile.revision);
    await selectAgent(selectedAgent.value.profile.id);
    await loadAgents();
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

async function submitTeam() {
  if (!teamForm.name.trim()) {
    ElMessage.warning("请输入团队名称");
    return;
  }
  saving.value = true;
  try {
    const team = teamForm.templateId
      ? await createTeamFromTemplate({ ...teamForm })
      : await createTeam({ ...teamForm });
    teamDialog.value = false;
    teamForm.name = "";
    teamForm.description = "";
    teamForm.templateId = "";
    await loadTeams();
    await selectTeam(team.id);
    ElMessage.success("团队已创建");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function inviteMember() {
  if (!inviteForm.email.trim()) return;
  try {
    const result = await invitePlatformUser({
      email: inviteForm.email.trim(),
      role: inviteForm.role,
      teamId: selectedTeam.value?.team?.id || "",
    });
    inviteDialog.value = false;
    inviteForm.email = "";
    await ElMessageBox.alert(result.token || "邀请已创建", "邀请令牌只显示一次，请安全发送给受邀成员");
    await loadTeams();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

function openMemberDialog() {
  memberDialog.value = true;
}

async function loginMember() {
  try {
    platformSessionState.value = await loginPlatformUser(memberForm.email.trim(), memberForm.password);
    memberDialog.value = false;
    memberForm.password = "";
    ElMessage.success("成员账户已登录");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function acceptInvitation() {
  try {
    const user = await acceptPlatformInvitation({
      token: memberForm.inviteToken.trim(),
      displayName: memberForm.displayName.trim(),
      password: memberForm.password,
    });
    memberForm.email = String(user.email || memberForm.email);
    memberTab.value = "login";
    ElMessage.success("邀请已接受，请使用邮箱和密码登录");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function logoutMember() {
  try {
    await logoutPlatformUser();
    platformSessionState.value = null;
    ElMessage.success("成员账户已退出");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function addSelectedAgentToTeam() {
  if (!selectedTeam.value || !selectedAgent.value) return;
  try {
    await addTeamMember(String(selectedTeam.value.team.id), {
      memberType: "agent",
      memberId: selectedAgent.value.profile.id,
      role: "member",
      responderMode: "mention_or_lead",
      capabilities: [],
    });
    await selectTeam(String(selectedTeam.value.team.id));
    ElMessage.success("Agent 已加入团队");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function addTeamContext() {
  if (!selectedTeam.value || !teamContextText.value.trim()) return;
  try {
    await createTeamContext(String(selectedTeam.value.team.id), {
      kind: "instruction",
      key: "",
      content: teamContextText.value.trim(),
      visibility: "team",
    });
    teamContextText.value = "";
    await selectTeam(String(selectedTeam.value.team.id));
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function configureTeamBudget() {
  if (!selectedTeam?.value?.team?.id) return;
  try {
    const { value } = await ElMessageBox.prompt("每日团队 Token 上限", "团队预算", {
      inputValue: "1000000",
      inputPattern: /^\d+$/,
      inputErrorMessage: "请输入正整数",
    });
    await createBudgetPolicy({
      scopeType: "team",
      scopeId: selectedTeam.value.team.id,
      period: "daily",
      tokenLimit: Number(value),
      costLimitMicroUsd: 0,
      toolCallLimit: 500,
      concurrencyLimit: 4,
      warningPercent: 80,
      hardStop: true,
    });
    ElMessage.success("团队预算已保存");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

async function testTeamRoute() {
  if (!selectedTeam?.value?.team?.id) return;
  try {
    const { value } = await ElMessageBox.prompt("输入一条团队测试消息", "团队消息路由");
    const result = await routeTeamMessage(selectedTeam.value.team.id, {
      message: value,
      conversationId: `team-route-${crypto.randomUUID()}`,
    });
    await ElMessageBox.alert(JSON.stringify(result, null, 2), "路由结果");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

async function decide(approval: ApprovalRecord, decision: "approved" | "denied") {
  try {
    await decideApproval(approval.id, {
      decision,
      grantMode: "once",
      reason: "",
      expectedRevision: approval.revision,
    });
    await loadInbox();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function resolveCase(item: ReconciliationCase) {
  try {
    await resolveReconciliation(item.id, {
      resolution: "用户已人工核对执行结果",
      expectedRevision: item.revision,
    });
    await loadInbox();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function submitRuntime() {
  if (!runtimeForm.agentId) {
    ElMessage.warning("请选择 Agent");
    return;
  }
  saving.value = true;
  try {
    const runtimePayload = {
      agentId: runtimeForm.agentId,
      provider: runtimeForm.provider,
      image: ["oci", "isolated"].includes(runtimeForm.provider) ? runtimeForm.image : "",
      cpuQuota: runtimeForm.cpuQuota,
      memoryBytes: runtimeForm.memoryMB * 1024 * 1024,
      diskBytes: runtimeForm.diskGB * 1024 * 1024 * 1024,
      processLimit: 64,
      idleTimeoutSeconds: 1800,
    };
    if (runtimeForm.provider === "remote") {
      await registerRemoteRuntime({ runtime: runtimePayload, endpoint: runtimeForm.endpoint });
    } else {
      await createRuntime(runtimePayload);
    }
    runtimeDialog.value = false;
    await loadRuntimes();
    ElMessage.success("Agent 电脑已创建");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function runtimeAct(runtime: RuntimeInstance, action: string) {
  try {
    await runtimeAction(runtime.id, action);
    await loadRuntimes();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function createSnapshot(runtime: RuntimeInstance) {
  try {
    await snapshotRuntime(runtime.id, `${agentName(runtime.agentId)}-${new Date().toISOString()}`);
    ElMessage.success("快照已创建");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

function openRuntimeTerminal(runtime: RuntimeInstance) {
  terminalRuntime.value = runtime;
  terminalResult.value = null;
  terminalDialog.value = true;
}

async function executeRuntimeCommand() {
  const runtime = terminalRuntime.value;
  if (!runtime || !terminalForm.command.trim()) return;
  terminalRunning.value = true;
  try {
    terminalResult.value = await execRuntime(runtime.id, {
      command: terminalForm.command,
      workingDir: terminalForm.workingDir,
      timeoutSeconds: terminalForm.timeoutSeconds,
    });
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  } finally {
    terminalRunning.value = false;
  }
}

async function takeOverDesktop(runtime: RuntimeInstance) {
  try {
    const session = await createHostControl(runtime.agentId, ["screen.capture", "input.mouse", "input.keyboard", "window.control"]);
    await approveHostControl(String(session.id));
    hostControlSessionID.value = String(session.id);
    await refreshHostPreview();
    hostControlDialog.value = true;
    startHostPreview();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function refreshHostPreview() {
  if (!hostControlSessionID.value || hostPreviewBusy.value) return;
  hostPreviewBusy.value = true;
  try {
    const capture = await executeHostControl(hostControlSessionID.value, { action: "screen.capture" });
    hostPreview.value = `data:${capture.mimeType};base64,${capture.base64}`;
  } finally {
    hostPreviewBusy.value = false;
  }
}

function startHostPreview() {
  if (hostPreviewTimer) clearInterval(hostPreviewTimer);
  hostPreviewTimer = setInterval(() => {
    refreshHostPreview().catch((error) => ElMessage.error(originalErrorMessage(error)));
  }, 1200);
}

function stopHostPreview() {
  if (hostPreviewTimer) clearInterval(hostPreviewTimer);
  hostPreviewTimer = null;
  hostPreviewBusy.value = false;
  const sessionID = hostControlSessionID.value;
  hostControlSessionID.value = "";
  if (sessionID) {
    endHostControl(sessionID).catch((error) => ElMessage.error(originalErrorMessage(error)));
  }
}

async function handleHostClick(event: MouseEvent) {
  if (!hostControlSessionID.value) return;
  const image = event.currentTarget as HTMLImageElement;
  const rect = image.getBoundingClientRect();
  const x = Math.round(((event.clientX - rect.left) / rect.width) * image.naturalWidth);
  const y = Math.round(((event.clientY - rect.top) / rect.height) * image.naturalHeight);
  try {
    await executeHostControl(hostControlSessionID.value, { action: "mouse.click", x, y, button: "left" });
    await refreshHostPreview();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function sendHostKeyboard() {
  if (!hostControlSessionID.value || !hostKeyboardText.value) return;
  try {
    await executeHostControl(hostControlSessionID.value, { action: "keyboard.type", text: hostKeyboardText.value });
    hostKeyboardText.value = "";
    await refreshHostPreview();
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

onBeforeUnmount(stopHostPreview);

async function saveAutonomy() {
  if (!selectedAgent.value) return;
  try {
    const policy = await saveAgentAutonomy(selectedAgent.value.profile.id, {
      level: autonomy.level,
      watchOnly: autonomy.level === 0,
      autoLowRisk: autonomy.autoLowRisk,
      externalApproval: autonomy.externalApproval,
      maxActionsPerDay: autonomy.maxActionsPerDay,
      maxCostMicroUsd: autonomy.maxCostMicroUsd,
      quietHours: autonomy.quietHours,
      escalationTarget: autonomy.escalationTarget,
      triggers: autonomy.triggers,
      expectedRevision: autonomy.expectedRevision,
    });
    Object.assign(autonomy, policy);
    ElMessage.success("自治策略已保存");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function addNetworkPolicy() {
  if (!selectedAgent.value || !networkForm.domain.trim()) return;
  try {
    const existing = networkPolicies.value.find((policy) => policy.agentId === selectedAgent.value?.profile.id);
    const domain = networkForm.domain.trim();
    if (existing) {
      const current = parsePolicyArray<string>(existing.allowDomains, "[]");
      const allowDomains = Array.from(new Set([...current, domain]));
      await updateNetworkPolicy(existing.id, {
        name: existing.name,
        defaultAction: existing.defaultAction,
        allowDomains,
        denyDomains: parsePolicyArray<string>(existing.denyDomains, "[]"),
        allowedPorts: parsePolicyArray<number>(existing.allowedPorts, "[443]"),
        uploadPolicy: existing.uploadPolicy,
        sensitiveDataPolicy: existing.sensitiveDataPolicy,
        expectedRevision: existing.revision,
      });
    } else {
      await createNetworkPolicy({
        agentId: selectedAgent.value.profile.id,
        name: `${selectedAgent.value.profile.name} 网络策略`,
        defaultAction: "deny",
        allowDomains: [domain],
        denyDomains: [],
        allowedPorts: [443],
        uploadPolicy: "approval",
        sensitiveDataPolicy: "block",
      });
    }
    networkForm.domain = "";
    await loadGovernance(selectedAgent.value.profile.id);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

function parsePolicyArray<T>(value: unknown, fallback: string): T[] {
  if (Array.isArray(value)) return value as T[];
  const parsed = JSON.parse(typeof value === "string" && value.trim() ? value : fallback);
  if (!Array.isArray(parsed)) throw new Error("网络策略字段格式无效");
  return parsed as T[];
}

async function addBudget() {
  if (!selectedAgent.value) return;
  try {
    await createBudgetPolicy({
      scopeType: "agent",
      scopeId: selectedAgent.value.profile.id,
      period: "monthly",
      tokenLimit: 1000000,
      costLimitMicroUsd: 10_000_000,
      toolCallLimit: 1000,
      concurrencyLimit: 2,
      warningPercent: 80,
      hardStop: true,
    });
    await loadGovernance(selectedAgent.value.profile.id);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function revokeGrant(id: string) {
  if (!selectedAgent.value) return;
  try {
    await revokeCredentialGrant(id);
    await loadGovernance(selectedAgent.value.profile.id);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function addCredentialGrant() {
  if (!selectedAgent.value || !credentialForm.provider.trim() || !credentialForm.secretValue) return;
  try {
    await createCredentialGrant({
      agentId: selectedAgent.value.profile.id,
      connectionId: `connection-${crypto.randomUUID()}`,
      provider: credentialForm.provider.trim(),
      accountId: credentialForm.accountId.trim(),
      scopes: ["agent.identity.*"],
      secretRef: "",
      secretValue: credentialForm.secretValue,
    });
    credentialForm.provider = "";
    credentialForm.accountId = "";
    credentialForm.secretValue = "";
    await loadGovernance(selectedAgent.value.profile.id);
    ElMessage.success("Agent 应用授权已保存");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function registerCurrentA2ACard() {
  if (!selectedAgent.value) return;
  try {
    await registerA2ACard({
      agentId: selectedAgent.value.profile.id,
      name: selectedAgent.value.profile.name,
      description: selectedAgent.value.profile.description,
      url: `${window.location.origin}/api/public/a2a/agents/${encodeURIComponent(selectedAgent.value.profile.id)}`,
      version: selectedAgent.value.release?.version || "1.0.0",
      skills: [],
      security: { approvalRequired: true },
    });
    await loadGovernance(selectedAgent.value.profile.id);
    ElMessage.success("A2A Agent Card 已注册");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function createDefaultEvaluationSuite() {
  if (!selectedAgent.value) return;
  try {
    const suite = await createEvaluationSuite({
      agentId: selectedAgent.value.profile.id,
      name: `${selectedAgent.value.profile.name} 发布门禁`,
      description: "Agent 发布前综合评估",
      config: {},
      gatePolicy: {
        taskSuccessRate: 0.9,
        toolAccuracy: 0.85,
        falseSuccessRate: 0.05,
        recoveryRate: 0.8,
      },
    });
    await createEvaluationCase(String(suite.id), {
      name: "基础可用性",
      category: "general",
      input: { message: "请用一句话说明你已准备好执行当前任务。" },
      expected: {},
      safetyCritical: false,
      weight: 1,
    });
    await loadGovernance(selectedAgent.value.profile.id);
    ElMessage.success("评估套件已创建");
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function runSuite(suiteID: string) {
  if (!selectedAgent.value) return;
  try {
    const result = await runEvaluation(suiteID, {
      revisionId: selectedAgent.value.revision.id,
      results: [],
    });
    ElMessage.success(`评估完成：${result.gateStatus || result.status}`);
    await loadGovernance(selectedAgent.value.profile.id);
  } catch (error) {
    ElMessage.error(originalErrorMessage(error));
  }
}

async function callRemoteAgent() {
  if (!selectedAgent.value) return;
  try {
    const { value: remoteAgentUrl } = await ElMessageBox.prompt("输入远程 A2A 端点", "调用远程 Agent", {
      inputPlaceholder: "https://example.com/a2a/tasks",
    });
    const { value: message } = await ElMessageBox.prompt("输入任务内容", "A2A 任务", {
      inputType: "textarea",
    });
    const result = await submitA2ATask({
      agentId: selectedAgent.value.profile.id,
      remoteAgentUrl,
      input: { message },
    });
    await ElMessageBox.alert(JSON.stringify(result.remote ?? result.task, null, 2), "远程 Agent 结果");
    await loadGovernance(selectedAgent.value.profile.id);
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(originalErrorMessage(error));
  }
}

function agentName(id: string) {
  return agentNames.value.get(id) || id || "系统";
}

function archetypeLabel(value: string) {
  return ({ companion: "陪伴型", worker: "执行型", team_lead: "团队负责人" } as Record<string, string>)[value] || value;
}

function statusLabel(value: string) {
  return ({ draft: "草稿", active: "运行中", disabled: "已停用" } as Record<string, string>)[value] || value;
}

function money(value: number) {
  return `$${(Number(value || 0) / 1_000_000).toFixed(4)}`;
}

function bytes(value: number) {
  if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(1)} GB`;
  if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(0)} MB`;
  return `${value} B`;
}

function pretty(value: unknown) {
  if (typeof value !== "string") {
    return JSON.stringify(value, null, 2);
  }
  try {
    return JSON.stringify(JSON.parse(value || "{}"), null, 2);
  } catch {
    return value;
  }
}

function dateTime(value: string) {
  if (!value) return "未知";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

onMounted(() => {
  refreshAll().catch((error) => ElMessage.error(originalErrorMessage(error)));
});
</script>

<style scoped>
.agent-os {
  min-height: 100%;
  padding: 28px;
  color: var(--text-primary);
  background:
    radial-gradient(circle at 88% 4%, color-mix(in srgb, var(--accent) 14%, transparent), transparent 28%),
    var(--workbench-bg);
}

.agent-os__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  max-width: 1480px;
  margin: 0 auto 22px;
}

.agent-os__header h1 {
  margin: 5px 0 6px;
  font-size: clamp(26px, 3vw, 40px);
  letter-spacing: -0.04em;
}

.agent-os__header p {
  margin: 0;
  color: var(--text-secondary);
}

.agent-os__eyebrow {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  color: var(--accent) !important;
}

.agent-os__header-actions {
  display: flex;
  gap: 10px;
}

.agent-os__tabs {
  display: flex;
  gap: 6px;
  max-width: 1480px;
  margin: 0 auto 18px;
  padding: 5px;
  overflow-x: auto;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface-bg) 92%, transparent);
}

.agent-os__tabs button {
  flex: 0 0 auto;
  padding: 9px 15px;
  border: 0;
  border-radius: 10px;
  color: var(--text-secondary);
  background: transparent;
  cursor: pointer;
}

.agent-os__tabs button.active {
  color: var(--text-primary);
  background: var(--surface-soft);
  box-shadow: 0 3px 12px rgba(0, 0, 0, 0.08);
}

.agent-os__content {
  max-width: 1480px;
  min-height: 520px;
  margin: 0 auto;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 14px;
}

.metrics article,
.panel,
.detail-card,
.list-panel,
.detail-panel,
.team-card,
.runtime-card,
.market-card {
  border: 1px solid var(--surface-border);
  background: var(--surface-bg);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.04);
}

.metrics article {
  display: grid;
  gap: 4px;
  padding: 20px;
  border-radius: 16px;
}

.metrics span,
.metrics small,
.muted,
.agent-card small,
.agent-row small,
.team-card p,
.runtime-card dd,
.activity-row p {
  color: var(--text-secondary);
}

.metrics strong {
  font-size: 30px;
  letter-spacing: -0.04em;
}

.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(360px, 0.85fr);
  gap: 14px;
}

.panel {
  padding: 20px;
  border-radius: 18px;
}

.panel__title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.panel__title h2,
.detail-panel h2,
.detail-card h3,
.team-card h3,
.runtime-card h3,
.market-card h3 {
  margin: 0;
}

.panel__title p,
.team-card p,
.market-card p {
  margin: 4px 0 0;
  color: var(--text-secondary);
}

.agent-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.agent-card,
.agent-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 14px;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  color: inherit;
  text-align: left;
  background: var(--surface-soft);
  cursor: pointer;
}

.agent-card:hover,
.agent-row:hover,
.team-card:hover {
  border-color: color-mix(in srgb, var(--accent) 45%, var(--surface-border));
}

.agent-card__avatar,
.agent-row__avatar,
.detail-panel__avatar,
.team-card__mark,
.market-card__icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 12px;
  color: white;
  font-weight: 700;
  background: linear-gradient(145deg, var(--accent), color-mix(in srgb, var(--accent) 45%, #111));
}

.agent-card__body {
  display: grid;
  min-width: 0;
  flex: 1;
}

.agent-card__revision {
  color: var(--text-secondary);
  font: 12px ui-monospace, SFMono-Regular, monospace;
}

.timeline {
  display: grid;
  gap: 2px;
}

.timeline__item {
  display: grid;
  grid-template-columns: 14px minmax(0, 1fr);
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid var(--surface-border);
}

.timeline__item:last-child {
  border-bottom: 0;
}

.timeline__item p,
.timeline__item small {
  margin: 3px 0 0;
  color: var(--text-secondary);
}

.timeline__dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  background: var(--text-secondary);
}

.timeline__dot.running,
.timeline__dot.active,
.activity-row__state.running {
  background: #2fb07a;
}

.timeline__dot.pending,
.timeline__dot.waiting {
  background: #e0a326;
}

.timeline__dot.failed,
.timeline__dot.error {
  background: #df5b5b;
}

.agents-layout {
  display: grid;
  grid-template-columns: 292px minmax(0, 1fr);
  gap: 14px;
}

.list-panel {
  align-self: start;
  padding: 14px;
  border-radius: 18px;
}

.list-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 4px 12px;
}

.list-panel__header h2 {
  margin: 0;
}

.agent-row {
  margin-bottom: 8px;
  background: transparent;
}

.agent-row.active {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-bg));
}

.agent-row > span:nth-child(2) {
  display: grid;
  flex: 1;
  min-width: 0;
}

.detail-panel {
  min-width: 0;
  padding: 22px;
  border-radius: 18px;
}

.detail-panel__hero,
.detail-panel__identity,
.detail-panel__actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.detail-panel__hero {
  justify-content: space-between;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--surface-border);
}

.detail-panel__avatar {
  width: 58px;
  height: 58px;
  font-size: 22px;
  border-radius: 18px;
}

.detail-panel__identity p {
  margin: 5px 0 8px;
  color: var(--text-secondary);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chips span {
  padding: 3px 8px;
  border: 1px solid var(--surface-border);
  border-radius: 999px;
  color: var(--text-secondary);
  font-size: 11px;
}

.detail-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(320px, 0.8fr);
  gap: 14px;
  margin-top: 14px;
}

.detail-card {
  min-width: 0;
  padding: 18px;
  border-radius: 16px;
}

.detail-card h4 {
  margin: 20px 0 10px;
}

.detail-card pre {
  max-height: 420px;
  overflow: auto;
  padding: 14px;
  border-radius: 12px;
  color: var(--text-primary);
  font: 12px/1.65 ui-monospace, SFMono-Regular, monospace;
  background: var(--surface-soft);
}

.history-row,
.simple-row,
.context-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 0;
  border-bottom: 1px solid var(--surface-border);
}

.history-row p,
.context-row p {
  margin: 3px 0 0;
  color: var(--text-secondary);
}

.history-row__meta {
  display: grid;
  justify-items: end;
  gap: 5px;
}

.teams-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  gap: 14px;
}

.team-grid,
.runtime-grid,
.market-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.human-members {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px;
  margin-bottom: 14px;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: var(--surface-soft);
}

.human-members h3,
.human-members p {
  margin: 0;
}

.human-members p {
  margin-top: 3px;
  color: var(--text-secondary);
}

.team-card {
  display: flex;
  gap: 13px;
  padding: 16px;
  border-radius: 16px;
  cursor: pointer;
}

.inbox-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 14px;
}

.governance-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 14px;
}

.governance-budget {
  grid-column: 1 / -1;
}

.governance-a2a {
  grid-column: 1 / -1;
}

.governance-eval {
  grid-column: 1 / -1;
}

.policy-form {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.credential-form {
  display: grid;
  gap: 8px;
  margin-bottom: 12px;
}

.credential-ref {
  display: block;
  margin-top: 3px;
  color: var(--text-secondary);
  font: 10px ui-monospace, SFMono-Regular, monospace;
}

.inbox-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 15px;
  margin-bottom: 10px;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: var(--surface-soft);
}

.inbox-card h3 {
  margin: 7px 0 4px;
}

.inbox-card p,
.inbox-card small {
  margin: 0;
  color: var(--text-secondary);
}

.risk {
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 10px;
  text-transform: uppercase;
  background: var(--surface-bg);
}

.risk.high,
.risk.critical {
  color: #c94848;
  background: #fff0f0;
}

.error-text {
  max-width: 560px;
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: #b54a4a !important;
}

.runtime-card,
.market-card {
  padding: 17px;
  border-radius: 16px;
}

.runtime-card__head,
.market-card__top,
.market-card__actions {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.runtime-provider {
  color: var(--accent);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.runtime-card dl {
  display: grid;
  gap: 7px;
  margin: 16px 0;
}

.runtime-card dl div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.runtime-card dt,
.runtime-card dd {
  margin: 0;
}

.runtime-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.terminal-input :deep(textarea),
.terminal-result pre {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.terminal-result {
  overflow: hidden;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: var(--surface-bg);
}

.terminal-result__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 12px;
  border-bottom: 1px solid var(--surface-border);
  color: var(--text-secondary);
  font-size: 12px;
}

.terminal-result pre {
  min-height: 180px;
  max-height: 420px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.6;
}

.market {
  display: grid;
  gap: 14px;
}

.market__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 12px;
  border: 1px solid var(--surface-border);
  border-radius: 16px;
  background: var(--surface-bg);
}

.category-strip {
  display: flex;
  gap: 6px;
  overflow-x: auto;
}

.category-strip button {
  flex: 0 0 auto;
  padding: 7px 11px;
  border: 0;
  border-radius: 999px;
  color: var(--text-secondary);
  background: var(--surface-soft);
  cursor: pointer;
}

.category-strip button.active {
  color: white;
  background: var(--accent);
}

.market-card__top {
  justify-content: flex-start;
}

.market-card__top > :last-child {
  margin-left: auto;
}

.market-card__top small {
  display: block;
  margin-top: 3px;
  color: var(--text-secondary);
  font: 10px ui-monospace, SFMono-Regular, monospace;
}

.market-card > p {
  min-height: 42px;
  margin-top: 14px;
}

.market-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 12px 0;
  color: var(--text-secondary);
  font-size: 11px;
}

.market-card__actions {
  justify-content: flex-end;
}

.activity-table {
  display: grid;
}

.activity-row {
  display: grid;
  grid-template-columns: 10px minmax(260px, 1fr) 150px 100px 170px;
  align-items: center;
  gap: 12px;
  padding: 12px 4px;
  border-bottom: 1px solid var(--surface-border);
}

.activity-row__state {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-secondary);
}

.activity-row p {
  margin: 3px 0 0;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.form-grid > * {
  min-width: 0;
}

.host-preview {
  display: grid;
  place-items: center;
  max-height: 76vh;
  overflow: auto;
  border-radius: 12px;
  background: #101114;
}

.host-preview img {
  display: block;
  max-width: 100%;
  height: auto;
  cursor: crosshair;
}

.host-keyboard {
  display: flex;
  gap: 10px;
  margin-top: 12px;
}

.config-editor :deep(textarea),
.diff-view {
  font: 12px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.diff-view {
  max-height: 68vh;
  overflow: auto;
  white-space: pre-wrap;
}

.inline-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 1100px) {
  .metrics,
  .runtime-grid,
  .market-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .overview-grid,
  .teams-layout,
  .inbox-layout,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .agent-os {
    padding: 18px 12px;
  }

  .agent-os__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .metrics,
  .agent-cards,
  .runtime-grid,
  .market-grid,
  .team-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .agents-layout {
    grid-template-columns: 1fr;
  }

  .detail-panel__hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .detail-panel__actions,
  .market__toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .activity-row {
    grid-template-columns: 10px minmax(0, 1fr);
  }

  .activity-row > span:not(.activity-row__state) {
    grid-column: 2;
  }
}
</style>
