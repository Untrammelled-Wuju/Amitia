<template>
  <main class="builder-page">
    <header class="builder-header">
      <div class="title-area">
        <div class="title-row">
          <nav aria-label="工作流页面层级">
            <ol class="builder-breadcrumb-list">
              <li><router-link class="breadcrumb-link" to="/creative-workshop">创意工坊</router-link></li>
              <li class="breadcrumb-separator" aria-hidden="true">/</li>
              <li><router-link class="breadcrumb-link" to="/creative-workshop/workflows">工作流</router-link></li>
              <li class="breadcrumb-separator" aria-hidden="true">/</li>
              <li class="current-title" aria-current="page">
                <el-input v-model="workflow.name" class="name-input" maxlength="80" aria-label="工作流名称" />
              </li>
            </ol>
          </nav>
          <div class="workflow-status">
            <span v-if="dirty" class="dirty-dot">未保存</span>
            <span v-else class="saved-state">已保存</span>
            <span class="target-badge">{{ workflowTargetLabel }}</span>
          </div>
        </div>
        <el-input v-model="workflow.description" class="description-input" placeholder="工作流描述" maxlength="240" />
      </div>
      <div class="header-actions">
        <el-radio-group v-model="editorMode" size="small" class="editor-mode-switch">
          <el-radio-button value="simple">简单模式</el-radio-button>
          <el-radio-button value="advanced">高级模式</el-radio-button>
        </el-radio-group>
        <el-button :disabled="historyIndex <= 0" @click="undo"><el-icon><RefreshLeft /></el-icon></el-button>
        <el-button :disabled="historyIndex >= history.length - 1" @click="redo"><el-icon><RefreshRight /></el-icon></el-button>
        <el-button :disabled="workflowTarget.location === 'device'" :loading="aiWorking" @click="inspectorTab = 'ai'">AI Copilot</el-button>
        <el-button @click="validateCurrent">校验</el-button>
        <el-button @click="runPreflight">预检</el-button>
        <el-button :loading="saving" type="primary" @click="save">保存</el-button>
        <el-button :loading="running" type="success" @click="openRunDialog"><el-icon><VideoPlay /></el-icon>运行</el-button>
      </div>
    </header>

    <section class="builder-layout">
      <aside class="palette-panel">
        <div class="palette-top">
          <div class="panel-title">节点</div>
          <p class="panel-tip">拖到画布或点击添加。节点直接映射到 Kernel Handler。</p>
        </div>
        <div class="palette-items">
          <button
            v-for="item in nodePalette"
            :key="item.type"
            type="button"
            class="palette-item"
            draggable="true"
            @dragstart="onPaletteDragStart(item.type, $event)"
            @click="addNode(item.type)"
          >
            <span class="node-type-icon">{{ item.short }}</span>
            <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
          </button>
        </div>
        <div class="palette-bottom">
          <div class="palette-divider"></div>
          <div class="panel-title small">画布</div>
          <div class="canvas-tools vertical">
            <el-button @click="zoomBy(0.1)">放大</el-button>
            <el-button @click="zoomBy(-0.1)">缩小</el-button>
            <el-button @click="fitView">适配</el-button>
            <el-button @click="autoLayout">自动布局</el-button>
          </div>
        </div>
      </aside>

      <section
        ref="canvasRef"
        class="workflow-canvas"
        @dragover.prevent
        @drop="onCanvasDrop"
        @pointerdown="onCanvasPointerDown"
        @wheel.prevent="onWheel"
      >
        <div class="canvas-grid" :style="gridStyle"></div>
        <div class="graph-transform" :style="graphTransform">
          <svg class="edge-layer" viewBox="0 0 4000 2600" aria-label="工作流连线">
            <g v-for="edge in workflow.edges" :key="edge.id" class="edge-group" @click.stop="selectEdge(edge.id)">
              <path :d="edgePath(edge)" class="edge-hit" />
              <path :d="edgePath(edge)" class="edge-line" :class="{ selected: selectedEdgeId === edge.id }" />
              <text v-if="edge.label" :x="edgeLabelPoint(edge).x" :y="edgeLabelPoint(edge).y" class="edge-label">{{ edge.label }}</text>
            </g>
            <path v-if="connectingFrom" :d="previewPath" class="edge-line preview" />
          </svg>

          <article
            v-for="node in workflow.nodes"
            :key="node.id"
            class="workflow-node"
            :class="[nodeStatusClass(node.id), { selected: selectedNodeId === node.id }]"
            :style="nodeStyle(node)"
            @pointerdown.stop="selectNode(node.id)"
          >
            <button class="input-handle" type="button" aria-label="连接到此节点" @pointerup.stop="finishConnect(node.id)" @click.stop="finishConnect(node.id)"></button>
            <div class="node-header" @pointerdown.stop="startNodeDrag(node, $event)">
              <span class="node-badge">{{ paletteByType(node.type)?.short || "N" }}</span>
              <div class="node-title"><strong>{{ node.label || paletteByType(node.type)?.label || node.type }}</strong><small>{{ node.type }}</small></div>
              <el-icon class="node-menu" @click.stop="removeNode(node.id)"><Close /></el-icon>
            </div>
            <div class="node-body">
              <span v-if="node.targetId" class="target-text">{{ node.targetId }}</span>
              <span v-else class="target-text muted">未配置目标</span>
              <span v-if="stepStatus(node.id)" class="status-pill">{{ stepStatus(node.id) }}</span>
            </div>
            <button class="output-handle" type="button" aria-label="从此节点连接" @pointerdown.stop="startConnect(node.id, $event)"></button>
          </article>
        </div>

        <div class="zoom-indicator">{{ Math.round(zoom * 100) }}%</div>
        <div class="minimap" aria-label="工作流缩略图">
          <svg viewBox="0 0 4000 2600">
            <rect v-for="node in workflow.nodes" :key="node.id" :x="node.position?.x || 0" :y="node.position?.y || 0" width="180" height="84" rx="8" class="mini-node" />
          </svg>
        </div>
        <div v-if="connectingFrom" class="connect-hint">正在从 {{ connectingFrom }} 连线，松开到目标节点左侧端口</div>
      </section>

      <aside class="inspector-panel">
        <el-tabs v-model="inspectorTab" stretch @tab-change="onInspectorTabChanged">
          <el-tab-pane label="属性" name="properties">
            <div v-if="selectedNode" class="inspector-content">
              <div class="panel-row"><div class="panel-title">节点属性</div><el-tag size="small" effect="plain">{{ editorMode === 'simple' ? '简单' : '高级' }}</el-tag></div>
              <p v-if="editorMode === 'simple'" class="panel-tip">简单模式只展示动作、参数、执行设备和失败策略；Runtime、Retry、When/Postcondition、Capability 与补偿保持原定义但隐藏。</p>
              <label>名称<el-input v-model="selectedNode.label" @change="markDirty" /></label>
              <label>类型
                <el-select v-model="selectedNode.type" @change="onNodeTypeChanged">
                  <el-option v-for="item in nodePalette" :key="item.type" :label="item.label" :value="item.type" />
                </el-select>
              </label>
              <label v-if="selectedNode.type === 'nested_workflow'">子工作流
                <el-select v-model="selectedNode.targetId" filterable placeholder="选择我的另一个工作流" @change="markDirty">
                  <el-option v-for="item in nestedWorkflowCandidates" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
              </label>
              <template v-else-if="selectedNode.type === 'tool'">
                <label>Action / Tool
                  <el-select v-model="selectedNode.targetId" filterable clearable placeholder="搜索 Android Action / Tool" @change="onToolTargetChanged">
                    <el-option-group v-for="group in toolCatalogGroups" :key="group.label" :label="group.label">
                      <el-option v-for="item in group.items" :key="item.id" :value="item.id" :label="`${item.name} · ${item.id}`" />
                    </el-option-group>
                  </el-select>
                </label>
                <div v-if="selectedToolCatalogItem" class="tool-catalog-card">
                  <div class="tool-catalog-head"><strong>{{ selectedToolCatalogItem.name }}</strong><el-tag size="small" :type="toolRiskTagType(selectedToolCatalogItem.riskLevel)">{{ selectedToolCatalogItem.riskLevel || 'unknown' }}</el-tag></div>
                  <p>{{ selectedToolCatalogItem.description || selectedToolCatalogItem.id }}</p>
                  <div class="tool-catalog-meta">
                    <span>副作用：{{ selectedToolCatalogItem.sideEffect || 'none' }}</span>
                    <span>可重试：{{ selectedToolCatalogItem.retryable ? '是' : '否' }}</span>
                    <span v-if="selectedToolCatalogItem.timeoutMs">默认超时：{{ selectedToolCatalogItem.timeoutMs }}ms</span>
                  </div>
                  <div v-if="selectedToolCatalogItem.permissions?.length" class="tool-permissions">
                    <el-tag v-for="permission in selectedToolCatalogItem.permissions" :key="permission.capability" size="small" effect="plain">{{ permission.capability }}</el-tag>
                  </div>
                  <div v-if="selectedToolInputFields.length" class="tool-schema-form">
                    <div class="tool-schema-title">参数配置</div>
                    <label v-for="field in selectedToolInputFields" :key="field.name" class="tool-schema-field">
                      <span>{{ field.name }}<em v-if="field.required">必填</em><small>{{ field.type }}</small></span>
                      <el-input
                        :model-value="toolInputFieldText(field.name)"
                        :placeholder="field.description || `输入 ${field.type}；也可使用 input./steps./runtime. 引用`"
                        @change="value => setToolInputField(field, value)"
                      />
                    </label>
                  </div>
                  <details v-if="editorMode === 'advanced' && selectedToolCatalogItem.inputSchema" class="schema-details"><summary>输入 Schema</summary><pre>{{ pretty(selectedToolCatalogItem.inputSchema) }}</pre></details>
                </div>
              </template>
              <label v-else-if="needsTarget(selectedNode.type)">目标 ID<el-input v-model="selectedNode.targetId" placeholder="MCP / Task / Runtime ID" @change="markDirty" /></label>
              <template v-if="editorMode === 'advanced' && needsRuntime(selectedNode.type)">
                <label>Runtime Type<el-input v-model="selectedNode.runtime.runtimeType" @change="markDirty" /></label>
                <label>Runtime ID<el-input v-model="selectedNode.runtime.runtimeId" placeholder="例如 MCP Server / Task Definition / Service ID" @change="markDirty" /></label>
                <label>Handler Name<el-input v-model="selectedNode.runtime.handlerName" placeholder="例如 MCP Tool / JS Handler / WASM Export" @change="markDirty" /></label>
                <label>Runtime Metadata JSON<el-input v-model="nodeRuntimeMetadataEditor" type="textarea" :rows="5" placeholder='例如 {"extensionId":"...","moduleId":"..."}' @change="applyNodeEditors" /></label>
              </template>
              <div v-if="isCloudWorkflow && supportsPlacement(selectedNode.type)" class="reliability-card">
                <div class="panel-title small">执行位置</div>
                <label>Placement
                  <el-select v-model="executionPlacement" @change="onExecutionTargetChanged">
                    <el-option label="云端" value="cloud" />
                    <el-option v-if="selectedNode.type !== 'nested_workflow'" label="自动选择设备" value="auto" />
                    <el-option label="指定设备" value="device" />
                  </el-select>
                </label>
                <label v-if="executionPlacement === 'device'">设备
                  <el-select v-model="executionDeviceId" filterable placeholder="选择设备" @change="onExecutionTargetChanged">
                    <el-option v-for="device in workflowDevices" :key="device.deviceId" :value="device.deviceId" :label="`${device.label || device.deviceId}${device.online ? ' · 在线' : ' · 离线'}`" />
                  </el-select>
                </label>
                <label v-if="executionPlacement === 'device' || executionPlacement === 'auto'">设备离线时
                  <el-select v-model="executionOfflinePolicy" @change="markDirty">
                    <el-option label="失败" value="fail" />
                    <el-option label="等待设备上线" value="wait" />
                  </el-select>
                </label>
                <p class="panel-tip">只选择设备即可。Runtime Session / Provider Instance 由 Cloud Core 根据当前在线能力自动解析。</p>
              </div>
              <label v-if="selectedNode.type === 'wait'">等待毫秒<el-input-number v-model="waitDuration" :min="0" :max="86400000" controls-position="right" @change="applyWaitDuration" /></label>
              <div v-if="editorMode === 'advanced'" class="reliability-card">
                <div class="panel-title small">执行可靠性</div>
                <label>节点超时（毫秒，0=继承工作流上限）<el-input-number v-model="nodeTimeoutMs" :min="0" :max="maxNodeTimeoutMs" :step="1000" controls-position="right" /></label>
                <label class="inline-switch">自定义重试<el-switch v-model="retryEnabled" /></label>
                <template v-if="retryEnabled">
                  <label>最大尝试次数<el-input-number v-model="retryMaxAttempts" :min="1" :max="10" controls-position="right" /></label>
                  <label>首次退避（毫秒，0=默认 200）<el-input-number v-model="retryInitialBackoffMs" :min="0" :max="600000" :step="100" controls-position="right" /></label>
                  <label>最大退避（毫秒，0=默认 30000）<el-input-number v-model="retryMaxBackoffMs" :min="0" :max="600000" :step="500" controls-position="right" /></label>
                  <label>退避倍率<el-input-number v-model="retryMultiplier" :min="1.1" :max="10" :step="0.1" :precision="1" controls-position="right" /></label>
                  <label>随机抖动（0~1）<el-input-number v-model="retryJitter" :min="0" :max="1" :step="0.05" :precision="2" controls-position="right" /></label>
                </template>
                <p class="panel-tip">重试只作用于当前节点；节点超时不能超过工作流的最大单步时长。未开启自定义重试时默认只尝试 1 次。</p>
              </div>
              <div v-if="editorMode === 'advanced'" class="reliability-card">
                <div class="panel-title small">Saga 补偿</div>
                <label class="inline-switch">失败时执行补偿<el-switch v-model="compensationEnabled" /></label>
                <template v-if="compensationEnabled && selectedNode.compensation">
                  <label>补偿类型
                    <el-select v-model="selectedNode.compensation.type" @change="markDirty">
                      <el-option label="Tool" value="tool" /><el-option label="Task" value="task" /><el-option label="MCP" value="mcp" /><el-option label="Trusted Service" value="trusted_service" />
                    </el-select>
                  </label>
                  <label>补偿目标 / Action<el-input v-model="selectedNode.compensation.targetId" placeholder="例如 payment.refund" @change="markDirty" /></label>
                  <label>补偿输入 JSON<el-input v-model="compensationInputEditor" type="textarea" :rows="5" placeholder='例如 {"transactionId":"${nodes.charge.output.transactionId}"}' @change="applyNodeEditors" /></label>
                  <label>补偿超时（毫秒）<el-input-number v-model="selectedNode.compensation.timeoutMs" :min="0" :max="maxNodeTimeoutMs" :step="1000" controls-position="right" @change="markDirty" /></label>
                  <p class="panel-tip">补偿按成功节点逆序执行，并复用 Retry / Checkpoint / Idempotency / Lease-Fencing。补偿失败会进入 durable recovery，而不是丢弃。</p>
                </template>
              </div>
              <label v-if="editorMode === 'advanced' || selectedNode.type !== 'tool' || !selectedToolInputFields.length">{{ editorMode === 'advanced' ? '输入 JSON' : '参数' }}<el-input v-model="nodeInputEditor" type="textarea" :rows="editorMode === 'advanced' ? 7 : 4" @change="applyNodeEditors" /></label>
              <p v-if="selectedNode.type === 'logic'" class="panel-tip">Logic 支持 eq/ne/gt/gte/lt/lte、and/or/not/xor、contains、in、matches、exists、empty、truthy。结果固定输出为 { result: boolean }。</p>
              <p v-if="selectedNode.type === 'extract'" class="panel-tip">Extract 支持 a.b、items[0].name、items[*].id；可用 path / paths / aliases / required / default / unwrap，无需 JavaScript。</p>
              <p v-if="selectedNode.type === 'transform'" class="panel-tip">Transform 支持 pick/omit/rename/set/merge/flatten、array_map/filter/take/sort、json_parse/stringify、unique/join/split/count/coalesce。</p>
              <div v-if="editorMode === 'simple'" class="simple-expression-builder">
                <div class="panel-row"><div class="panel-title">条件</div><el-switch v-model="simpleWhenEnabled" @change="onSimpleWhenEnabledChanged" /></div>
                <template v-if="simpleWhenEnabled">
                  <el-alert v-if="!simpleWhenCompatible" type="warning" :closable="false" title="当前 When 使用了高级表达式，简单模式不会自动覆盖。" show-icon>
                    <template #default><el-button size="small" @click="replaceAdvancedWhenWithSimple">用简单条件替换</el-button></template>
                  </el-alert>
                  <template v-else>
                    <label v-if="simpleConditions.length > 1">条件关系<el-radio-group v-model="simpleConditionJoin" size="small" @change="applySimpleWhen"><el-radio-button value="and">全部满足 AND</el-radio-button><el-radio-button value="or">任一满足 OR</el-radio-button></el-radio-group></label>
                    <article v-for="(condition,index) in simpleConditions" :key="condition.id" class="simple-condition-row">
                      <div class="simple-condition-head"><strong>条件 {{ index + 1 }}</strong><el-checkbox v-model="condition.not" @change="applySimpleWhen">取反 NOT</el-checkbox><el-button v-if="simpleConditions.length > 1" size="small" text type="danger" @click="removeSimpleCondition(index)">删除</el-button></div>
                      <label>数据来源<el-select v-model="condition.source" @change="applySimpleWhen"><el-option label="工作流输入" value="input" /><el-option label="节点输出" value="node_output" /></el-select></label>
                      <label v-if="condition.source === 'node_output'">节点<el-select v-model="condition.nodeId" filterable @change="applySimpleWhen"><el-option v-for="node in simpleConditionNodeOptions" :key="node.id" :label="node.label || node.id" :value="node.id" /></el-select></label>
                      <label>字段路径<el-input v-model="condition.path" placeholder="例如 enabled / data.status" @change="applySimpleWhen" /></label>
                      <label>判断<el-select v-model="condition.op" @change="applySimpleWhen"><el-option label="等于" value="eq" /><el-option label="不等于" value="ne" /><el-option label="包含" value="contains" /><el-option label="大于" value="gt" /><el-option label="大于等于" value="gte" /><el-option label="小于" value="lt" /><el-option label="小于等于" value="lte" /><el-option label="存在" value="exists" /><el-option label="为空" value="is_null" /></el-select></label>
                      <label v-if="!['exists','is_null'].includes(condition.op)">比较值<el-input v-model="condition.value" placeholder='可填 true、123、"文本" 或普通文本' @change="applySimpleWhen" /></label>
                    </article>
                    <el-button size="small" plain @click="addSimpleCondition">增加条件</el-button>
                  </template>
                </template>
              </div>
              <template v-if="editorMode === 'advanced'">
                <label>前置条件 / When JSON<el-input v-model="nodeWhenEditor" type="textarea" :rows="5" placeholder='例如 {"op":"eq","left":{"ref":{"source":"input","path":["enabled"]}},"right":{"value":true}}' @change="applyNodeEditors" /></label>
                <label>后置条件 / Postcondition JSON<el-input v-model="nodePostconditionEditor" type="textarea" :rows="5" placeholder='例如 {"op":"eq","left":{"ref":{"source":"node_output","nodeId":"当前节点ID","path":["ok"]}},"right":{"value":true}}' @change="applyNodeEditors" /></label>
                <p class="panel-tip">Postcondition 在节点返回后、成功提交前验证；失败会进入该节点现有 Retry / OnError / Saga 语义。</p>
                <label>权限（逗号分隔）<el-input v-model="nodePermissionsEditor" @change="applyNodeEditors" /></label>
                <label>Capability Requirement（逗号分隔）<el-input v-model="nodeCapabilitiesEditor" placeholder="例如 android.accessibility, android.visual.ocr" @change="applyNodeEditors" /></label>
                <p class="panel-tip">额外 Capability 会进入 Preflight 和实际路由 Gate；设备执行时必须由同一目标设备提供。</p>
              </template>
              <label>失败策略
                <el-select v-model="selectedNode.step.onError.mode" @change="markDirty">
                  <el-option label="失败即终止" value="fail" />
                  <el-option label="继续执行" value="continue" />
                  <el-option label="使用默认值" value="use_default" />
                </el-select>
              </label>
              <label v-if="selectedNode.step.onError.mode === 'use_default'">失败默认值 JSON<el-input v-model="nodeErrorDefaultEditor" type="textarea" :rows="5" placeholder='例如 {"ok":false}' @change="applyNodeEditors" /></label>
              <el-button type="danger" plain @click="removeNode(selectedNode.id)">删除节点</el-button>
            </div>
            <div v-else-if="selectedEdge" class="inspector-content">
              <div class="panel-title">连线属性</div>
              <label>标签<el-input v-model="selectedEdge.label" @change="markDirty" /></label>
              <div class="edge-summary">{{ selectedEdge.source }} → {{ selectedEdge.target }}</div>
              <template v-if="editorMode === 'advanced'">
                <label>条件 JSON<el-input v-model="edgeConditionEditor" type="textarea" :rows="7" placeholder="为空表示无条件依赖" @change="applyEdgeEditor" /></label>
                <p class="panel-tip">条件会编译为目标节点的 When 表达式；多个入边条件按 AND 合并。</p>
              </template>
              <p v-else class="panel-tip">简单模式隐藏连线表达式；切换高级模式可编辑 When AST。</p>
              <el-button type="danger" plain @click="removeEdge(selectedEdge.id)">删除连线</el-button>
            </div>
            <div v-else class="empty-inspector">选择节点或连线后编辑属性。</div>
          </el-tab-pane>

          <el-tab-pane label="触发器" name="triggers">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">Trigger Center</div><el-button size="small" @click="addTrigger">新增</el-button></div>
              <article v-for="(trigger, index) in workflow.triggers" :key="trigger.id" class="trigger-card">
                <div class="trigger-head"><strong>{{ trigger.id }}</strong><el-switch v-model="trigger.enabled" @change="markDirty" /></div>
                <label>类型
                  <el-select v-model="trigger.type" @change="normalizeTrigger(trigger)">
                    <el-option label="手动" value="manual" />
                    <el-option label="系统 / 设备事件" value="event" />
                    <el-option label="Cron" value="cron" />
                    <el-option label="间隔" value="interval" />
                    <el-option label="单次" value="one_shot" />
                  </el-select>
                </label>
                <template v-if="isEventTrigger(trigger.type)">
                  <label>触发方式
                    <el-select :model-value="eventTriggerPreset(trigger)" @change="applyEventTriggerPreset(trigger, $event)">
                      <el-option v-if="editorMode === 'advanced'" label="高级事件" value="advanced" />
                      <el-option label="Android Intent" value="android_intent" :disabled="!canUseDeviceTrigger('android_intent')" />
                      <el-option label="Tasker" value="tasker" :disabled="!canUseDeviceTrigger('tasker')" />
                      <el-option label="Voice Wake" value="voice_wake" :disabled="!canUseDeviceTrigger('voice_wake')" />
                      <el-option label="Voice Phrase" value="voice_phrase" :disabled="!canUseDeviceTrigger('voice_phrase')" />
                      <el-option label="App Launch / Foreground" value="app_foreground" :disabled="!canUseDeviceTrigger('app_foreground')" />
                      <el-option label="通知" value="notification" :disabled="!canUseDeviceTrigger('notification')" />
                      <el-option label="电量 / 充电" value="battery" :disabled="!canUseDeviceTrigger('battery')" />
                      <el-option label="网络" value="network" :disabled="!canUseDeviceTrigger('network')" />
                      <el-option label="应用安装 / 更新 / 卸载" value="package_event" :disabled="!canUseDeviceTrigger('package_event')" />
                      <el-option label="Bluetooth / BLE" value="bluetooth" :disabled="!canUseDeviceTrigger('bluetooth')" />
                      <el-option label="Geofence" value="geofence" :disabled="!canUseDeviceTrigger('geofence')" />
                      <el-option label="Android 系统事件" value="system_event" :disabled="!canUseDeviceTrigger('system_event')" />
                    </el-select>
                  </label>
                  <label v-if="eventTriggerPreset(trigger) === 'advanced'">事件类型<el-input v-model="trigger.eventType" placeholder="例如 message.created / device.wifi.changed" @change="markDirty" /></label>
                  <div v-if="eventTriggerPreset(trigger) !== 'advanced'" class="trigger-capability">
                    <el-tag size="small" :type="triggerCapabilityTagType(trigger)">{{ triggerCapabilityLabel(trigger) }}</el-tag>
                    <span>{{ triggerCapabilityReason(trigger) }}</span>
                  </div>
                  <template v-if="eventTriggerPreset(trigger) === 'android_intent'">
                    <label>Intent Action<el-select v-model="trigger.config.actions" multiple filterable allow-create default-first-option placeholder="com.example.EVENT" @change="markDirty" /></label>
                    <label>Categories<el-select v-model="trigger.config.categories" multiple filterable allow-create default-first-option placeholder="可选" @change="markDirty" /></label>
                    <label>Data Schemes<el-select v-model="trigger.config.dataSchemes" multiple filterable allow-create default-first-option placeholder="可选，例如 https" @change="markDirty" /></label>
                    <label>MIME Types<el-select v-model="trigger.config.mimeTypes" multiple filterable allow-create default-first-option placeholder="可选，例如 text/*" @change="markDirty" /></label>
                    <label>去重窗口 ms<el-input-number v-model="trigger.config.dedupWindowMs" :min="0" :max="600000" controls-position="right" @change="markDirty" /></label>
                    <p class="panel-tip">第三方 App 需向 Amitia Receiver 发送显式 Broadcast；Release Component=com.amitia.amitia_app/com.amitia.amitia_app.workflow.WorkflowIntentReceiver，Debug/变体构建请使用实际 applicationId。Action 仍使用上方配置值。</p>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'tasker'">
                    <label>事件名<el-input v-model="trigger.config.eventName" placeholder="home_arrived" @change="markDirty" /></label>
                    <label>Secret Ref
                      <div class="tasker-secret-row">
                        <el-input v-model="trigger.config.secretRef" readonly placeholder="点击生成 Secret" />
                        <el-button size="small" :loading="taskerSecretBusy === trigger.id" :disabled="!canUseDeviceTrigger('tasker')" @click="generateTaskerSecret(trigger)">生成</el-button>
                      </div>
                    </label>
                    <label>允许变量<el-select v-model="trigger.config.allowedVariables" multiple filterable allow-create default-first-option placeholder="battery / location" @change="markDirty" /></label>
                    <p class="panel-tip">Tasker 使用显式 Broadcast：Action=com.amitia.workflow.TASKER；Release Component=com.amitia.amitia_app/com.amitia.amitia_app.workflow.WorkflowIntentReceiver，Debug/变体构建请使用实际 applicationId。Secret 只在生成时显示一次，Definition 仅保存 Secret Ref。</p>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'voice_wake'">
                    <label>Wake Config
                      <div class="tasker-secret-row">
                        <el-select v-model="trigger.config.wakeConfigId" filterable placeholder="选择启用的 Wake Config" @change="markDirty">
                          <el-option v-for="item in triggerWakeConfigs" :key="item.id" :label="`${item.name} · ${item.backend}`" :value="item.id" />
                        </el-select>
                        <el-button size="small" :disabled="!canUseDeviceTrigger('voice_wake')" @click="createWakeConfig(trigger)">创建</el-button>
                      </div>
                    </label>
                    <p class="panel-tip">默认使用本地 KWS：无需 API Key，模型在设备 Runtime 内识别；创建 Wake Config 时也可显式选择云 ASR。VAD 只做语音切段，只有识别到配置短语才触发，响声本身不会触发工作流。</p>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'voice_phrase'">
                    <label>短语<el-select v-model="trigger.config.phrases" multiple filterable allow-create default-first-option placeholder="开始回家模式" @change="markDirty" /></label>
                    <label>匹配模式<el-select v-model="trigger.config.matchMode" @change="markDirty"><el-option label="标准化匹配" value="normalized" /><el-option label="精确匹配" value="exact" /></el-select></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'app_foreground'">
                    <label>Package
                      <el-select v-model="trigger.config.packages" multiple filterable allow-create default-first-option placeholder="选择设备应用或输入 package" @change="markDirty">
                        <el-option v-for="app in triggerAppCatalog" :key="app.packageName" :label="app.label ? `${app.label} · ${app.packageName}` : app.packageName" :value="app.packageName" />
                      </el-select>
                    </label>
                    <label>冷却时间 ms<el-input-number v-model="trigger.config.cooldownMs" :min="0" :max="3600000" controls-position="right" @change="markDirty" /></label>
                    <p class="panel-tip">仅 package 从其他 App 切换为目标 App 时触发；同包 Activity 切换不会重复执行。</p>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'notification'">
                    <label>通知事件<el-select v-model="trigger.eventType" @change="markDirty"><el-option label="收到通知" value="device.notification.posted" /><el-option label="通知移除" value="device.notification.removed" /></el-select></label>
                    <label>Package<el-select v-model="trigger.config.packages" multiple filterable allow-create default-first-option placeholder="留空表示全部应用" @change="markDirty"><el-option v-for="app in triggerAppCatalog" :key="`n-${app.packageName}`" :label="app.label ? `${app.label} · ${app.packageName}` : app.packageName" :value="app.packageName" /></el-select></label>
                    <label>标题包含<el-input v-model="trigger.config.titleContains" maxlength="512" clearable @change="markDirty" /></label>
                    <label>正文包含<el-input v-model="trigger.config.textContains" maxlength="1024" clearable @change="markDirty" /></label>
                    <label>Channel ID<el-select v-model="trigger.config.channelIds" multiple filterable allow-create default-first-option placeholder="可选" @change="markDirty" /></label>
                    <label>Category<el-select v-model="trigger.config.categories" multiple filterable allow-create default-first-option placeholder="可选" @change="markDirty" /></label>
                    <label>Ongoing<el-select v-model="trigger.config.ongoing" clearable placeholder="不限" @clear="clearTriggerConfigKey(trigger, 'ongoing')" @change="markDirty"><el-option label="是" :value="true" /><el-option label="否" :value="false" /></el-select></label>
                    <label>可清除<el-select v-model="trigger.config.clearable" clearable placeholder="不限" @clear="clearTriggerConfigKey(trigger, 'clearable')" @change="markDirty"><el-option label="是" :value="true" /><el-option label="否" :value="false" /></el-select></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'battery'">
                    <label>最低电量 %<el-input-number v-model="trigger.config.minPercent" :min="0" :max="100" controls-position="right" @change="markDirty" /></label>
                    <label>最高电量 %<el-input-number v-model="trigger.config.maxPercent" :min="0" :max="100" controls-position="right" @change="markDirty" /></label>
                    <label>充电状态<el-select v-model="trigger.config.charging" clearable placeholder="不限" @clear="clearTriggerConfigKey(trigger, 'charging')" @change="markDirty"><el-option label="正在充电" :value="true" /><el-option label="未充电" :value="false" /></el-select></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'network'">
                    <label>网络事件<el-select v-model="trigger.eventType" @change="markDirty"><el-option label="网络变化" value="device.network.changed" /><el-option label="网络可用" value="device.network.available" /><el-option label="网络丢失" value="device.network.lost" /></el-select></label>
                    <label>传输类型<el-select v-model="trigger.config.transports" multiple placeholder="不限" @change="markDirty"><el-option label="Wi-Fi" value="wifi" /><el-option label="蜂窝" value="cellular" /><el-option label="Ethernet" value="ethernet" /><el-option label="VPN" value="vpn" /><el-option label="Bluetooth" value="bluetooth" /><el-option label="其他" value="other" /></el-select></label>
                    <label>Validated<el-select v-model="trigger.config.validated" clearable placeholder="不限" @clear="clearTriggerConfigKey(trigger, 'validated')" @change="markDirty"><el-option label="是" :value="true" /><el-option label="否" :value="false" /></el-select></label>
                    <label>Metered<el-select v-model="trigger.config.metered" clearable placeholder="不限" @clear="clearTriggerConfigKey(trigger, 'metered')" @change="markDirty"><el-option label="计费网络" :value="true" /><el-option label="非计费网络" :value="false" /></el-select></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'package_event'">
                    <label>应用事件<el-select v-model="trigger.eventType" @change="markDirty"><el-option label="安装" value="device.app.installed" /><el-option label="更新" value="device.app.updated" /><el-option label="卸载" value="device.app.removed" /><el-option label="Amitia 自身更新" value="device.app.self_updated" /></el-select></label>
                    <label>Package<el-select v-model="trigger.config.packages" multiple filterable allow-create default-first-option placeholder="留空表示全部应用" @change="markDirty"><el-option v-for="app in triggerAppCatalog" :key="`p-${app.packageName}`" :label="app.label ? `${app.label} · ${app.packageName}` : app.packageName" :value="app.packageName" /></el-select></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'bluetooth'">
                    <label>Bluetooth 事件<el-select v-model="trigger.eventType" @change="normalizeBluetoothTrigger(trigger)"><el-option label="状态变化" value="device.bluetooth.state_changed" /><el-option label="设备连接" value="device.bluetooth.connected" /><el-option label="设备断开" value="device.bluetooth.disconnected" /><el-option label="BLE Characteristic Changed" value="device.ble.characteristic_changed" /></el-select></label>
                    <template v-if="trigger.eventType === 'device.ble.characteristic_changed'">
                      <label>Session ID<el-input v-model="trigger.config.sessionId" clearable @change="markDirty" /></label>
                      <label>Address<el-input v-model="trigger.config.address" placeholder="AA:BB:CC:DD:EE:FF" clearable @change="markDirty" /></label>
                      <label>Service UUID<el-input v-model="trigger.config.serviceUuid" clearable @change="markDirty" /></label>
                      <label>Characteristic UUID<el-input v-model="trigger.config.characteristicUuid" clearable @change="markDirty" /></label>
                    </template>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'geofence'">
                    <label>Geofence 事件<el-select v-model="trigger.eventType" @change="markDirty"><el-option label="进入" value="device.location.geofence.enter" /><el-option label="离开" value="device.location.geofence.exit" /></el-select></label>
                    <label>Fence ID<el-select v-model="trigger.config.fenceIds" multiple filterable allow-create default-first-option placeholder="留空表示全部已注册围栏" @change="markDirty" /></label>
                  </template>
                  <template v-if="eventTriggerPreset(trigger) === 'system_event'">
                    <label>系统事件<el-select v-model="trigger.eventType" filterable @change="markDirty"><el-option v-for="item in androidSystemEventOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></label>
                  </template>
                </template>
                <template v-if="trigger.type === 'cron'">
                  <template v-if="editorMode === 'simple'">
                    <label>每天执行时间<el-time-picker :model-value="simpleCronTime(trigger)" format="HH:mm" value-format="HH:mm" placeholder="08:00" @update:model-value="applySimpleCronTime(trigger,$event)" /></label>
                    <p class="panel-tip">简单模式按“每天 HH:mm”生成 Cron；高级模式可编辑完整表达式、时区、Misfire 和 DST。</p>
                  </template>
                  <template v-else>
                    <label>Cron 表达式<el-input v-model="trigger.config.cronExpression" placeholder="0 8 * * *" @change="markDirty" /></label>
                    <label>时区<el-input v-model="trigger.config.timezone" placeholder="Asia/Shanghai" @change="markDirty" /></label>
                    <label>漏触发策略
                      <el-select v-model="trigger.config.misfirePolicy" @change="markDirty">
                        <el-option label="补执行一次" value="fire_once" /><el-option label="跳过" value="skip" /><el-option label="有限补跑" value="catch_up_limited" /><el-option label="从现在重新调度" value="reschedule_from_now" />
                      </el-select>
                    </label>
                    <label v-if="trigger.config.misfirePolicy === 'catch_up_limited'">最大补跑次数<el-input-number v-model="trigger.config.maxCatchUp" :min="1" :max="1000" controls-position="right" @change="markDirty" /></label>
                    <label>重叠策略<el-select v-model="trigger.config.overlapPolicy" @change="markDirty"><el-option label="禁止重叠" value="forbid" /><el-option label="允许并行" value="allow" /><el-option label="替换旧运行" value="replace" /><el-option label="只排队一个" value="queue_one" /><el-option label="运行中则跳过" value="skip_if_running" /></el-select></label>
                    <label>DST 春季跳时<el-select v-model="trigger.config.dstSpringPolicy" @change="markDirty"><el-option label="跳过不存在的时间" value="skip" /><el-option label="跳时后补执行一次" value="fire_once_after_gap" /><el-option label="使用下一个有效时间" value="next_valid_time" /></el-select></label>
                    <label>DST 秋季重复时间<el-select v-model="trigger.config.dstFallPolicy" @change="markDirty"><el-option label="第一次执行一次" value="fire_once_first" /><el-option label="第二次执行一次" value="fire_once_second" /><el-option label="两次都执行" value="fire_twice" /></el-select></label>
                  </template>
                </template>
                <template v-if="trigger.type === 'interval'">
                  <label>间隔秒数<el-input-number v-model="trigger.config.intervalSeconds" :min="1" :max="31536000" controls-position="right" @change="markDirty" /></label>
                </template>
                <template v-if="trigger.type === 'one_shot'">
                  <label>执行时间（RFC3339）<el-input v-model="trigger.config.runAt" placeholder="2026-09-01T08:00:00+08:00" @change="markDirty" /></label>
                </template>
                <template v-if="trigger.type === 'interval' || trigger.type === 'one_shot'">
                  <label>漏触发策略
                    <el-select v-model="trigger.config.misfirePolicy" @change="markDirty">
                      <el-option label="补执行一次" value="fire_once" />
                      <el-option label="跳过" value="skip" />
                      <el-option label="有限补跑" value="catch_up_limited" />
                      <el-option label="从现在重新调度" value="reschedule_from_now" />
                    </el-select>
                  </label>
                  <label v-if="trigger.config.misfirePolicy === 'catch_up_limited'">最大补跑次数
                    <el-input-number v-model="trigger.config.maxCatchUp" :min="1" :max="1000" controls-position="right" @change="markDirty" />
                  </label>
                  <label>重叠策略
                    <el-select v-model="trigger.config.overlapPolicy" @change="markDirty">
                      <el-option label="禁止重叠" value="forbid" />
                      <el-option label="允许并行" value="allow" />
                      <el-option label="替换旧运行" value="replace" />
                      <el-option label="只排队一个" value="queue_one" />
                      <el-option label="运行中则跳过" value="skip_if_running" />
                    </el-select>
                  </label>
                </template>
                <el-button v-if="workflow.triggers.length > 1" text type="danger" @click="removeTrigger(index)">移除</el-button>
              </article>
            </div>
          </el-tab-pane>

          <el-tab-pane label="映射" name="mapping">
            <div v-if="selectedNode" class="inspector-content">
              <div class="panel-title">可视化数据映射</div>
              <p class="panel-tip">把工作流输入或其他节点输出绑定到当前节点输入。跨节点绑定会自动补齐 DAG 依赖，并在保存前校验循环。</p>
              <label>目标输入字段
                <el-select v-model="mappingTargetPath" filterable allow-create default-first-option placeholder="选择或输入字段路径">
                  <el-option v-for="path in mappingTargetFields" :key="path" :label="path" :value="path" />
                </el-select>
              </label>
              <label>数据来源
                <el-select v-model="mappingSourceRef" filterable placeholder="选择上游数据">
                  <el-option-group v-for="group in mappingSourceGroups" :key="group.label" :label="group.label">
                    <el-option v-for="item in group.items" :key="item.ref" :label="item.label" :value="item.ref" />
                  </el-option-group>
                </el-select>
              </label>
              <el-button type="primary" :disabled="!mappingTargetPath || !mappingSourceRef" @click="bindMapping">绑定数据</el-button>
              <div v-if="currentMappings.length" class="mapping-list">
                <div v-for="item in currentMappings" :key="item.path" class="mapping-row">
                  <span><strong>{{ item.path }}</strong><small>{{ item.ref }}</small></span>
                  <el-button text type="danger" @click="removeMapping(item.path)">移除</el-button>
                </div>
              </div>
              <div v-else class="empty-inspector compact">当前节点还没有数据引用。</div>
            </div>
            <div v-else class="empty-inspector">先选择一个节点，再配置数据映射。</div>
          </el-tab-pane>

          <el-tab-pane label="AI" name="ai">
            <div class="inspector-content">
              <div class="panel-title">AI Workflow Copilot</div>
              <p class="panel-tip">AI 直接编辑 workflow-v2 DAG。修改会先通过 Kernel 规范化和编译校验，再作为草稿应用到画布，不会绕过保存校验。</p>
              <label>修改要求<el-input v-model="aiInstruction" type="textarea" :rows="6" placeholder="例如：在天气节点后增加条件，只有下雨才通知；HTTP 失败重试 3 次。" /></label>
              <div class="ai-actions">
                <el-button type="primary" :loading="aiWorking" :disabled="!aiInstruction.trim()" @click="aiEdit">AI 修改</el-button>
                <el-button :loading="aiWorking" @click="aiRepair">自动修复</el-button>
                <el-button :loading="aiWorking" @click="aiExplain">解释工作流</el-button>
              </div>
              <div v-if="aiResult" class="ai-result">
                <strong>{{ aiResult.title }}</strong>
                <p>{{ aiResult.summary }}</p>
                <ul v-if="aiResult.items.length"><li v-for="(item, i) in aiResult.items" :key="i">{{ item }}</li></ul>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="运行" name="runs">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">Execution Trace</div><el-button size="small" @click="refreshObservability">刷新</el-button></div>
              <div v-if="executionStats" class="stats-strip">
                <span><strong>{{ executionStats.runCount }}</strong><small>运行</small></span>
                <span><strong>{{ Math.round(executionStats.successRate * 100) }}%</strong><small>成功率</small></span>
                <span><strong>{{ formatDuration(executionStats.averageRunMs) }}</strong><small>平均耗时</small></span>
                <span><strong>{{ executionStats.failed }}</strong><small>失败</small></span>
              </div>
              <div v-if="currentRun" class="current-run-card">
                <div>
                  <strong>{{ currentRun.status }}</strong>
                  <small>{{ currentRun.executionId }}</small>
                  <small v-if="currentRun.context?.executionOptions?.mode">模式 · {{ executionModeLabel(currentRun.context.executionOptions.mode) }}</small>
                  <small v-if="pendingConfirmations.length">待确认 · {{ pendingConfirmations.map(nodeLabel).join("、") }}</small>
                  <template v-if="currentRun.status === 'waiting_device'">
                    <small>等待设备 · {{ waitingDeviceLabel }}</small>
                    <small>原因 · {{ waitingDeviceReason }}</small>
                    <small>离线策略 · 等待设备重新上线后继续原 execution</small>
                  </template>
                  <div v-if="currentRunError" class="run-error-diagnostic">
                    <el-tag size="small" type="danger">{{ currentRunError.category }}</el-tag>
                    <span>{{ currentRunError.message }}</span>
                    <small v-if="currentRunError.recommendedAction">建议 · {{ currentRunError.recommendedAction }}</small>
                  </div>
                </div>
                <div class="run-actions">
                  <el-button v-if="canConfirm" size="small" type="warning" @click="confirmCurrent">确认副作用并继续</el-button>
                  <el-button size="small" :disabled="!canPause" @click="pauseCurrent">暂停</el-button>
                  <el-button size="small" :disabled="!canResume" @click="resumeCurrent">恢复</el-button>
                  <el-button size="small" :disabled="!canRecover" @click="recoverCurrent">从 Checkpoint 恢复</el-button>
                  <el-button size="small" :disabled="!canRerun" @click="rerunCurrent">使用原输入重跑</el-button>
                  <el-button size="small" type="danger" plain :disabled="!canCancel" @click="cancelCurrent">取消</el-button>
                </div>
              </div>
              <article v-for="step in stepRuns" :key="step.nodeId" class="trace-step" @click="selectNode(step.nodeId)">
                <span class="trace-status" :class="`s-${step.status}`"></span>
                <div>
                  <strong>{{ nodeLabel(step.nodeId) }} <em v-if="checkpointNodeIds.has(step.nodeId)" class="checkpoint-badge">checkpoint</em></strong>
                  <small>{{ step.status }} · 最终 attempt {{ step.attempt || 1 }}</small>
                  <small v-if="step.traceId">trace · {{ step.traceId }}<template v-if="step.deviceId"> · device {{ step.deviceId }}</template><template v-if="step.runtimeId"> · runtime {{ step.runtimeId }}</template></small>
                  <small v-if="step.toolCallId">toolCall · {{ step.toolCallId }}</small>
                  <p v-if="step.error">{{ step.error }}</p>
                  <div v-if="attemptsForNode(step.nodeId).length" class="attempt-list">
                    <span v-for="attempt in attemptsForNode(step.nodeId)" :key="`${attempt.generation}-${attempt.attempt}`" :class="`attempt-${attempt.status}`">
                      G{{ attempt.generation }} / #{{ attempt.attempt }} · {{ attempt.status }}<template v-if="attempt.nextBackoffMs"> · {{ attempt.nextBackoffMs }}ms 后重试</template><template v-if="attempt.attemptId"> · {{ attempt.attemptId }}</template><template v-if="attempt.fencingToken"> · fence {{ attempt.fencingToken }}</template>
                    </span>
                  </div>
                </div>
              </article>
              <div class="panel-title run-history-title">历史运行</div>
              <article v-for="run in runs" :key="run.executionId" class="run-history" @click="openRun(run.executionId)">
                <div><strong>{{ run.status }}</strong><small>{{ executionModeLabel(run.context?.executionOptions?.mode) }} · {{ formatTime(run.startedAt) }}</small></div><span>{{ run.executionId.slice(-8) }}</span>
              </article>
              <div v-if="runs.length === 0" class="empty-inspector">暂无运行记录。</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="版本" name="versions">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">版本历史</div><el-button size="small" :loading="revisionBusy" @click="manualSnapshot">保存快照</el-button></div>
              <p class="panel-tip">每次保存修改前会自动记录旧版本，最多保留最近 50 个版本快照。回滚前也会先保存当前状态。</p>
              <article v-for="item in revisions" :key="item.revisionId" class="revision-card">
                <div class="revision-main">
                  <strong>#{{ item.revisionNo }} · {{ item.note || "自动快照" }}</strong>
                  <small>{{ formatTime(item.createdAt) }}</small>
                  <span>{{ item.definitionHash.slice(0, 12) }}</span>
                </div>
                <el-button size="small" text type="primary" :loading="revisionBusy" @click="rollbackRevision(item)">回滚</el-button>
              </article>
              <div v-if="revisions.length === 0" class="empty-inspector compact">暂无版本快照。</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="安全" name="security">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">权限与风险摘要</div><el-tag v-if="safetyAnalysis" :type="riskTagType(safetyAnalysis.riskLevel)" size="small">{{ safetyAnalysis.riskLevel }}</el-tag></div>
              <p class="panel-tip">这里只展示工作流声明和静态风险，不代表相关权限已经被授予；真实执行仍由 Kernel Capability / Scope 策略决定。</p>
              <template v-if="safetyAnalysis">
                <div class="security-section"><strong>声明权限</strong><span v-if="!safetyAnalysis.declaredPermissions.length">无显式权限声明</span><code v-for="item in safetyAnalysis.declaredPermissions" :key="item">{{ item }}</code></div>
                <div class="security-section"><strong>Secret / Credential 引用</strong><span v-if="!safetyAnalysis.secretReferences.length">未发现引用</span><code v-for="item in safetyAnalysis.secretReferences" :key="item">{{ item }}</code></div>
                <div class="security-section"><strong>子工作流依赖</strong><span v-if="!safetyAnalysis.nestedDependencies.length">无</span><div v-for="dep in safetyAnalysis.nestedDependencies" :key="`${dep.nodeId}-${dep.workflowId}`" class="dependency-row"><span>{{ dep.name || dep.workflowId }}</span><el-tag size="small" :type="dep.status === 'ok' ? 'success' : 'danger'">{{ dep.status }}</el-tag></div></div>
                <div class="security-section"><strong>风险提示</strong><span v-if="!safetyAnalysis.risks.length">未发现需要特别提示的静态风险</span><div v-for="(risk, i) in safetyAnalysis.risks" :key="i" class="risk-row"><el-tag size="small" :type="riskTagType(risk.level)">{{ risk.level }}</el-tag><span>{{ risk.nodeId ? `${nodeLabel(risk.nodeId)}：` : '' }}{{ risk.message }}</span></div></div>
              </template>
              <div v-else class="empty-inspector compact">打开此页后加载安全摘要。</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="设置" name="settings">
            <div class="inspector-content">
              <label>启用工作流<el-switch v-model="workflow.enabled" @change="markDirty" /></label>
              <label>允许 AI 调用<el-switch v-model="workflow.callableByAgent" @change="markDirty" /></label>
              <template v-if="workflow.callableByAgent">
                <label>Agent Tool 名称<el-input v-model="workflow.agentTool.name" placeholder="留空则自动生成" maxlength="64" @change="markDirty" /></label>
                <label>Agent Tool 描述<el-input v-model="workflow.agentTool.description" type="textarea" :rows="3" placeholder="告诉模型何时调用这个工作流" maxlength="500" @change="markDirty" /></label>
                <p class="panel-tip">启用并保存后，此工作流会按当前用户隔离注册到 Agent Tool Registry；禁用、关闭或删除时自动撤销。</p>
              </template>
              <div class="reliability-card">
                <div class="panel-title small">并发策略</div>
                <label>同一 Workflow 同时运行时
                  <el-select v-model="workflowConcurrencyMode" @change="markDirty">
                    <el-option label="ALLOW · 全部允许" value="ALLOW" />
                    <el-option label="SINGLETON · 已有运行则不再启动" value="SINGLETON" />
                    <el-option label="QUEUE · 持久化排队" value="QUEUE" />
                    <el-option label="REPLACE · 取消旧运行后执行最新" value="REPLACE" />
                    <el-option label="DROP · 丢弃新运行" value="DROP" />
                    <el-option label="MAX_N · 最多 N 个并发" value="MAX_N" />
                  </el-select>
                </label>
                <label v-if="workflowConcurrencyMode === 'MAX_N'">最大并发数<el-input-number v-model="workflowConcurrencyMaxN" :min="1" :max="64" controls-position="right" /></label>
              </div>
              <template v-if="editorMode === 'advanced'">
                <label>Input Schema<el-input v-model="inputSchemaEditor" type="textarea" :rows="8" @change="applySchemaEditors" /></label>
                <label>Output Schema<el-input v-model="outputSchemaEditor" type="textarea" :rows="8" @change="applySchemaEditors" /></label>
              </template>
              <div class="definition-meta">Schema {{ workflow.schemaVersion }}<br />Hash {{ workflow.definitionHash || "保存后生成" }}</div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </aside>
    </section>

    <el-dialog v-model="preflightDialogVisible" title="工作流预检" width="680px" append-to-body>
      <div v-if="preflightReport" class="preflight-center">
        <div class="preflight-summary"><el-tag :type="preflightReport.runnable ? (preflightReport.status === 'WARNING' ? 'warning' : 'success') : 'danger'">{{ preflightReport.status }}</el-tag><span>{{ preflightReport.runnable ? '可以运行' : '存在阻断项' }}</span></div>
        <article v-for="check in preflightReport.checks" :key="`${check.code}-${check.nodeId || ''}`" class="preflight-check">
          <el-tag size="small" :type="check.status === 'PASS' ? 'success' : check.status === 'WARNING' ? 'warning' : 'danger'">{{ check.status === 'PASS' ? '✓' : check.status === 'WARNING' ? '!' : '×' }}</el-tag>
          <div><strong>{{ check.message }}</strong><small>{{ check.code }}<template v-if="check.nodeId"> · {{ nodeLabel(check.nodeId) }}</template></small></div>
          <el-button v-if="check.nodeId" size="small" text @click="selectNode(check.nodeId); preflightDialogVisible=false">定位节点</el-button>
        </article>
      </div>
      <div v-else class="empty-inspector">尚未执行预检。</div>
      <template #footer><el-button @click="preflightDialogVisible=false">关闭</el-button><el-button type="primary" @click="runPreflight">重新预检</el-button></template>
    </el-dialog>

    <el-dialog v-model="runDialogVisible" title="运行工作流" width="560px" append-to-body destroy-on-close>
      <div class="run-dialog-body">
        <label>执行模式
          <el-select v-model="runMode" style="width:100%">
            <el-option label="Live · 正式执行" value="live" />
            <el-option label="Dry Run · 只验证/规划，不产生副作用" value="dry_run" />
            <el-option label="Mocked · 使用显式 Mock 输出" value="mocked" />
            <el-option label="Controlled Live · 副作用前等待确认" value="controlled_live" />
          </el-select>
        </label>
        <p class="panel-tip run-mode-tip">{{ executionModeDescription(runMode) }}</p>
        <label>Workflow Input (JSON)
          <el-input v-model="runInputEditor" type="textarea" :rows="6" spellcheck="false" />
        </label>
        <label v-if="runMode === 'mocked'">Mocks (JSON Array)
          <el-input v-model="runMocksEditor" type="textarea" :rows="7" spellcheck="false" placeholder='[{"nodeId":"node-1","output":{"ok":true}}]' />
        </label>
        <p v-if="runMode === 'mocked'" class="panel-tip">副作用节点没有显式 Mock 时会直接阻断，不会回落到真实执行。</p>
        <p v-if="runMode === 'controlled_live'" class="controlled-warning">副作用节点会以 <code>waiting_confirmation</code> 持久化暂停；确认后继续同一个 Run ID，不会重新创建一次运行。</p>
      </div>
      <template #footer>
        <el-button @click="runDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="running" @click="startRun">开始运行</el-button>
      </template>
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Close, RefreshLeft, RefreshRight, VideoPlay } from "@element-plus/icons-vue";
import {
  cancelWorkflowRun, confirmWorkflowRun, createWorkflowRevision, createWorkflowTaskerSecret, createWorkflowWakeConfig, editWorkflowWithAI, explainWorkflowWithAI, getWorkflow, getWorkflowAnalysis, getWorkflowCatalog, getWorkflowRun, getWorkflowStats, getWorkflowTriggerAppCatalog, getWorkflowTriggerCapabilities, getWorkflowTriggerWakeConfigs, listWorkflowDevices, listWorkflowRevisions, listWorkflowRuns, listWorkflowSyncEvents, listWorkflows, pauseWorkflowRun, recoverWorkflowRun, repairWorkflowWithAI, rerunWorkflowRun, resumeWorkflowRun, rollbackWorkflowRevision,
  runWorkflow, updateWorkflow, validateWorkflow,
  type WorkflowAIProposal, type WorkflowCatalogItem, type WorkflowCheckpoint, type WorkflowDefinition, type WorkflowDeviceDescriptor, type WorkflowEdge, type WorkflowExecutionStats, type WorkflowErrorDiagnostic, type WorkflowExecutionPlacement, type WorkflowExecutionMode, type WorkflowMockBehavior, type WorkflowNode, type WorkflowOfflinePolicy, type WorkflowPreflightReport, type WorkflowRevisionSummary, type WorkflowRun, type WorkflowSafetyAnalysis, type WorkflowStepAttempt, type WorkflowStepRun, type WorkflowTarget, type WorkflowTrigger, type WorkflowTriggerAppCatalogItem, type WorkflowTriggerCapabilityStatus, type WorkflowTriggerWakeConfigItem, workflowTargetFromQuery,
} from "@/api/workflow";

const route = useRoute();
const workflowId = String(route.params.id || "");
const workflowTarget: WorkflowTarget = workflowTargetFromQuery(route.query as Record<string, unknown>);
const isCloudWorkflow = workflowTarget.location === "cloud";
const workflowTargetLabel = computed(() => workflowTarget.location === "local" ? "当前设备" : workflowTarget.location === "cloud" ? "云端" : `设备 · ${workflowTarget.deviceId || "未选择"}`);
const workflow = reactive<WorkflowDefinition>({
  schemaVersion: "workflow-v2", id: workflowId, name: "工作流", description: "", inputSchema: { type: "object" }, outputSchema: {},
  nodes: [], edges: [], triggers: [{ id: "manual", type: "manual", enabled: true, config: {} }], callableByAgent: false, agentTool: {}, enabled: true,
});
const nodePalette = [
  { type: "tool", label: "Tool", short: "T", description: "调用 Kernel Tool" },
  { type: "mcp", label: "MCP", short: "M", description: "调用 MCP Runtime" },
  { type: "task", label: "Task", short: "K", description: "执行 Task Runtime" },
  { type: "javascript", label: "JavaScript", short: "JS", description: "JavaScript Runtime" },
  { type: "wasm", label: "WASM", short: "W", description: "WASM Runtime" },
  { type: "trusted_service", label: "Trusted Service", short: "S", description: "受信服务 Runtime" },
  { type: "nested_workflow", label: "子工作流", short: "WF", description: "调用另一个工作流" },
  { type: "condition", label: "条件", short: "?", description: "兼容条件锚点 / 条件结果" },
  { type: "logic", label: "逻辑", short: "∴", description: "AND / OR / 比较 / 正则 / 集合判断" },
  { type: "extract", label: "提取", short: "⇲", description: "路径、数组下标与通配符数据提取" },
  { type: "transform", label: "转换", short: "↔", description: "Pick / Merge / Map / Filter / Sort 等" },
  { type: "wait", label: "等待", short: "⏱", description: "延迟执行" },
];

const canvasRef = ref<HTMLElement | null>(null);
const selectedNodeId = ref(""); const selectedEdgeId = ref(""); const inspectorTab = ref("properties");
const editorMode = ref<"simple" | "advanced">("simple");
const dirty = ref(false); const saving = ref(false); const running = ref(false);
const zoom = ref(1); const pan = reactive({ x: 80, y: 70 }); const pointerGraph = reactive({ x: 0, y: 0 });
const connectingFrom = ref("");
const nodeInputEditor = ref("{}"); const nodeWhenEditor = ref(""); const nodePostconditionEditor = ref(""); const nodePermissionsEditor = ref(""); const nodeCapabilitiesEditor = ref(""); const nodeRuntimeMetadataEditor = ref("{}"); const nodeErrorDefaultEditor = ref(""); const compensationInputEditor = ref("{}"); const edgeConditionEditor = ref("");
type SimpleCondition = { id:string; source:"input"|"node_output"; nodeId:string; path:string; op:string; value:string; not:boolean };
const simpleWhenEnabled = ref(false); const simpleWhenCompatible = ref(true); const simpleConditionJoin = ref<"and"|"or">("and"); const simpleConditions = ref<SimpleCondition[]>([]);
const inputSchemaEditor = ref("{}"); const outputSchemaEditor = ref("{}");
const catalog = ref<WorkflowCatalogItem[]>([]); const deviceToolCatalog = ref<WorkflowCatalogItem[]>([]); const deviceToolCatalogId = ref(""); const mappingTargetPath = ref(""); const mappingSourceRef = ref("");
const workflowDevices = ref<WorkflowDeviceDescriptor[]>([]);
const triggerCapabilities = ref<WorkflowTriggerCapabilityStatus[]>([]);
const triggerAppCatalog = ref<WorkflowTriggerAppCatalogItem[]>([]);
const triggerWakeConfigs = ref<WorkflowTriggerWakeConfigItem[]>([]);
const androidSystemEventOptions = [
  { label: "低电量", value: "device.power.battery_low" },
  { label: "电量恢复", value: "device.power.battery_okay" },
  { label: "接入电源", value: "device.power.connected" },
  { label: "断开电源", value: "device.power.disconnected" },
  { label: "屏幕点亮", value: "device.screen.on" },
  { label: "屏幕关闭", value: "device.screen.off" },
  { label: "用户解锁", value: "device.user.present" },
  { label: "耳机连接", value: "device.audio.headset_connected" },
  { label: "耳机断开", value: "device.audio.headset_disconnected" },
  { label: "Wi-Fi 状态变化", value: "device.wifi.state_changed" },
  { label: "Wi-Fi 已启用", value: "device.wifi.enabled" },
  { label: "Wi-Fi 已禁用", value: "device.wifi.disabled" },
  { label: "Wi-Fi 已连接", value: "device.wifi.connected" },
  { label: "Wi-Fi 已断开", value: "device.wifi.disconnected" },
  { label: "设备启动完成", value: "device.system.boot_completed" },
  { label: "系统时间变化", value: "device.time.changed" },
  { label: "时区变化", value: "device.time.timezone_changed" },
  { label: "日期变化", value: "device.time.date_changed" },
];
const deviceWorkflowEventTypes = new Set([
  "device.android.intent",
  "device.android.tasker",
  "voice.wake.detected",
  "voice.asr.final",
  "device.app.foreground",
  "device.notification.posted",
  "device.notification.removed",
  "device.power.battery_changed",
  "device.network.changed",
  "device.network.available",
  "device.network.lost",
  "device.app.installed",
  "device.app.updated",
  "device.app.removed",
  "device.app.self_updated",
  "device.bluetooth.state_changed",
  "device.bluetooth.connected",
  "device.bluetooth.disconnected",
  "device.ble.characteristic_changed",
  "device.location.geofence.enter",
  "device.location.geofence.exit",
  ...androidSystemEventOptions.map(item => item.value),
]);
const taskerSecretBusy = ref("");
const aiInstruction = ref(""); const aiWorking = ref(false); const aiResult = ref<{title:string;summary:string;items:string[]}|null>(null);
const history = ref<string[]>([]); const historyIndex = ref(-1); let restoringHistory = false;
const runs = ref<WorkflowRun[]>([]); const currentRun = ref<WorkflowRun | null>(null); const currentRunError = ref<WorkflowErrorDiagnostic | null>(null); const stepRuns = ref<WorkflowStepRun[]>([]); const stepAttempts = ref<WorkflowStepAttempt[]>([]); const checkpoints = ref<WorkflowCheckpoint[]>([]); let pollTimer: number | undefined;
const executionStats = ref<WorkflowExecutionStats | null>(null); const safetyAnalysis = ref<WorkflowSafetyAnalysis | null>(null); const nestedWorkflowCandidates = ref<WorkflowDefinition[]>([]);
const preflightDialogVisible = ref(false); const preflightReport = ref<WorkflowPreflightReport | null>(null);
const runDialogVisible = ref(false); const runMode = ref<WorkflowExecutionMode>("live"); const runInputEditor = ref("{}"); const runMocksEditor = ref("[]"); const pendingConfirmations = ref<string[]>([]);
const revisions = ref<WorkflowRevisionSummary[]>([]); const revisionBusy = ref(false);
let syncTimer: number | undefined; let syncCursor: number | null = null; let syncBusy = false; let conflictNoticeRevision = 0; let deviceSyncTicks = 0;
let dragState: { nodeId: string; startClientX: number; startClientY: number; startX: number; startY: number } | null = null;
let panState: { startClientX: number; startClientY: number; startX: number; startY: number } | null = null;
let draggedPaletteType = "";

const selectedNode = computed(() => workflow.nodes.find(n => n.id === selectedNodeId.value));
const selectedEdge = computed(() => workflow.edges.find(e => e.id === selectedEdgeId.value));
const simpleConditionNodeOptions = computed(() => workflow.nodes.filter(n => n.id !== selectedNodeId.value));
const effectiveToolCatalog = computed(() => {
  const node = selectedNode.value;
  const deviceId = String(node?.executionTarget?.deviceId || "").trim();
  if (isCloudWorkflow && node?.type === "tool" && node.executionTarget?.placement === "device" && deviceId && deviceToolCatalogId.value === deviceId) return deviceToolCatalog.value;
  return catalog.value;
});
const selectedToolCatalogItem = computed(() => selectedNode.value?.type === "tool" ? effectiveToolCatalog.value.find(item => item.id === selectedNode.value?.targetId) : undefined);
type ToolInputSchemaField = { name:string; type:string; description:string; required:boolean };
const selectedToolInputFields = computed<ToolInputSchemaField[]>(() => {
  const schema = selectedToolCatalogItem.value?.inputSchema;
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) return [];
  const root = schema as Record<string, any>; const properties = root.properties;
  if (!properties || typeof properties !== "object" || Array.isArray(properties)) return [];
  const required = new Set(Array.isArray(root.required) ? root.required.map((value:unknown) => String(value)) : []);
  return Object.entries(properties as Record<string, any>).map(([name,raw]) => {
    const field = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {};
    const type = Array.isArray(field.type) ? field.type.join(" | ") : String(field.type || "any");
    return { name, type, description:String(field.description || field.title || ""), required:required.has(name) };
  }).slice(0, 64);
});
const androidToolGroupLabels: Record<string,string> = {
  accessibility: "无障碍", interaction: "UI 交互", uitree: "UI Tree", display: "屏幕", virtualdisplay: "虚拟屏", root: "Root", adb: "ADB",
  overlay: "悬浮窗", external: "外部自动化", externalautomation: "外部自动化", notification: "通知", clipboard: "剪贴板", share: "分享", camera: "相机",
};
const toolCatalogGroups = computed(() => {
  const groups = new Map<string, WorkflowCatalogItem[]>();
  for (const item of effectiveToolCatalog.value) {
    const runtimeType = String(item.runtime?.runtimeType || "").toLowerCase();
    const android = item.id.startsWith("android.") || runtimeType === "android_native" || String(item.metadata?.bridgeProtocol || "") === "android_native";
    let label = "其他 Tool";
    if (android) {
      const namespace = item.id.split(".")[1] || "native";
      label = `Android · ${androidToolGroupLabels[namespace] || namespace}`;
    } else if (item.source === "workflow") label = "工作流 Tool";
    else if (item.source === "mcp") label = "MCP Tool";
    else if (item.source === "plugin") label = "插件 Tool";
    else if (item.source === "builtin") label = "内置 Tool";
    const values = groups.get(label) || []; values.push(item); groups.set(label, values);
  }
  return [...groups.entries()].sort(([a],[b]) => { const aa=a.startsWith("Android ·")?0:1, bb=b.startsWith("Android ·")?0:1; return aa-bb || a.localeCompare(b); }).map(([label,items]) => ({ label, items: items.sort((a,b) => a.name.localeCompare(b.name)) }));
});
function ensureExecutionTarget(node: WorkflowNode) {
  node.executionTarget ||= { placement: isCloudWorkflow ? "cloud" : "local", offlinePolicy: "fail" };
  node.executionTarget.offlinePolicy ||= "fail";
  return node.executionTarget;
}
const executionPlacement = computed<WorkflowExecutionPlacement>({
  get: () => (selectedNode.value ? (ensureExecutionTarget(selectedNode.value).placement || (isCloudWorkflow ? "cloud" : "local")) : "cloud") as WorkflowExecutionPlacement,
  set: (value) => { if (!selectedNode.value) return; const target = ensureExecutionTarget(selectedNode.value); target.placement = value; if (value !== "device") target.deviceId = ""; markDirty(); void refreshNestedCandidates(); },
});
const executionDeviceId = computed<string>({
  get: () => selectedNode.value ? String(ensureExecutionTarget(selectedNode.value).deviceId || "") : "",
  set: (value) => { if (!selectedNode.value) return; ensureExecutionTarget(selectedNode.value).deviceId = value; markDirty(); void refreshNestedCandidates(); },
});
const executionOfflinePolicy = computed<WorkflowOfflinePolicy>({
  get: () => selectedNode.value ? (ensureExecutionTarget(selectedNode.value).offlinePolicy || "fail") : "fail",
  set: (value) => { if (!selectedNode.value) return; ensureExecutionTarget(selectedNode.value).offlinePolicy = value; markDirty(); },
});
const graphTransform = computed(() => ({ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom.value})` }));
const gridStyle = computed(() => ({ backgroundPosition: `${pan.x}px ${pan.y}px`, backgroundSize: `${24 * zoom.value}px ${24 * zoom.value}px` }));
const waitDuration = computed({ get: () => Number(selectedNode.value?.runtime?.metadata?.durationMs || 0), set: v => { if (selectedNode.value) { ensureRuntime(selectedNode.value).metadata!.durationMs = Number(v || 0); markDirty(); } } });
const nodeTimeoutMs = computed({ get: () => Number(selectedNode.value?.timeoutMs || 0), set: v => { if (selectedNode.value) { selectedNode.value.timeoutMs = Math.max(0, Number(v || 0)); markDirty(); } } });
const maxNodeTimeoutMs = computed(() => Math.max(1, Number((workflow.limits as Record<string, unknown> | undefined)?.maxStepDurationMs || 300000)));
const retryEnabled = computed({ get: () => !!selectedNode.value?.retry, set: enabled => { const n=selectedNode.value;if(!n)return;n.retry=enabled?{maxAttempts:3,initialBackoffMs:200,maxBackoffMs:30000,multiplier:2,jitter:0.2}:undefined;markDirty(); } });
const retryMaxAttempts = computed({ get: () => Number(selectedNode.value?.retry?.maxAttempts || 3), set: v => { const n=selectedNode.value;if(!n)return;n.retry ||= {};n.retry.maxAttempts=Math.max(1,Math.min(10,Number(v||1)));markDirty(); } });
const retryInitialBackoffMs = computed({ get: () => Number(selectedNode.value?.retry?.initialBackoffMs ?? 200), set: v => { const n=selectedNode.value;if(!n)return;n.retry ||= {};n.retry.initialBackoffMs=Math.max(0,Number(v||0));markDirty(); } });
const retryMaxBackoffMs = computed({ get: () => Number(selectedNode.value?.retry?.maxBackoffMs ?? 30000), set: v => { const n=selectedNode.value;if(!n)return;n.retry ||= {};n.retry.maxBackoffMs=Math.max(0,Number(v||0));markDirty(); } });
const retryMultiplier = computed({ get: () => Number(selectedNode.value?.retry?.multiplier ?? 2), set: v => { const n=selectedNode.value;if(!n)return;n.retry ||= {};n.retry.multiplier=Math.max(1.1,Number(v||2));markDirty(); } });
const retryJitter = computed({ get: () => Number(selectedNode.value?.retry?.jitter ?? 0.2), set: v => { const n=selectedNode.value;if(!n)return;n.retry ||= {};n.retry.jitter=Math.max(0,Math.min(1,Number(v||0)));markDirty(); } });
const compensationEnabled = computed({ get:()=>!!selectedNode.value?.compensation, set:(enabled:boolean)=>{const n=selectedNode.value;if(!n)return;if(enabled&&!n.compensation)n.compensation={type:"tool",targetId:"",input:{},timeoutMs:30000};if(!enabled)n.compensation=undefined;compensationInputEditor.value=pretty(n.compensation?.input??{});markDirty();} });
const workflowConcurrencyMode = computed({get:()=>workflow.concurrencyPolicy?.mode||"ALLOW",set:(mode)=>{workflow.concurrencyPolicy={...(workflow.concurrencyPolicy||{}),mode:mode as any};if(mode!=="MAX_N")workflow.concurrencyPolicy.maxN=undefined;markDirty();}});
const workflowConcurrencyMaxN = computed({get:()=>Number(workflow.concurrencyPolicy?.maxN||1),set:(value)=>{workflow.concurrencyPolicy={...(workflow.concurrencyPolicy||{}),mode:"MAX_N",maxN:Math.max(1,Math.min(64,Number(value||1)))};markDirty();}});
const canPause = computed(() => currentRun.value?.status === "running" || currentRun.value?.status === "resuming");
const canResume = computed(() => currentRun.value?.status === "paused");
const canCancel = computed(() => ["queued","running","pausing","paused","resuming","waiting_device","waiting_confirmation","compensating","cancel_requested","cancelling"].includes(currentRun.value?.status || ""));
const canConfirm = computed(() => currentRun.value?.status === "waiting_confirmation" && pendingConfirmations.value.length > 0);
const waitingDeviceLabel = computed(() => { const ctx=currentRun.value?.context as Record<string,any>|undefined; const id=String(ctx?.waitingDeviceId||ctx?.deviceId||"").trim(); const known=workflowDevices.value.find(item=>item.deviceId===id); return known?.label || id || "目标设备"; });
const waitingDeviceReason = computed(() => { const ctx=currentRun.value?.context as Record<string,any>|undefined; return String(ctx?.waitingReason||currentRun.value?.error||"设备离线或所需 Capability 暂不可用"); });
const canRerun = computed(() => !!currentRun.value && ["succeeded","failed","cancelled","compensated"].includes(currentRun.value.status) && workflow.enabled);
const canRecover = computed(() => !!currentRun.value && ["failed","cancelled"].includes(currentRun.value.status) && checkpoints.value.length > 0 && workflow.enabled);
const checkpointNodeIds = computed(() => new Set(checkpoints.value.map(item => item.nodeId)));
const previewPath = computed(() => { const n = workflow.nodes.find(x => x.id === connectingFrom.value); if (!n) return ""; const s = outputPoint(n); return bezier(s.x, s.y, pointerGraph.x, pointerGraph.y); });
const selectedTargetCatalog = computed(() => {
  const node = selectedNode.value; if (!node) return undefined;
  const items = node.type === "tool" ? effectiveToolCatalog.value : catalog.value;
  return items.find(item => item.id === node.targetId || item.modelName === node.targetId || item.runtime?.runtimeId === node.runtime?.runtimeId);
});
const mappingTargetFields = computed(() => {
  const paths = schemaLeafPaths(selectedTargetCatalog.value?.inputSchema);
  if (paths.length) return paths;
  const input = selectedNode.value?.step?.input;
  return input && typeof input === "object" && !Array.isArray(input) ? Object.keys(input as Record<string, unknown>) : [];
});
const mappingSourceGroups = computed(() => {
  const groups: Array<{label:string;items:Array<{label:string;ref:string}>}> = [];
  const inputItems = schemaLeafPaths(workflow.inputSchema).map(path => ({ label: `工作流输入 · ${path}`, ref: `input.${path}` }));
  if (inputItems.length) groups.push({ label: "工作流输入", items: inputItems });
  groups.push({
    label: "运行时上下文",
    items: [
      { label: "当前用户 · userId", ref: "runtime.userId" },
      { label: "当前会话 · conversationId", ref: "runtime.conversationId" },
      { label: "当前角色 · characterId", ref: "runtime.characterId" },
      { label: "根任务 · rootId", ref: "runtime.rootId" },
      { label: "调度任务 · scheduleId", ref: "runtime.scheduleId" },
      { label: "执行追踪 · traceId", ref: "runtime.traceId" },
    ],
  });
  const target = selectedNode.value;
  if (!target) return groups;
  for (const node of workflow.nodes) {
    if (node.id === target.id || wouldCreateCycle(node.id, target.id)) continue;
    const item = catalog.value.find(x => x.id === node.targetId || x.modelName === node.targetId || x.runtime?.runtimeId === node.runtime?.runtimeId);
    let paths = schemaLeafPaths(item?.outputSchema);
    if (!paths.length) {
      const observed = stepRuns.value.find(step => step.nodeId === node.id)?.output;
      paths = valueLeafPaths(observed);
    }
    if (!paths.length) continue;
    groups.push({ label: node.label || node.id, items: paths.map(path => ({ label: `${node.label || node.id} · ${path}`, ref: `steps.${node.id}.${path}` })) });
  }
  return groups;
});
const currentMappings = computed(() => collectMappings(selectedNode.value?.step?.input));

function paletteByType(type: string) { return nodePalette.find(x => x.type === type); }
function supportsPlacement(type: string) { return ["tool", "mcp", "task", "javascript", "wasm", "trusted_service", "nested_workflow"].includes(type); }
function onExecutionTargetChanged() { markDirty(); void refreshNestedCandidates(); void refreshSelectedToolCatalog(); }
async function refreshSelectedToolCatalog() {
  const node = selectedNode.value;
  if (!isCloudWorkflow || node?.type !== "tool" || node.executionTarget?.placement !== "device") { deviceToolCatalog.value=[]; deviceToolCatalogId.value=""; return; }
  const deviceId=String(node.executionTarget?.deviceId||"").trim();
  if(!deviceId){deviceToolCatalog.value=[];deviceToolCatalogId.value="";return;}
  if(deviceToolCatalogId.value===deviceId&&deviceToolCatalog.value.length)return;
  try{
    const items=await getWorkflowCatalog({location:"device",deviceId});
    const current=selectedNode.value;
    if(current?.type==="tool"&&current.executionTarget?.placement==="device"&&String(current.executionTarget?.deviceId||"").trim()===deviceId){deviceToolCatalog.value=items;deviceToolCatalogId.value=deviceId;}
  }catch{
    const current=selectedNode.value;
    if(current?.type==="tool"&&String(current.executionTarget?.deviceId||"").trim()===deviceId){deviceToolCatalog.value=[];deviceToolCatalogId.value=deviceId;}
  }
}
async function refreshNestedCandidates() {
  if (selectedNode.value?.type !== "nested_workflow") return;
  let target: WorkflowTarget = workflowTarget;
  if (isCloudWorkflow && executionPlacement.value === "device" && executionDeviceId.value) target = { location: "device", deviceId: executionDeviceId.value };
  if (isCloudWorkflow && executionPlacement.value === "cloud") target = { location: "cloud" };
  try { nestedWorkflowCandidates.value = (await listWorkflows(target)).filter(item => item.id !== workflowId); } catch { nestedWorkflowCandidates.value = []; }
}
function needsTarget(type: string) { return ["tool","mcp","task","javascript","wasm","trusted_service","nested_workflow"].includes(type); }
function needsRuntime(type: string) { return ["mcp","task","javascript","wasm","trusted_service"].includes(type); }
function toolRiskTagType(risk?: string) { return risk === "high" ? "danger" : risk === "medium" ? "warning" : risk === "low" ? "success" : "info"; }
function onToolTargetChanged() {
  const node=selectedNode.value; if(!node || node.type!=="tool") return;
  const item=effectiveToolCatalog.value.find(entry=>entry.id===node.targetId);
  if(item){
    node.permissions=(item.permissions||[]).map(permission=>permission.capability).filter(Boolean);
    if(!node.timeoutMs && item.timeoutMs) node.timeoutMs=item.timeoutMs;
  }
  nodePermissionsEditor.value=(node.permissions||[]).join(", ");
  markDirty();
}
function toolInputObject(): Record<string,unknown> {
  const value=selectedNode.value?.step?.input;
  return value && typeof value==="object" && !Array.isArray(value) ? value as Record<string,unknown> : {};
}
function toolInputFieldText(name:string): string {
  const value=toolInputObject()[name];
  if(value==null)return "";
  if(typeof value==="string")return value;
  if(typeof value==="object")return JSON.stringify(value);
  return String(value);
}
function setToolInputField(field:ToolInputSchemaField, rawValue:unknown) {
  const node=selectedNode.value;if(!node)return;
  if(!node.step?.input || typeof node.step.input!=="object" || Array.isArray(node.step.input))node.step.input={};
  const input=node.step.input as Record<string,unknown>; const raw=String(rawValue??"").trim();
  if(!raw){delete input[field.name];nodeInputEditor.value=pretty(input);markDirty();return;}
  const isRef=/^(?:\$\{|input\.|steps\.|nodes\.|runtime\.|config\.|literal:)/.test(raw);
  let value:unknown=raw;
  if(!isRef){
    const type=field.type.toLowerCase();
    try{
      if(type.includes("boolean")&&(raw==="true"||raw==="false"))value=raw==="true";
      else if(type.includes("integer")){const parsed=Number(raw);if(!Number.isInteger(parsed))throw new Error("必须是整数");value=parsed;}
      else if(type.includes("number")){const parsed=Number(raw);if(!Number.isFinite(parsed))throw new Error("必须是数字");value=parsed;}
      else if(type.includes("object")||type.includes("array"))value=JSON.parse(raw);
    }catch(error:any){ElMessage.error(`${field.name}：${error?.message||"参数格式错误"}`);return;}
  }
  input[field.name]=value;nodeInputEditor.value=pretty(input);markDirty();
}
function defaultNodeInput(type: string): Record<string,unknown> {
  if(type === "logic") return { op: "eq", left: true, right: true };
  if(type === "extract") return { path: "value", required: false, unwrap: true };
  if(type === "transform") return { op: "pick", fields: [] };
  return {};
}
function defaultRuntimeType(type: string) { return ({ mcp:"mcp", task:"task", javascript:"javascript", wasm:"wasm", trusted_service:"trusted_service" } as Record<string,string>)[type] || ""; }
function ensureRuntime(node: WorkflowNode) { node.runtime ||= {}; node.runtime.metadata ||= {}; return node.runtime; }
function markDirty() { if (!restoringHistory) dirty.value = true; }
function pretty(value: unknown) { return value == null ? "" : JSON.stringify(value, null, 2); }
function parseEditor(text: string, fallback: unknown) { if (!text.trim()) return fallback; return JSON.parse(text); }
function modelSnapshot() { return JSON.stringify({ name: workflow.name, description: workflow.description, inputSchema: workflow.inputSchema, outputSchema: workflow.outputSchema, nodes: workflow.nodes, edges: workflow.edges, triggers: workflow.triggers, concurrencyPolicy: workflow.concurrencyPolicy, callableByAgent: workflow.callableByAgent, agentTool: workflow.agentTool, enabled: workflow.enabled }); }
function pushHistory() { const snap = modelSnapshot(); if (history.value[historyIndex.value] === snap) return; history.value = history.value.slice(0, historyIndex.value + 1); history.value.push(snap); if (history.value.length > 60) history.value.shift(); historyIndex.value = history.value.length - 1; }
function restoreSnapshot(snap: string) { restoringHistory = true; const data = JSON.parse(snap); Object.assign(workflow, data); selectedNodeId.value = ""; selectedEdgeId.value = ""; restoringHistory = false; dirty.value = true; }
function undo() { if (historyIndex.value <= 0) return; historyIndex.value--; restoreSnapshot(history.value[historyIndex.value]); }
function redo() { if (historyIndex.value >= history.value.length - 1) return; historyIndex.value++; restoreSnapshot(history.value[historyIndex.value]); }

function schemaLeafPaths(schema: unknown, prefix = ""): string[] {
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) return [];
  const obj = schema as Record<string, any>; const props = obj.properties;
  if (!props || typeof props !== "object") return prefix ? [prefix] : [];
  const out: string[] = [];
  for (const [key, child] of Object.entries(props as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${key}` : key; const nested = schemaLeafPaths(child, path); out.push(...(nested.length ? nested : [path]));
  }
  return out;
}
function valueLeafPaths(value: unknown, prefix = "", depth = 0): string[] {
  if (depth > 6) return prefix ? [prefix] : [];
  if (value == null || typeof value !== "object") return prefix ? [prefix] : [];
  if (Array.isArray(value)) {
    if (!value.length) return prefix ? [prefix] : [];
    return valueLeafPaths(value[0], prefix ? `${prefix}.0` : "0", depth + 1);
  }
  const entries = Object.entries(value as Record<string, unknown>);
  if (!entries.length) return prefix ? [prefix] : [];
  const out: string[] = [];
  for (const [key, child] of entries) {
    const path = prefix ? `${prefix}.${key}` : key;
    const nested = valueLeafPaths(child, path, depth + 1);
    out.push(...(nested.length ? nested : [path]));
  }
  return [...new Set(out)].slice(0, 100);
}
function setPath(root: Record<string, any>, path: string, value: unknown) {
  const parts = path.split(".").map(x=>x.trim()).filter(Boolean); if (!parts.length) return; let cur=root;
  for (let i=0;i<parts.length-1;i++){const key=parts[i];if(!cur[key]||typeof cur[key]!=="object"||Array.isArray(cur[key]))cur[key]={};cur=cur[key];}
  cur[parts[parts.length-1]]=value;
}
function deletePath(root: Record<string, any>, path: string) {
  const parts=path.split(".").filter(Boolean); if(!parts.length)return; let cur:any=root;
  for(let i=0;i<parts.length-1;i++){cur=cur?.[parts[i]];if(!cur||typeof cur!=="object")return;} delete cur[parts[parts.length-1]];
}
function collectMappings(value: unknown, prefix=""): Array<{path:string;ref:string}> {
  if (typeof value === "string" && /^(input|steps|runtime)\./.test(value)) return prefix ? [{path:prefix,ref:value}] : [];
  if (!value || typeof value !== "object" || Array.isArray(value)) return [];
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) => collectMappings(child, prefix ? `${prefix}.${key}` : key));
}
function isAncestor(source:string,target:string){if(source===target)return false;const stack=[source],seen=new Set<string>();while(stack.length){const id=stack.pop()!;if(id===target)return true;if(seen.has(id))continue;seen.add(id);for(const e of workflow.edges)if(e.source===id)stack.push(e.target);}return false;}
function bindMapping(){
  const node=selectedNode.value,targetPath=mappingTargetPath.value.trim(),refValue=mappingSourceRef.value.trim();if(!node||!targetPath||!refValue)return;
  const match=/^steps\.([^.]+)\./.exec(refValue); const sourceId=match?.[1];
  if(sourceId&&!isAncestor(sourceId,node.id)){if(wouldCreateCycle(sourceId,node.id)){ElMessage.error("该数据来源会形成循环依赖");return;}workflow.edges.push({id:`edge-map-${Date.now().toString(36)}`,source:sourceId,target:node.id,label:"data"});}
  const input=(node.step.input&&typeof node.step.input==="object"&&!Array.isArray(node.step.input)?node.step.input:{} ) as Record<string,any>; setPath(input,targetPath,refValue); node.step.input=input; nodeInputEditor.value=pretty(input); markDirty();pushHistory();ElMessage.success("数据映射已绑定");
}
function removeMapping(path:string){const node=selectedNode.value;if(!node||!node.step.input||typeof node.step.input!=="object"||Array.isArray(node.step.input))return;deletePath(node.step.input as Record<string,any>,path);nodeInputEditor.value=pretty(node.step.input);markDirty();pushHistory();}
function normalizeLoadedDefinition(){
  workflow.nodes=Array.isArray(workflow.nodes)?workflow.nodes:[]; workflow.edges ||= []; workflow.triggers ||= [{id:"manual",type:"manual",enabled:true,config:{}}]; workflow.agentTool ||= {}; workflow.concurrencyPolicy ||= {mode:"ALLOW"};
  for(const trigger of workflow.triggers){trigger.config ||= {};} for(const n of workflow.nodes){n.position ||= {x:100,y:100};n.requiredCapabilities ||= [];n.step ||= {input:{},onError:{mode:"fail"}};n.step.onError ||= {mode:"fail"};ensureRuntime(n);const target=ensureExecutionTarget(n);if(!isCloudWorkflow){target.placement="local";target.deviceId="";target.runtimeId="";target.providerInstanceId="";}else if(!target.placement||target.placement==="local"){target.placement="cloud";}}
  inputSchemaEditor.value=pretty(workflow.inputSchema);outputSchemaEditor.value=pretty(workflow.outputSchema);
}
function applyAIProposal(proposal: WorkflowAIProposal){
  const def=JSON.parse(JSON.stringify(proposal.definition)) as WorkflowDefinition;def.id=workflow.id;Object.assign(workflow,def);normalizeLoadedDefinition();selectedNodeId.value="";selectedEdgeId.value="";dirty.value=true;pushHistory();autoLayout();aiResult.value={title:"AI 修改已应用到草稿",summary:proposal.summary||"工作流已更新",items:[...(proposal.changes||[]),...(proposal.warnings||[]).map(x=>`警告：${x}`)]};
}
async function ensureSavedForAI(){if(workflowTarget.location==="device"){ElMessage.warning("远程设备控制面不暴露 AI Copilot，请在当前设备或云端工作流中使用。");return false;}if(!dirty.value)return true;await save();return !dirty.value;}
async function aiEdit(){if(!aiInstruction.value.trim()||aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{applyAIProposal(await editWorkflowWithAI(workflow.id,aiInstruction.value.trim(),workflowTarget));ElMessage.success("AI 修改已应用，请确认后保存");}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 修改失败");}finally{aiWorking.value=false;}}
async function aiRepair(){if(aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{applyAIProposal(await repairWorkflowWithAI(workflow.id,aiInstruction.value.trim(),workflowTarget));ElMessage.success("AI 修复建议已应用，请确认后保存");}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 修复失败");}finally{aiWorking.value=false;}}
async function aiExplain(){if(aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{const result=await explainWorkflowWithAI(workflow.id,aiInstruction.value.trim(),workflowTarget);aiResult.value={title:"AI 工作流解释",summary:result.summary,items:[...(result.flow||[]).map(x=>`流程：${x}`),...(result.issues||[]).map(x=>`问题：${x}`),...(result.suggestions||[]).map(x=>`建议：${x}`)]};}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 解释失败");}finally{aiWorking.value=false;}}

function addNode(type: string, position?: {x:number;y:number}) {
  pushHistory(); const index = workflow.nodes.length;
  const node: WorkflowNode = { id: `${type}-${Date.now().toString(36)}-${index+1}`, type, label: paletteByType(type)?.label || type, position: position || { x: 120 + (index%4)*230, y: 120 + Math.floor(index/4)*150 }, step: { input: defaultNodeInput(type), onError: { mode: "fail" } }, runtime: { runtimeType: defaultRuntimeType(type), metadata: {} }, executionTarget: { placement: isCloudWorkflow ? "cloud" : "local", offlinePolicy: "fail" } };
  if (type === "wait") node.runtime.metadata = { durationMs: 1000 };
  workflow.nodes.push(node); selectNode(node.id); markDirty(); pushHistory();
}
function removeNode(id: string) { pushHistory(); workflow.nodes = workflow.nodes.filter(n => n.id !== id); workflow.edges = workflow.edges.filter(e => e.source !== id && e.target !== id); if (selectedNodeId.value === id) selectedNodeId.value=""; markDirty(); pushHistory(); }
function removeEdge(id: string) { pushHistory(); workflow.edges = workflow.edges.filter(e => e.id !== id); selectedEdgeId.value=""; markDirty(); pushHistory(); }
function selectNode(id: string) { selectedNodeId.value=id; selectedEdgeId.value=""; inspectorTab.value="properties"; syncNodeEditors(); void refreshNestedCandidates(); void refreshSelectedToolCatalog(); }
function selectEdge(id: string) { selectedEdgeId.value=id; selectedNodeId.value=""; inspectorTab.value="properties"; edgeConditionEditor.value = pretty(selectedEdge.value?.condition); }
function onNodeTypeChanged() { const n=selectedNode.value; if(!n)return; const rt=defaultRuntimeType(n.type); if(rt) ensureRuntime(n).runtimeType=rt; ensureExecutionTarget(n); markDirty(); void refreshNestedCandidates(); void refreshSelectedToolCatalog(); }
function newSimpleCondition(): SimpleCondition { return { id:`cond-${Date.now().toString(36)}-${Math.random().toString(36).slice(2,6)}`, source:"input", nodeId:"", path:"", op:"eq", value:"true", not:false }; }
function simpleValueText(value: unknown): string { if(typeof value === "string") return value; try{return JSON.stringify(value);}catch{return String(value ?? "");} }
function parseSimpleConditionExpression(raw:any): SimpleCondition | null { let expr=raw,not=false;if(expr&&expr.op==="not"&&expr.right){not=true;expr=expr.right;}if(!expr||typeof expr!=="object")return null;const op=String(expr.op||"");if(!["eq","ne","contains","gt","gte","lt","lte","exists","is_null"].includes(op))return null;const unary=["exists","is_null"].includes(op);const ref=unary?expr.ref:expr.left?.ref;if(!ref)return null;const source=String(ref.source||"");if(source!=="input"&&source!=="node_output")return null;const path=Array.isArray(ref.path)?ref.path.map((v:any)=>String(v)).join("."):"";if(!unary&&(!expr.right||!("value" in expr.right)))return null;return {id:`cond-${Date.now().toString(36)}-${Math.random().toString(36).slice(2,6)}`,source:source as "input"|"node_output",nodeId:String(ref.nodeId||""),path,op,value:unary?"":simpleValueText(expr.right.value),not}; }
function syncSimpleWhen(raw:unknown){simpleWhenEnabled.value=raw!=null;simpleWhenCompatible.value=true;simpleConditionJoin.value="and";simpleConditions.value=[];if(raw==null){simpleConditions.value=[newSimpleCondition()];return;}const root=raw as any;let rows:any[]=[root];if(root&&["and","or"].includes(String(root.op))&&Array.isArray(root.args)){simpleConditionJoin.value=root.op;rows=root.args;}const parsed=rows.map(parseSimpleConditionExpression);if(parsed.some(x=>!x)){simpleWhenCompatible.value=false;simpleConditions.value=[newSimpleCondition()];return;}simpleConditions.value=parsed as SimpleCondition[];if(!simpleConditions.value.length)simpleConditions.value=[newSimpleCondition()];}
function decodeSimpleValue(value:string):unknown{const trimmed=value.trim();if(!trimmed)return "";try{return JSON.parse(trimmed);}catch{return value;}}
function buildSimpleConditionExpression(c:SimpleCondition):any{const ref:any={source:c.source,path:c.path.split(".").map(v=>v.trim()).filter(Boolean)};if(c.source==="node_output")ref.nodeId=c.nodeId;const unary=["exists","is_null"].includes(c.op);let expr:any=unary?{op:c.op,ref}:{op:c.op,left:{ref},right:{value:decodeSimpleValue(c.value)}};if(c.not)expr={op:"not",right:expr};return expr;}
function applySimpleWhen(){const n=selectedNode.value;if(!n||!simpleWhenCompatible.value)return;n.step ||= {input:{},onError:{mode:"fail"}};if(!simpleWhenEnabled.value){n.step.when=undefined;nodeWhenEditor.value="";markDirty();return;}if(!simpleConditions.value.length)simpleConditions.value=[newSimpleCondition()];const args=simpleConditions.value.map(buildSimpleConditionExpression);const expr=args.length===1?args[0]:{op:simpleConditionJoin.value,args};n.step.when=expr;nodeWhenEditor.value=pretty(expr);markDirty();}
function onSimpleWhenEnabledChanged(){if(simpleWhenEnabled.value&&!simpleConditions.value.length)simpleConditions.value=[newSimpleCondition()];if(simpleWhenCompatible.value)applySimpleWhen();}
function replaceAdvancedWhenWithSimple(){simpleWhenCompatible.value=true;simpleConditions.value=[newSimpleCondition()];simpleWhenEnabled.value=true;applySimpleWhen();}
function addSimpleCondition(){simpleConditions.value.push(newSimpleCondition());applySimpleWhen();}
function removeSimpleCondition(index:number){simpleConditions.value.splice(index,1);if(!simpleConditions.value.length)simpleConditions.value=[newSimpleCondition()];applySimpleWhen();}
function syncNodeEditors() { const n=selectedNode.value; if(!n)return; ensureRuntime(n); nodeInputEditor.value=pretty(n.step?.input ?? {}); nodeWhenEditor.value=pretty(n.step?.when); syncSimpleWhen(n.step?.when); nodePostconditionEditor.value=pretty(n.step?.postcondition); nodePermissionsEditor.value=(n.permissions||[]).join(", "); nodeCapabilitiesEditor.value=(n.requiredCapabilities||[]).join(", "); nodeRuntimeMetadataEditor.value=pretty(n.runtime?.metadata ?? {}); nodeErrorDefaultEditor.value=pretty(n.step?.onError?.default); compensationInputEditor.value=pretty(n.compensation?.input ?? {}); }
function applyNodeEditors(): boolean { const n=selectedNode.value; if(!n)return true; try { n.step ||= { input:{}, onError:{mode:"fail"} }; n.step.onError ||= { mode:"fail" }; n.step.input=parseEditor(nodeInputEditor.value, {}); if(editorMode.value === "advanced") n.step.when=nodeWhenEditor.value.trim()?parseEditor(nodeWhenEditor.value, undefined):undefined; n.step.postcondition=nodePostconditionEditor.value.trim()?parseEditor(nodePostconditionEditor.value, undefined):undefined; n.step.onError.default=n.step.onError.mode==="use_default"?(nodeErrorDefaultEditor.value.trim()?parseEditor(nodeErrorDefaultEditor.value, undefined):null):undefined; n.permissions=nodePermissionsEditor.value.split(",").map(x=>x.trim()).filter(Boolean); n.requiredCapabilities=nodeCapabilitiesEditor.value.split(",").map(x=>x.trim()).filter(Boolean); ensureRuntime(n).metadata=parseEditor(nodeRuntimeMetadataEditor.value, {}) as Record<string, unknown>; if(n.compensation){n.compensation.input=compensationInputEditor.value.trim()?parseEditor(compensationInputEditor.value, {}):{};n.compensation.action=n.compensation.targetId||n.compensation.action||"";} markDirty(); return true; } catch(e:any){ ElMessage.error(`节点 JSON 无效：${e.message}`); return false; } }
function applyEdgeEditor(): boolean { const e=selectedEdge.value;if(!e)return true; try{e.condition=edgeConditionEditor.value.trim()?parseEditor(edgeConditionEditor.value, undefined):undefined;markDirty();return true;}catch(err:any){ElMessage.error(`连线条件 JSON 无效：${err.message}`);return false;} }
function applySchemaEditors(): boolean { try { workflow.inputSchema=parseEditor(inputSchemaEditor.value,{}); workflow.outputSchema=parseEditor(outputSchemaEditor.value,{}); markDirty(); return true; } catch(e:any){ElMessage.error(`Schema JSON 无效：${e.message}`);return false;} }
function applyWaitDuration(v: number | undefined) { waitDuration.value = Number(v || 0); }

function nodeStyle(node: WorkflowNode) { return { left:`${node.position?.x || 0}px`, top:`${node.position?.y || 0}px` }; }
function inputPoint(node: WorkflowNode) { return { x:(node.position?.x||0), y:(node.position?.y||0)+42 }; }
function outputPoint(node: WorkflowNode) { return { x:(node.position?.x||0)+180, y:(node.position?.y||0)+42 }; }
function bezier(x1:number,y1:number,x2:number,y2:number) { const d=Math.max(70,Math.abs(x2-x1)*0.45); return `M ${x1} ${y1} C ${x1+d} ${y1}, ${x2-d} ${y2}, ${x2} ${y2}`; }
function edgePath(edge: WorkflowEdge) { const s=workflow.nodes.find(n=>n.id===edge.source), t=workflow.nodes.find(n=>n.id===edge.target); if(!s||!t)return ""; const a=outputPoint(s),b=inputPoint(t); return bezier(a.x,a.y,b.x,b.y); }
function edgeLabelPoint(edge: WorkflowEdge) { const s=workflow.nodes.find(n=>n.id===edge.source), t=workflow.nodes.find(n=>n.id===edge.target); if(!s||!t)return{x:0,y:0}; const a=outputPoint(s),b=inputPoint(t); return{x:(a.x+b.x)/2,y:(a.y+b.y)/2-8}; }
function canvasCoordinates(clientX:number,clientY:number){const r=canvasRef.value!.getBoundingClientRect();return{x:(clientX-r.left-pan.x)/zoom.value,y:(clientY-r.top-pan.y)/zoom.value};}
function startNodeDrag(node:WorkflowNode,event:PointerEvent){ if(event.button!==0)return; pushHistory(); dragState={nodeId:node.id,startClientX:event.clientX,startClientY:event.clientY,startX:node.position?.x||0,startY:node.position?.y||0}; window.addEventListener("pointermove",onGlobalPointerMove);window.addEventListener("pointerup",onGlobalPointerUp,{once:true}); }
function startConnect(nodeId:string,event:PointerEvent){ connectingFrom.value=nodeId; const p=canvasCoordinates(event.clientX,event.clientY);pointerGraph.x=p.x;pointerGraph.y=p.y;window.addEventListener("pointermove",onGlobalPointerMove);window.addEventListener("pointerup",onConnectPointerUp,{once:true}); }
function onConnectPointerUp(){window.removeEventListener("pointermove",onGlobalPointerMove);setTimeout(()=>{connectingFrom.value=""},50);}
function finishConnect(targetId:string){ const source=connectingFrom.value;if(!source||source===targetId)return; if(workflow.edges.some(e=>e.source===source&&e.target===targetId)){connectingFrom.value="";return;} if(wouldCreateCycle(source,targetId)){ElMessage.error("该连线会形成环，DAG 不允许循环依赖");connectingFrom.value="";return;} pushHistory(); workflow.edges.push({id:`edge-${Date.now().toString(36)}`,source,target:targetId});connectingFrom.value="";markDirty();pushHistory(); }
function wouldCreateCycle(source:string,target:string){const adj=new Map<string,string[]>();for(const e of workflow.edges){const a=adj.get(e.source)||[];a.push(e.target);adj.set(e.source,a);}const stack=[target],seen=new Set<string>();while(stack.length){const n=stack.pop()!;if(n===source)return true;if(seen.has(n))continue;seen.add(n);stack.push(...(adj.get(n)||[]));}return false;}
function onGlobalPointerMove(event:PointerEvent){if(dragState){const n=workflow.nodes.find(x=>x.id===dragState!.nodeId);if(n){n.position={x:Math.max(0,dragState.startX+(event.clientX-dragState.startClientX)/zoom.value),y:Math.max(0,dragState.startY+(event.clientY-dragState.startClientY)/zoom.value)};markDirty();}}if(connectingFrom.value&&canvasRef.value){const p=canvasCoordinates(event.clientX,event.clientY);pointerGraph.x=p.x;pointerGraph.y=p.y;}}
function onGlobalPointerUp(){window.removeEventListener("pointermove",onGlobalPointerMove);dragState=null;pushHistory();}
function onCanvasPointerDown(event:PointerEvent){if(event.target!==canvasRef.value&&!(event.target as HTMLElement).classList.contains("canvas-grid"))return;if(event.button!==0)return;selectedNodeId.value="";selectedEdgeId.value="";panState={startClientX:event.clientX,startClientY:event.clientY,startX:pan.x,startY:pan.y};const move=(e:PointerEvent)=>{if(!panState)return;pan.x=panState.startX+(e.clientX-panState.startClientX);pan.y=panState.startY+(e.clientY-panState.startClientY);};const up=()=>{window.removeEventListener("pointermove",move);panState=null;};window.addEventListener("pointermove",move);window.addEventListener("pointerup",up,{once:true});}
function onWheel(e:WheelEvent){const delta=e.deltaY>0?-0.08:0.08;zoomBy(delta);}
function zoomBy(delta:number){zoom.value=Math.min(1.8,Math.max(0.35,Number((zoom.value+delta).toFixed(2))));}
function fitView(){if(workflow.nodes.length===0){zoom.value=1;pan.x=80;pan.y=70;return;}const xs=workflow.nodes.map(n=>n.position?.x||0),ys=workflow.nodes.map(n=>n.position?.y||0);const minX=Math.min(...xs),maxX=Math.max(...xs)+180,minY=Math.min(...ys),maxY=Math.max(...ys)+84;const r=canvasRef.value?.getBoundingClientRect();if(!r)return;const z=Math.min(1.2,Math.max(0.35,Math.min((r.width-100)/(maxX-minX+1),(r.height-100)/(maxY-minY+1))));zoom.value=z;pan.x=50-minX*z;pan.y=50-minY*z;}
function autoLayout(){pushHistory();const indeg=new Map<string,number>(),adj=new Map<string,string[]>();workflow.nodes.forEach(n=>indeg.set(n.id,0));workflow.edges.forEach(e=>{indeg.set(e.target,(indeg.get(e.target)||0)+1);const a=adj.get(e.source)||[];a.push(e.target);adj.set(e.source,a);});let current=workflow.nodes.filter(n=>(indeg.get(n.id)||0)===0).map(n=>n.id),level=0,seen=0;while(current.length){const next:string[]=[];current.forEach((id,i)=>{const n=workflow.nodes.find(x=>x.id===id);if(n)n.position={x:100+level*260,y:90+i*140};seen++;for(const t of adj.get(id)||[]){indeg.set(t,(indeg.get(t)||0)-1);if(indeg.get(t)===0)next.push(t);}});current=next;level++;}if(seen!==workflow.nodes.length){ElMessage.error("存在循环依赖，无法自动布局");return;}markDirty();pushHistory();fitView();}
function onPaletteDragStart(type:string,e:DragEvent){draggedPaletteType=type;if(e.dataTransfer)e.dataTransfer.effectAllowed="copy";}
function onCanvasDrop(e:DragEvent){if(!draggedPaletteType||!canvasRef.value)return;const p=canvasCoordinates(e.clientX,e.clientY);addNode(draggedPaletteType,{x:Math.max(0,p.x-90),y:Math.max(0,p.y-42)});draggedPaletteType="";}

function addTrigger(){pushHistory();workflow.triggers.push({id:`trigger-${Date.now().toString(36)}`,type:"event",eventType:"",enabled:true,config:{}});markDirty();pushHistory();}
function removeTrigger(index:number){pushHistory();workflow.triggers.splice(index,1);markDirty();pushHistory();}
function isEventTrigger(type:string){return type === "event";}
function eventTriggerPreset(trigger:WorkflowTrigger){const eventType=String(trigger.eventType||"");switch(eventType){case "device.android.intent":return "android_intent";case "device.android.tasker":return "tasker";case "voice.wake.detected":return "voice_wake";case "voice.asr.final":return "voice_phrase";case "device.app.foreground":return "app_foreground";case "device.notification.posted":case "device.notification.removed":return "notification";case "device.power.battery_changed":return "battery";case "device.network.changed":case "device.network.available":case "device.network.lost":return "network";case "device.app.installed":case "device.app.updated":case "device.app.removed":case "device.app.self_updated":return "package_event";case "device.bluetooth.state_changed":case "device.bluetooth.connected":case "device.bluetooth.disconnected":case "device.ble.characteristic_changed":return "bluetooth";case "device.location.geofence.enter":case "device.location.geofence.exit":return "geofence";default:return androidSystemEventOptions.some(item=>item.value===eventType)?"system_event":"advanced";}}
function triggerCapabilityId(preset:string){switch(preset){case "android_intent":return "workflow.trigger.android_intent.v1";case "tasker":return "workflow.trigger.tasker.v1";case "voice_wake":return "workflow.trigger.voice_wake.v1";case "voice_phrase":return "workflow.trigger.voice_phrase.v1";case "app_foreground":return "workflow.trigger.app_foreground.v1";case "notification":return "workflow.trigger.notification.v1";case "battery":case "package_event":case "system_event":return "workflow.trigger.system_event.v1";case "network":return "workflow.trigger.network.v1";case "bluetooth":return "workflow.trigger.bluetooth.v1";case "geofence":return "workflow.trigger.location.v1";default:return "";}}
function capabilityForPreset(preset:string){const id=triggerCapabilityId(preset);return id?triggerCapabilities.value.find(item=>item.id===id):undefined;}
function canUseDeviceTrigger(preset:string){if(preset==="advanced")return true;if(isCloudWorkflow)return false;const item=capabilityForPreset(preset);return !!item?.supported;}
function triggerCapabilityLabel(trigger:WorkflowTrigger){if(isCloudWorkflow)return "仅本地";const item=capabilityForPreset(eventTriggerPreset(trigger));if(!item)return "状态未知";if(!item.supported)return "不支持";if(item.available)return "可用";if(item.permissionRequired)return "需要权限";return "暂不可用";}
function triggerCapabilityTagType(trigger:WorkflowTrigger):"success"|"warning"|"danger"|"info"{if(isCloudWorkflow)return "danger";const item=capabilityForPreset(eventTriggerPreset(trigger));if(!item)return "info";if(!item.supported)return "danger";return item.available?"success":"warning";}
function triggerCapabilityReason(trigger:WorkflowTrigger){if(isCloudWorkflow)return "设备触发器必须安装到当前设备或指定远程设备，Cloud Core 不参与实时本地触发。";const item=capabilityForPreset(eventTriggerPreset(trigger));if(!item)return "未取得目标设备 Capability 状态。";if(item.available)return "目标设备当前已具备运行条件。";return item.reason||item.permission||"目标设备当前不可用。";}
async function generateTaskerSecret(trigger:WorkflowTrigger){if(!canUseDeviceTrigger("tasker")){ElMessage.warning(triggerCapabilityReason(trigger));return;}taskerSecretBusy.value=trigger.id;try{const value=await createWorkflowTaskerSecret(workflowTarget);trigger.config.secretRef=value.secretRef;markDirty();await ElMessageBox.alert(`Secret：${value.secret}\n\nAction：com.amitia.workflow.TASKER\nSecret 只会显示这一次，请立即保存到 Tasker。`,"Tasker Secret",{confirmButtonText:"我已保存",closeOnClickModal:false,closeOnPressEscape:false});}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"Tasker Secret 生成失败");}finally{taskerSecretBusy.value="";}}
async function createWakeConfig(trigger:WorkflowTrigger){
  if(!canUseDeviceTrigger("voice_wake")){ElMessage.warning(triggerCapabilityReason(trigger));return;}
  try{
    const phraseResult=await ElMessageBox.prompt("输入需要识别的唤醒短语。声音能量只用于切段，不会直接触发。","创建 Wake Config",{inputPlaceholder:"例如：你好 Amitia",confirmButtonText:"下一步",cancelButtonText:"取消",inputValidator:(value)=>String(value||"").trim().length>0||"唤醒短语不能为空"});
    const phrase=String(phraseResult.value||"").trim();
    let backend:"local"|"cloud"="local";
    try{
      await ElMessageBox.confirm("本地唤醒不需要 API Key，模型在设备 Runtime 内推理；如果尚未安装本地 KWS 模型，可改用云 ASR。","选择语音引擎",{confirmButtonText:"本地唤醒",cancelButtonText:"云 ASR",distinguishCancelAndClose:true,type:"info"});
    }catch(choice:any){
      if(choice==="cancel") backend="cloud"; else throw choice;
    }
    const nameResult=await ElMessageBox.prompt("给这组唤醒短语命名。","Wake Config 名称",{inputValue:phrase,inputPlaceholder:"例如：Amitia 唤醒",confirmButtonText:"创建",cancelButtonText:"取消",inputValidator:(value)=>String(value||"").trim().length>0||"名称不能为空"});
    const item=await createWorkflowWakeConfig(workflowTarget,{name:String(nameResult.value||"").trim(),phrases:[phrase],locale:"zh-CN",threshold:0.85,cooldownMs:2000,backend});
    triggerWakeConfigs.value=[...triggerWakeConfigs.value.filter(existing=>existing.id!==item.id),item];
    trigger.config.wakeConfigId=item.id;markDirty();
    triggerCapabilities.value=await getWorkflowTriggerCapabilities(workflowTarget).catch(()=>triggerCapabilities.value);
    ElMessage.success(`${backend==="local"?"本地":"云 ASR"} Wake Config 已创建并绑定`);
  }catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"Wake Config 创建失败");}
}
function applyEventTriggerPreset(trigger:WorkflowTrigger,value:unknown){const preset=String(value||"advanced");if(preset!=="advanced"&&!canUseDeviceTrigger(preset)){ElMessage.warning(isCloudWorkflow?"设备触发器不能安装到 Cloud Workflow":"目标设备不支持该触发器");return;}trigger.config={};switch(preset){case "android_intent":trigger.eventType="device.android.intent";trigger.config={actions:[],categories:[],dataSchemes:[],mimeTypes:[],dedupWindowMs:2000};break;case "tasker":trigger.eventType="device.android.tasker";trigger.config={eventName:"",secretRef:"",allowedVariables:[]};break;case "voice_wake":trigger.eventType="voice.wake.detected";trigger.config={mode:"wake",wakeConfigId:""};break;case "voice_phrase":trigger.eventType="voice.asr.final";trigger.config={mode:"phrase",phrases:[],matchMode:"normalized"};break;case "app_foreground":trigger.eventType="device.app.foreground";trigger.config={packages:[],cooldownMs:30000};break;case "notification":trigger.eventType="device.notification.posted";trigger.config={packages:[],titleContains:"",textContains:"",channelIds:[],categories:[]};break;case "battery":trigger.eventType="device.power.battery_changed";trigger.config={minPercent:0,maxPercent:100};break;case "network":trigger.eventType="device.network.changed";trigger.config={transports:[]};break;case "package_event":trigger.eventType="device.app.installed";trigger.config={packages:[]};break;case "bluetooth":trigger.eventType="device.bluetooth.state_changed";trigger.config={};break;case "geofence":trigger.eventType="device.location.geofence.enter";trigger.config={fenceIds:[]};break;case "system_event":trigger.eventType=androidSystemEventOptions[0]?.value||"device.power.battery_low";trigger.config={};break;default:trigger.eventType="";trigger.config={};}markDirty();}
function simpleCronTime(trigger:WorkflowTrigger): string { const cron=String(trigger.config?.cronExpression||"").trim();const match=cron.match(/^(\d{1,2})\s+(\d{1,2})\s+\*\s+\*\s+\*$/);if(!match)return "08:00";const minute=Math.max(0,Math.min(59,Number(match[1]))),hour=Math.max(0,Math.min(23,Number(match[2])));return `${String(hour).padStart(2,"0")}:${String(minute).padStart(2,"0")}`;}
function applySimpleCronTime(trigger:WorkflowTrigger,value:unknown){const text=String(value||"").trim();const match=text.match(/^(\d{2}):(\d{2})$/);if(!match)return;trigger.config ||= {};trigger.config.type="cron";trigger.config.cronExpression=`${Number(match[2])} ${Number(match[1])} * * *`;trigger.config.timezone ||= Intl.DateTimeFormat().resolvedOptions().timeZone||"UTC";trigger.config.misfirePolicy ||= "fire_once";trigger.config.overlapPolicy ||= "forbid";markDirty();}
function normalizeTrigger(trigger:WorkflowTrigger){trigger.config ||= {};if(trigger.type==="event"){const preset=eventTriggerPreset(trigger);if(preset==="android_intent"){trigger.config.actions ||= [];trigger.config.categories ||= [];trigger.config.dataSchemes ||= [];trigger.config.mimeTypes ||= [];trigger.config.dedupWindowMs ??= 2000;}if(preset==="tasker"){trigger.config.eventName ||= "";trigger.config.secretRef ||= "";trigger.config.allowedVariables ||= [];}if(preset==="voice_wake"){trigger.config.mode="wake";trigger.config.wakeConfigId ||= "";}if(preset==="voice_phrase"){trigger.config.mode="phrase";trigger.config.phrases ||= [];trigger.config.matchMode ||= "normalized";}if(preset==="app_foreground"){trigger.config.packages ||= [];trigger.config.cooldownMs ??= 30000;}if(preset==="notification"){trigger.config.packages ||= [];trigger.config.titleContains ||= "";trigger.config.textContains ||= "";trigger.config.channelIds ||= [];trigger.config.categories ||= [];}if(preset==="battery"){trigger.config.minPercent ??= 0;trigger.config.maxPercent ??= 100;}if(preset==="network"){trigger.config.transports ||= [];}if(preset==="package_event"){trigger.config.packages ||= [];}if(preset==="bluetooth"&&trigger.eventType==="device.ble.characteristic_changed"){trigger.config.sessionId ||= "";trigger.config.address ||= "";trigger.config.serviceUuid ||= "";trigger.config.characteristicUuid ||= "";}if(preset==="geofence"){trigger.config.fenceIds ||= [];}}if(trigger.type==="cron"){trigger.config.type="cron";trigger.config.cronExpression ||= "0 8 * * *";trigger.config.timezone ||= Intl.DateTimeFormat().resolvedOptions().timeZone||"UTC";trigger.config.misfirePolicy ||= "fire_once";trigger.config.maxCatchUp ??= 3;trigger.config.overlapPolicy ||= "forbid";trigger.config.dstSpringPolicy ||= "skip";trigger.config.dstFallPolicy ||= "fire_once_first";}if(trigger.type==="interval"){trigger.config.type="interval";trigger.config.intervalSeconds ||= 3600;trigger.config.misfirePolicy ||= "fire_once";trigger.config.maxCatchUp ??= 3;trigger.config.overlapPolicy ||= "forbid";}if(trigger.type==="one_shot"){trigger.config.type="one_shot";trigger.config.runAt ||= new Date(Date.now()+3600000).toISOString();trigger.config.misfirePolicy ||= "fire_once";trigger.config.overlapPolicy ||= "forbid";}markDirty();}
function normalizeBluetoothTrigger(trigger:WorkflowTrigger){trigger.config={};if(trigger.eventType==="device.ble.characteristic_changed")trigger.config={sessionId:"",address:"",serviceUuid:"",characteristicUuid:""};markDirty();}
function clearTriggerConfigKey(trigger:WorkflowTrigger,key:string){delete trigger.config[key];markDirty();}

async function validateCurrent(showSuccess=true){if(!applyNodeEditors()||!applyEdgeEditor()||!applySchemaEditors())return false;if(workflowTarget.location==="device")return true;try{const result=await validateWorkflow(JSON.parse(JSON.stringify(workflow)),workflowTarget);preflightReport.value=result.preflight||null;if(showSuccess)ElMessage.success(`DAG 校验通过，拓扑节点 ${result.topologicalOrder?.length||0} 个`);return result.valid!==false;}catch(e:any){if(showSuccess)ElMessage.error(e?.response?.data?.error || e?.message || "工作流校验失败");return false;}}
async function runPreflight(){if(workflowTarget.location==="device"){ElMessage.info("远程设备工作流由设备端保存/预检");return;}const ok=await validateCurrent(false);preflightDialogVisible.value=true;if(!preflightReport.value){ElMessage.error("未取得预检报告");return;}if(ok&&preflightReport.value.status==="PASS")ElMessage.success("预检通过");else if(preflightReport.value.runnable)ElMessage.warning("预检通过，但存在需要处理的警告");else ElMessage.error("预检存在阻断项");}
function hasEnabledDeviceTrigger(){return workflow.enabled!==false&&workflow.triggers.some(trigger=>trigger.enabled!==false&&trigger.type==="event"&&deviceWorkflowEventTypes.has(String(trigger.eventType||"")));}
function hasHighImpactWorkflowNodes(){return workflow.hasSideEffects===true||workflow.nodes.some(node=>["tool","task","mcp","javascript","wasm","trusted_service"].includes(String(node.type||"").toLowerCase()));}
async function confirmDeviceTriggerRisk(){if(!hasEnabledDeviceTrigger()||!hasHighImpactWorkflowNodes())return true;try{await ElMessageBox.confirm("此工作流包含可产生副作用或调用外部能力的节点，并启用了设备自动触发器。触发条件满足时可能在无人交互情况下执行。请确认触发来源、权限、输入过滤与幂等策略均符合预期。","确认设备自动执行",{type:"warning",confirmButtonText:"确认并保存",cancelButtonText:"取消"});return true;}catch{return false;}}
async function save(){if(saving.value)return;if(!(await validateCurrent(false)))return;if(!(await confirmDeviceTriggerRisk()))return;saving.value=true;try{const updated=await updateWorkflow(workflow.id,JSON.parse(JSON.stringify(workflow)),workflowTarget);Object.assign(workflow,updated);dirty.value=false;pushHistory();await refreshRevisions();ElMessage.success("工作流已保存");}finally{saving.value=false;}}
function executionModeLabel(mode?: string){switch(mode){case "dry_run":return "Dry Run";case "mocked":return "Mocked";case "controlled_live":return "Controlled Live";default:return "Live";}}
function executionModeDescription(mode: WorkflowExecutionMode){switch(mode){case "dry_run":return "执行路由、依赖、条件、权限和预检，不调用真实节点 Handler。";case "mocked":return "使用节点 Mock 输出；副作用节点必须显式 Mock，避免误执行真实操作。";case "controlled_live":return "按真实链路执行，但所有副作用节点必须在执行前获得确认。";default:return "按正式运行语义执行节点和副作用。";}}
function openRunDialog(){runInputEditor.value="{}";runMocksEditor.value="[]";runDialogVisible.value=true;}
function parseRunJSON<T>(text:string,label:string):T{try{return JSON.parse(text||"null") as T;}catch(e:any){throw new Error(`${label} JSON 无效：${e?.message||e}`);}}
async function startRun(){if(dirty.value){await save();if(dirty.value)return;}running.value=true;try{const input=parseRunJSON<unknown>(runInputEditor.value,"Input");let mocks:WorkflowMockBehavior[]|undefined;if(runMode.value==="mocked"){const parsed=parseRunJSON<unknown>(runMocksEditor.value,"Mocks");if(!Array.isArray(parsed))throw new Error("Mocks 必须是 JSON Array");mocks=parsed as WorkflowMockBehavior[];}const res=await runWorkflow(workflow.id,input,false,workflowTarget,{mode:runMode.value,mocks});currentRun.value={executionId:res.executionId,workflowId:workflow.id,status:res.status||"running",context:{executionOptions:{mode:res.executionMode||runMode.value,mocks}}};pendingConfirmations.value=res.requiredConfirmations||[];runDialogVisible.value=false;inspectorTab.value="runs";if(res.status==="waiting_confirmation"){ElMessage.warning(`等待确认 ${pendingConfirmations.value.length} 个副作用节点`);return;}ElMessage.success(runMode.value==="dry_run"?`Dry Run 已完成：${res.executionId}`:`已开始运行：${res.executionId}`);startPolling(res.executionId);}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"运行失败");}finally{running.value=false;}}
function applyRunDetail(data: Awaited<ReturnType<typeof getWorkflowRun>>){currentRun.value=data.run;currentRunError.value=data.classifiedError||null;stepRuns.value=data.stepRuns||[];stepAttempts.value=data.attempts||[];checkpoints.value=data.checkpoints||[];pendingConfirmations.value=data.requiredConfirmations||[];}
function runStopsPolling(status:string){return ["succeeded","failed","cancelled","compensated","compensation_failed","manual_intervention_required","cancel_timeout","cancel_failed","dropped","waiting_confirmation"].includes(status);}
function startPolling(runId:string){if(pollTimer)clearInterval(pollTimer);const tick=async()=>{try{const data=await getWorkflowRun(runId,workflowTarget);applyRunDetail(data);if(runStopsPolling(data.run.status)){if(pollTimer)clearInterval(pollTimer);pollTimer=undefined;await refreshObservability();}}catch{/* run can take a moment to be persisted */}};void tick();pollTimer=window.setInterval(tick,800);}
async function refreshRevisions(){if(workflowTarget.location==="device"){revisions.value=[];return;}try{revisions.value=await listWorkflowRevisions(workflowId,50,workflowTarget);}catch{revisions.value=[];}}
async function manualSnapshot(){if(dirty.value){ElMessage.warning("请先保存当前修改，再创建版本快照");return;}revisionBusy.value=true;try{const {value}=await ElMessageBox.prompt("可选：给这个快照加一个备注。","保存版本快照",{inputPlaceholder:"例如：调整天气分支前",confirmButtonText:"保存",cancelButtonText:"取消"});await createWorkflowRevision(workflowId,String(value||"").trim(),workflowTarget);await refreshRevisions();ElMessage.success("版本快照已保存");}catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"保存快照失败");}finally{revisionBusy.value=false;}}
async function rollbackRevision(item:WorkflowRevisionSummary){try{await ElMessageBox.confirm(`回滚到版本 #${item.revisionNo}？当前状态会先自动保存，回滚会恢复当时的 DAG、触发器和设置。`,"版本回滚",{type:"warning",confirmButtonText:"回滚",cancelButtonText:"取消"});revisionBusy.value=true;const restored=await rollbackWorkflowRevision(workflowId,item.revisionId,workflowTarget,workflow.installation?.revision);Object.assign(workflow,restored);normalizeLoadedDefinition();dirty.value=false;history.value=[modelSnapshot()];historyIndex.value=0;await Promise.all([refreshRevisions(),refreshRuns()]);ElMessage.success(`已回滚到版本 #${item.revisionNo}`);}catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"回滚失败");}finally{revisionBusy.value=false;}}
async function refreshRuns(){try{const data=await listWorkflowRuns(workflow.id,40,workflowTarget);runs.value=data.items||[];}catch{runs.value=[];}}
async function refreshObservability(){if(workflowTarget.location==="device"){executionStats.value=null;await refreshRuns();return;}await Promise.all([refreshRuns(), getWorkflowStats(workflow.id,workflowTarget).then(v=>executionStats.value=v).catch(()=>executionStats.value=null)]);}
async function refreshSafety(){if(workflowTarget.location==="device"){safetyAnalysis.value=null;return;}try{safetyAnalysis.value=await getWorkflowAnalysis(workflow.id,workflowTarget);}catch{safetyAnalysis.value=null;}}
async function openRun(runId:string){const data=await getWorkflowRun(runId,workflowTarget);applyRunDetail(data);if(!runStopsPolling(data.run.status))startPolling(runId);}
async function confirmCurrent(){if(!currentRun.value||!canConfirm.value)return;const runId=currentRun.value.executionId;try{const result=await confirmWorkflowRun(runId,pendingConfirmations.value,workflowTarget);pendingConfirmations.value=result.missingConfirmations||[];if(result.run)currentRun.value=result.run;if(pendingConfirmations.value.length){ElMessage.warning(`仍有 ${pendingConfirmations.value.length} 个副作用节点未确认`);return;}ElMessage.success("已确认，继续原运行");startPolling(runId);}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"确认失败");}}
async function pauseCurrent(){if(!currentRun.value)return;await pauseWorkflowRun(currentRun.value.executionId,"Paused from Creative Workshop",workflowTarget);startPolling(currentRun.value.executionId);}
async function resumeCurrent(){if(!currentRun.value)return;await resumeWorkflowRun(currentRun.value.executionId,workflowTarget);startPolling(currentRun.value.executionId);}
async function recoverCurrent(){if(!currentRun.value||!canRecover.value)return;try{await ElMessageBox.confirm(`将复用当前运行的 ${checkpoints.value.length} 个 Checkpoint，并按当前已保存工作流继续执行。不会单独绕过 DAG 重跑某个节点。`,`从 Checkpoint 恢复`,{type:"warning",confirmButtonText:"恢复",cancelButtonText:"取消"});const res=await recoverWorkflowRun(currentRun.value.executionId,workflowTarget);ElMessage.success(`已从 ${res.checkpointCount} 个 Checkpoint 恢复执行`);startPolling(res.executionId);}catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"Checkpoint 恢复失败");}}
async function rerunCurrent(){if(!currentRun.value||!canRerun.value)return;try{const source=currentRun.value.executionId;const res=await rerunWorkflowRun(source,false,workflowTarget);currentRun.value={executionId:res.executionId,workflowId:workflow.id,status:res.status||"running",context:{executionOptions:{mode:res.executionMode||"live"}}};stepRuns.value=[];stepAttempts.value=[];checkpoints.value=[];pendingConfirmations.value=res.requiredConfirmations||[];if(res.status==="waiting_confirmation"){ElMessage.warning(`新的 Controlled Live 运行需要重新确认 ${pendingConfirmations.value.length} 个副作用节点`);return;}ElMessage.success("已使用原运行输入和执行模式重新执行当前已保存工作流");startPolling(res.executionId);}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"重新运行失败");}}
async function cancelCurrent(){if(!currentRun.value)return;await cancelWorkflowRun(currentRun.value.executionId,workflowTarget);startPolling(currentRun.value.executionId);}
function stepStatus(nodeId:string){return stepRuns.value.find(s=>s.nodeId===nodeId)?.status||"";}
function nodeStatusClass(nodeId:string){const s=stepStatus(nodeId);return s?`run-${s}`:"";}
function nodeLabel(id:string){return workflow.nodes.find(n=>n.id===id)?.label||id;}
function formatTime(v?:string){return v?new Date(v).toLocaleString():"";}
function formatDuration(ms:number){if(!Number.isFinite(ms)||ms<=0)return "0ms";if(ms<1000)return `${Math.round(ms)}ms`;if(ms<60000)return `${(ms/1000).toFixed(1)}s`;return `${(ms/60000).toFixed(1)}m`;}
function attemptsForNode(nodeId:string){return stepAttempts.value.filter(item=>item.nodeId===nodeId);}
function riskTagType(level:string){return level==="high"?"danger":level==="medium"?"warning":"success";}
function onInspectorTabChanged(name:string|number){if(String(name)==="runs")void refreshObservability();if(String(name)==="security")void refreshSafety();}
function workflowRevisionOf(def: WorkflowDefinition){return Number(def.installation?.revision||0);}
function syncEventMatches(event:any){
  if(!String(event?.type||"").startsWith("workflow.installation."))return false;
  const payload=event?.payload&&typeof event.payload==="object"?event.payload:{};
  if(String(payload.workflowId||"")!==workflowId)return false;
  if(workflowTarget.location==="device")return String(payload.hostDeviceId||"")===String(workflowTarget.deviceId||"");
  if(workflowTarget.location==="cloud")return String(payload.location||"")==="cloud";
  return true;
}
async function refreshDefinitionFromSync(){
  try{
    const latest=await getWorkflow(workflowId,workflowTarget);
    const latestRevision=workflowRevisionOf(latest);
    const localRevision=workflowRevisionOf(workflow);
    if(dirty.value){
      if(latestRevision>localRevision&&latestRevision!==conflictNoticeRevision){
        conflictNoticeRevision=latestRevision;
        ElMessage.warning(`该工作流已在其他客户端更新到 revision ${latestRevision}。当前草稿未覆盖，保存时会执行冲突校验。`);
      }
      return;
    }
    if(latestRevision&&latestRevision===localRevision)return;
    Object.assign(workflow,latest);normalizeLoadedDefinition();dirty.value=false;history.value=[modelSnapshot()];historyIndex.value=0;conflictNoticeRevision=0;
    await Promise.all([refreshRevisions(),refreshObservability()]);
  }catch{/* transient sync failures are retried */}
}
async function primeBuilderSync(){
  try{const page=await listWorkflowSyncEvents(workflowTarget);syncCursor=page.cursor;}catch{syncCursor=null;}
}
async function pollBuilderSync(){
  if(document.visibilityState!=="visible"||syncBusy)return;
  syncBusy=true;
  try{
    if(syncCursor===null){await primeBuilderSync();return;}
    const page=await listWorkflowSyncEvents(workflowTarget,syncCursor);syncCursor=page.cursor;
    if(page.items.some(syncEventMatches))await refreshDefinitionFromSync();
    deviceSyncTicks++;
    if(workflowTarget.location==="device"&&deviceSyncTicks%8===0)await refreshDefinitionFromSync();
  }catch{/* retain cursor for durable retry */}finally{syncBusy=false;}
}

watch(()=>workflow.name,markDirty);watch(()=>workflow.description,markDirty);
watch(selectedNodeId,()=>{syncNodeEditors();mappingTargetPath.value="";mappingSourceRef.value="";void refreshNestedCandidates();void refreshSelectedToolCatalog();});
watch(selectedEdgeId,()=>{edgeConditionEditor.value=pretty(selectedEdge.value?.condition);});

onMounted(async()=>{
  const [loaded,items,ownedWorkflows,devices,capabilities,appCatalog,wakeConfigs]=await Promise.all([
    getWorkflow(workflowId,workflowTarget),
    getWorkflowCatalog(workflowTarget).catch(()=>[] as WorkflowCatalogItem[]),
    listWorkflows(workflowTarget).catch(()=>[] as WorkflowDefinition[]),
    isCloudWorkflow ? listWorkflowDevices().catch(()=>[] as WorkflowDeviceDescriptor[]) : Promise.resolve([] as WorkflowDeviceDescriptor[]),
    isCloudWorkflow ? Promise.resolve([] as WorkflowTriggerCapabilityStatus[]) : getWorkflowTriggerCapabilities(workflowTarget).catch(()=>[] as WorkflowTriggerCapabilityStatus[]),
    isCloudWorkflow ? Promise.resolve([] as WorkflowTriggerAppCatalogItem[]) : getWorkflowTriggerAppCatalog(workflowTarget).catch(()=>[] as WorkflowTriggerAppCatalogItem[]),
    isCloudWorkflow ? Promise.resolve([] as WorkflowTriggerWakeConfigItem[]) : getWorkflowTriggerWakeConfigs(workflowTarget).catch(()=>[] as WorkflowTriggerWakeConfigItem[]),
  ]);
  Object.assign(workflow,loaded);catalog.value=items;workflowDevices.value=devices;triggerCapabilities.value=capabilities;triggerAppCatalog.value=appCatalog;triggerWakeConfigs.value=wakeConfigs;nestedWorkflowCandidates.value=ownedWorkflows.filter(item=>item.id!==workflowId);normalizeLoadedDefinition();dirty.value=false;history.value=[modelSnapshot()];historyIndex.value=0;await Promise.all([refreshObservability(),refreshRevisions(),primeBuilderSync()]);
  const requestedRunId=String(route.query.runId||"").trim();if(requestedRunId){inspectorTab.value="runs";try{await openRun(requestedRunId);}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"运行详情加载失败");}}
  syncTimer=window.setInterval(()=>{void pollBuilderSync();},2000);setTimeout(fitView,0);
});
onBeforeUnmount(()=>{if(pollTimer)clearInterval(pollTimer);if(syncTimer)clearInterval(syncTimer);window.removeEventListener("pointermove",onGlobalPointerMove);});
</script>

<style scoped>
.builder-page{height:100%;min-height:0;display:flex;flex-direction:column;color:var(--console-text);overflow:hidden}.builder-header{display:flex;align-items:center;justify-content:space-between;gap:18px;padding:0 0 14px;border-bottom:1px solid var(--console-border)}.title-area{min-width:0;flex:1}.back-link{display:inline-block;margin-bottom:5px;color:var(--console-text-muted);font-size:12px;text-decoration:none}.title-row{display:flex;align-items:center;gap:10px}.name-input{max-width:420px}.name-input :deep(.el-input__wrapper),.description-input :deep(.el-input__wrapper){box-shadow:none;background:transparent;padding-left:0}.name-input :deep(.el-input__inner){font-size:20px;font-weight:650}.description-input{max-width:560px}.description-input :deep(.el-input__wrapper){height:30px}.description-input :deep(.el-input__inner){font-size:12px;color:var(--console-text-muted)}.dirty-dot,.saved-state{font-size:11px;white-space:nowrap}.dirty-dot{color:var(--el-color-warning)}.saved-state{color:var(--console-text-muted)}.target-badge{color:var(--el-color-primary);padding:2px 6px;border:1px solid var(--console-border);border-radius:6px}.header-actions{display:flex;gap:7px;align-items:center}.builder-layout{flex:1;height:calc(100% - 78px);min-height:0;display:grid;grid-template-columns:210px minmax(0,1fr) 310px}.palette-panel,.inspector-panel{min-height:0;background:var(--ac-color-surface)}.palette-panel{display:flex;flex-direction:column;overflow:hidden;padding:0;border-right:1px solid var(--console-border)}.palette-top{flex-shrink:0;padding:14px 14px 0}.palette-items{flex:1;min-height:0;overflow:auto;padding:10px 14px 0}.palette-bottom{flex-shrink:0;padding:10px 14px 14px}.inspector-panel{overflow:auto;border-left:1px solid var(--console-border);padding:0 12px 16px}.inspector-panel :deep(.el-tabs__header){position:sticky;top:0;z-index:2;background:var(--ac-color-surface)}.panel-title{font-size:13px;font-weight:650;margin-bottom:9px}.panel-title.small{font-size:12px}.panel-tip{margin:0 0 12px;color:var(--console-text-muted);font-size:11px;line-height:1.5}.palette-item{width:100%;display:grid;grid-template-columns:34px 1fr;align-items:center;text-align:left;gap:9px;border:1px solid transparent;border-radius:9px;background:transparent;color:var(--console-text);padding:8px;cursor:grab}.palette-item:hover{background:var(--ac-color-surface-soft);border-color:var(--console-border)}.palette-item strong,.palette-item small{display:block}.palette-item strong{font-size:12px}.palette-item small{font-size:10px;color:var(--console-text-muted);margin-top:2px}.node-type-icon{width:32px;height:32px;border-radius:8px;background:var(--ac-color-surface-soft);display:grid;place-items:center;color:var(--el-color-primary);font-size:11px;font-weight:700}.palette-divider{height:1px;background:var(--console-border);margin:13px 0}.canvas-tools.vertical{display:grid;grid-template-columns:1fr 1fr;gap:6px}.canvas-tools :deep(.el-button){margin-left:0}.workflow-canvas{position:relative;min-width:0;min-height:0;overflow:hidden;background:var(--ac-color-page,#0f0f10);touch-action:none}.canvas-grid{position:absolute;inset:0;background-image:radial-gradient(circle,var(--console-border) 1px,transparent 1px);opacity:.55}.graph-transform{position:absolute;left:0;top:0;width:4000px;height:2600px;transform-origin:0 0}.edge-layer{position:absolute;inset:0;width:4000px;height:2600px;overflow:visible}.edge-line{fill:none;stroke:var(--console-text-muted);stroke-width:1.5;opacity:.72}.edge-line.selected{stroke:var(--el-color-primary);stroke-width:2.5;opacity:1}.edge-line.preview{stroke:var(--el-color-primary);stroke-dasharray:6 4}.edge-hit{fill:none;stroke:transparent;stroke-width:14;cursor:pointer}.edge-label{font-size:11px;fill:var(--console-text-muted);text-anchor:middle}.workflow-node{position:absolute;width:180px;height:84px;border:1px solid var(--console-border);border-radius:11px;background:var(--ac-color-surface);box-shadow:0 5px 18px rgba(0,0,0,.12);user-select:none}.workflow-node.selected{border-color:var(--el-color-primary);box-shadow:0 0 0 1px var(--el-color-primary)}.workflow-node.run-running{border-color:var(--el-color-warning)}.workflow-node.run-succeeded,.workflow-node.run-defaulted{border-color:var(--el-color-success)}.workflow-node.run-failed{border-color:var(--el-color-danger)}.workflow-node.run-skipped{opacity:.58}.node-header{height:48px;display:grid;grid-template-columns:32px 1fr 18px;gap:8px;align-items:center;padding:7px 9px;cursor:grab}.node-badge{width:30px;height:30px;border-radius:8px;background:var(--ac-color-surface-soft);display:grid;place-items:center;color:var(--el-color-primary);font-size:10px;font-weight:700}.node-title{min-width:0}.node-title strong,.node-title small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.node-title strong{font-size:12px}.node-title small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.node-menu{font-size:14px;color:var(--console-text-muted);cursor:pointer}.node-body{height:35px;border-top:1px solid var(--console-border);display:flex;align-items:center;justify-content:space-between;gap:6px;padding:0 9px;font-size:9px}.target-text{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.muted{color:var(--console-text-muted)}.status-pill{font-size:8px;text-transform:uppercase;color:var(--el-color-primary)}.input-handle,.output-handle{position:absolute;top:34px;width:14px;height:14px;border-radius:50%;border:2px solid var(--ac-color-surface);background:var(--console-text-muted);padding:0;cursor:crosshair;z-index:3}.input-handle{left:-8px}.output-handle{right:-8px;background:var(--el-color-primary)}.zoom-indicator{position:absolute;left:12px;bottom:12px;padding:5px 8px;border:1px solid var(--console-border);border-radius:7px;background:var(--ac-color-surface);font-size:10px;color:var(--console-text-muted)}.minimap{position:absolute;right:12px;bottom:12px;width:150px;height:94px;border:1px solid var(--console-border);border-radius:8px;background:var(--ac-color-surface);overflow:hidden;opacity:.88}.minimap svg{width:100%;height:100%}.mini-node{fill:var(--console-text-muted);opacity:.55}.connect-hint{position:absolute;left:50%;bottom:14px;transform:translateX(-50%);padding:7px 11px;border-radius:8px;background:var(--ac-color-surface);border:1px solid var(--el-color-primary);font-size:10px}.inspector-content{display:flex;flex-direction:column;gap:11px;padding:4px 3px}.inspector-content label{display:flex;flex-direction:column;gap:5px;font-size:11px;color:var(--console-text-muted)}.inspector-content :deep(.el-select),.inspector-content :deep(.el-input-number){width:100%}.reliability-card{display:flex;flex-direction:column;gap:9px;padding:10px;border:1px solid var(--console-border);border-radius:9px;background:var(--ac-color-surface-soft)}.inline-switch{flex-direction:row!important;align-items:center;justify-content:space-between}.empty-inspector{padding:30px 4px;text-align:center;color:var(--console-text-muted);font-size:12px}.edge-summary,.definition-meta{padding:8px;border-radius:8px;background:var(--ac-color-surface-soft);font-size:10px;color:var(--console-text-muted);word-break:break-all}.panel-row,.trigger-head,.current-run-card,.run-history{display:flex;align-items:center;justify-content:space-between;gap:8px}.trigger-card{display:flex;flex-direction:column;gap:9px;padding:10px;border:1px solid var(--console-border);border-radius:10px}.trigger-head strong{font-size:11px}.trigger-capability{display:flex;align-items:flex-start;gap:7px;padding:7px;border-radius:7px;background:var(--ac-color-surface-soft);font-size:9px;color:var(--console-text-muted);line-height:1.45}.tasker-secret-row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:6px;align-items:center}.current-run-card{align-items:flex-start;padding:10px;border-radius:9px;background:var(--ac-color-surface-soft)}.current-run-card strong,.current-run-card small{display:block}.run-error-diagnostic{margin-top:7px;padding:7px;border:1px solid var(--el-color-danger-light-7);border-radius:7px}.run-error-diagnostic span{display:block;margin-top:4px;font-size:9px;color:var(--el-color-danger)}.current-run-card small{font-size:9px;color:var(--console-text-muted);word-break:break-all;margin-top:3px}.run-actions{display:flex;gap:4px;flex-wrap:wrap;justify-content:flex-end}.trace-step{display:grid;grid-template-columns:10px 1fr;gap:8px;padding:8px;border:1px solid var(--console-border);border-radius:8px;cursor:pointer}.trace-status{width:8px;height:8px;border-radius:50%;margin-top:4px;background:var(--console-text-muted)}.trace-status.s-succeeded,.trace-status.s-defaulted{background:var(--el-color-success)}.trace-status.s-failed{background:var(--el-color-danger)}.trace-status.s-running{background:var(--el-color-warning)}.trace-step strong,.trace-step small{display:block}.trace-step strong{font-size:11px}.trace-step small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.trace-step p{margin:4px 0 0;color:var(--el-color-danger);font-size:9px;word-break:break-word}.run-history-title{margin-top:8px}.run-history{padding:8px;border-bottom:1px solid var(--console-border);cursor:pointer}.run-history strong,.run-history small{display:block;font-size:10px}.run-history small,.run-history>span{color:var(--console-text-muted);font-size:9px}.mapping-list{display:flex;flex-direction:column;gap:6px}.mapping-row{display:flex;align-items:center;justify-content:space-between;gap:6px;padding:8px;border:1px solid var(--console-border);border-radius:8px}.mapping-row span{min-width:0}.mapping-row strong,.mapping-row small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.mapping-row strong{font-size:10px}.mapping-row small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.empty-inspector.compact{padding:12px 4px}.ai-actions{display:flex;gap:6px;flex-wrap:wrap}.ai-result{padding:10px;border:1px solid var(--console-border);border-radius:9px;background:var(--ac-color-surface-soft);font-size:10px}.ai-result strong{font-size:11px}.ai-result p{margin:6px 0;color:var(--console-text-muted);line-height:1.5}.ai-result ul{margin:6px 0 0;padding-left:18px;line-height:1.5}.revision-card{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:9px;border:1px solid var(--console-border);border-radius:8px}.revision-main{min-width:0}.revision-main strong,.revision-main small,.revision-main span{display:block}.revision-main strong{font-size:10px}.revision-main small,.revision-main span{font-size:9px;color:var(--console-text-muted);margin-top:2px}.revision-main span{font-family:monospace}
.stats-strip{display:grid;grid-template-columns:repeat(4,1fr);gap:6px}.stats-strip span{display:flex;flex-direction:column;gap:2px;padding:7px;border:1px solid var(--console-border);border-radius:8px;background:var(--ac-color-surface-soft);text-align:center}.stats-strip strong{font-size:11px}.stats-strip small{font-size:8px;color:var(--console-text-muted)}.attempt-list{display:flex;flex-direction:column;gap:2px;margin-top:5px}.attempt-list span{font-size:8px;color:var(--console-text-muted)}.attempt-list .attempt-timed_out,.attempt-list .attempt-failed{color:var(--el-color-danger)}.attempt-list .attempt-succeeded{color:var(--el-color-success)}.checkpoint-badge{font-style:normal;font-size:8px;padding:1px 4px;border-radius:4px;background:var(--el-color-success-light-9);color:var(--el-color-success)}.security-section{display:flex;flex-direction:column;gap:6px;padding:9px;border:1px solid var(--console-border);border-radius:8px}.security-section>strong{font-size:10px}.security-section>span{font-size:9px;color:var(--console-text-muted)}.security-section code{font-size:9px;word-break:break-all}.dependency-row,.risk-row{display:flex;align-items:flex-start;justify-content:space-between;gap:6px;font-size:9px}.risk-row{justify-content:flex-start;line-height:1.5}.risk-row span{color:var(--console-text-muted)}
.builder-header{align-items:flex-start}.title-row{align-items:center;flex-wrap:wrap}.title-row nav{min-width:0}.builder-breadcrumb-list{display:flex;align-items:center;gap:8px;min-width:0;margin:0;padding:0;list-style:none}.breadcrumb-link,.breadcrumb-separator{font-size:24px;line-height:32px}.breadcrumb-link{position:relative;display:inline-flex;align-items:center;color:var(--ac-color-primary);font-weight:600;text-decoration:none;transition:color 180ms ease}.breadcrumb-link::before{position:absolute;inset:-6px 0;content:""}.breadcrumb-link:hover{color:var(--el-color-primary-light-3);text-decoration:underline;text-underline-offset:4px}.breadcrumb-link:focus-visible{border-radius:4px;outline:3px solid var(--ac-color-primary);outline-offset:3px}.breadcrumb-separator{color:var(--ac-color-text-muted);font-weight:400}.current-title{display:flex;align-items:center;min-width:160px;max-width:420px;height:32px;overflow-wrap:anywhere}.name-input{width:clamp(160px,24vw,420px);max-width:100%;font-family:inherit}.name-input :deep(.el-input__wrapper){min-height:32px;padding-top:0;padding-bottom:0}.name-input :deep(.el-input__inner){height:32px;color:var(--ac-color-text);font-family:inherit;font-size:24px;line-height:32px;font-weight:600}
.title-row{width:100%}.workflow-status{display:flex;align-items:center;gap:10px;margin-left:auto;font-size:11px;line-height:16px;white-space:nowrap}.workflow-status>span{font-size:inherit;line-height:inherit}
.builder-header{flex-direction:column;align-items:stretch;gap:12px}.title-area,.header-actions{width:100%}.header-actions{justify-content:flex-start;gap:8px;flex-wrap:wrap}.header-actions :deep(.el-button){margin-left:0}.builder-layout{position:relative}
@media(prefers-reduced-motion:reduce){.breadcrumb-link{transition:none}}
@media(max-width:1050px){.builder-layout{grid-template-columns:170px minmax(0,1fr) 270px}}@media(max-width:760px){.builder-header{align-items:flex-start;flex-direction:column}.header-actions{flex-wrap:wrap}.builder-layout{grid-template-columns:1fr}.palette-panel{display:none}.inspector-panel{position:absolute;right:0;top:106px;bottom:0;width:min(86vw,320px);z-index:8;box-shadow:-8px 0 24px rgba(0,0,0,.18)}.workflow-canvas{min-height:600px}}
@media(max-width:760px){.builder-header{align-items:stretch}.inspector-panel{top:0}}
.run-dialog-body{display:flex;flex-direction:column;gap:12px}.run-dialog-body label{display:flex;flex-direction:column;gap:6px;font-size:12px;color:var(--console-text-muted)}.run-mode-tip{margin-bottom:0}.controlled-warning{margin:0;padding:10px;border:1px solid var(--el-color-warning-light-5);border-radius:8px;background:var(--el-color-warning-light-9);font-size:11px;line-height:1.55;color:var(--console-text-muted)}.controlled-warning code{color:var(--el-color-warning-dark-2)}

.tool-catalog-card{border:1px solid var(--el-border-color-lighter);border-radius:10px;padding:10px;background:var(--el-fill-color-extra-light)}
.tool-catalog-head{display:flex;align-items:center;justify-content:space-between;gap:8px}.tool-catalog-card p{margin:6px 0;font-size:12px;line-height:1.5;color:var(--el-text-color-secondary)}
.tool-catalog-meta{display:flex;flex-wrap:wrap;gap:8px;font-size:11px;color:var(--el-text-color-secondary)}.tool-permissions{display:flex;flex-wrap:wrap;gap:5px;margin-top:8px}
.tool-schema-form{display:flex;flex-direction:column;gap:8px;margin-top:10px;padding-top:10px;border-top:1px solid var(--el-border-color-lighter)}.tool-schema-title{font-size:11px;font-weight:600}.tool-schema-field{display:flex;flex-direction:column;gap:4px}.tool-schema-field>span{display:flex;align-items:center;gap:6px;font-size:11px}.tool-schema-field em{font-size:9px;font-style:normal;color:var(--el-color-danger)}.tool-schema-field small{font-size:9px;color:var(--el-text-color-secondary)}
.schema-details{margin-top:8px;font-size:12px}.schema-details pre{max-height:220px;overflow:auto;white-space:pre-wrap;word-break:break-word;background:var(--el-bg-color);padding:8px;border-radius:6px}
.preflight-center{display:flex;flex-direction:column;gap:8px}.preflight-summary{display:flex;align-items:center;gap:8px;padding:10px;border:1px solid var(--console-border);border-radius:8px}.preflight-check{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:8px;padding:9px;border-bottom:1px solid var(--console-border)}.preflight-check strong,.preflight-check small{display:block}.preflight-check strong{font-size:11px}.preflight-check small{margin-top:2px;font-size:9px;color:var(--console-text-muted)}.simple-expression-builder{display:flex;flex-direction:column;gap:8px;padding:10px;border:1px solid var(--console-border);border-radius:8px}.simple-condition-row{display:flex;flex-direction:column;gap:7px;padding:9px;border:1px solid var(--console-border);border-radius:7px}.simple-condition-head{display:flex;align-items:center;gap:8px}.simple-condition-head strong{margin-right:auto;font-size:11px}
.editor-mode-switch{white-space:nowrap}
</style>
