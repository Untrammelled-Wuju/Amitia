<template>
  <div class="ai-char-settings">
    <!-- Header -->
    <div class="page-header">
      <div>
        <h2>角色性格设置</h2>
        <p class="page-desc">调整 AI 角色的回复风格、陪伴方式和安全边界</p>
      </div>
    </div>

    <!-- Top bar: Info + Actions -->
    <div class="top-bar">
      <div class="info-row">
        <div class="info-item">
          <label>角色名称</label>
          <el-input v-model="form.name" placeholder="角色名称" style="width:200px" size="default" />
        </div>
        <div class="info-item">
          <label>角色描述</label>
          <el-input v-model="form.description" placeholder="简要描述" style="width:280px" size="default" />
        </div>
        <div class="info-item">
          <label>默认角色</label>
          <el-switch v-model="form.isDefault" @change="setAsDefault" />
        </div>
        <div class="info-item" v-if="charId">
          <label>角色 ID</label>
          <span class="char-id">{{ charId }}</span>
        </div>
      </div>
      <div class="action-row">
        <el-button @click="editCharPrompt" :loading="promptLoading">
          <el-icon><View /></el-icon> 修改角色提示词
        </el-button>
        <el-button @click="resetConfig" :loading="resetting" type="warning" plain>
          重置默认
        </el-button>
        <el-button type="primary" @click="saveConfig" :loading="saving">
          保存
        </el-button>
      </div>
    </div>

    <!-- ====== Slider Sections ====== -->
    <div class="sections">

      <!-- 0. 角色性别 -->
      <div class="section-card">
        <div class="section-title">角色性别</div>
        <div class="gender-grid">
          <div class="gender-item">
            <label class="gender-label">角色性别</label>
            <el-select v-model="genderForm.gender" placeholder="选择性别" size="default" style="width:100%">
              <el-option v-for="opt in GENDER_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">角色的基础性别画像，不影响聊天功能</span>
          </div>
          <div class="gender-item">
            <label class="gender-label">角色代词</label>
            <el-select v-model="genderForm.pronoun" placeholder="选择代词" size="default" style="width:100%" :disabled="genderForm.gender === 'CUSTOM'">
              <el-option v-for="opt in PRONOUN_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">在对话中引用角色时使用的代词</span>
          </div>
          <div class="gender-item" v-if="genderForm.gender === 'CUSTOM'">
            <label class="gender-label">自定义性别标签</label>
            <el-input v-model="genderForm.genderLabel" placeholder='例如"伙伴""守护者"' size="default" />
            <span class="gender-hint">CUSTOM 模式下可自由定义性别标签</span>
          </div>
          <div class="gender-item" v-if="genderForm.gender === 'CUSTOM'">
            <label class="gender-label">自定义代词</label>
            <el-input v-model="genderForm.pronoun" placeholder='例如"TA""它"' size="default" />
            <span class="gender-hint">CUSTOM 模式下可自由定义代词</span>
          </div>
          <div class="gender-item">
            <label class="gender-label">用户称呼风格</label>
            <el-select v-model="genderForm.userAddressingStyle" placeholder="选择风格" size="default" style="width:100%" clearable>
              <el-option v-for="opt in ADDRESSING_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">角色称呼用户的风格</span>
          </div>
          <div class="gender-item">
            <label class="gender-label">
              性别表达强度
              <span class="gender-value">{{ genderForm.genderExpression }}</span>
            </label>
            <div class="sr-body">
              <span class="sr-left">中性</span>
              <div class="sr-slider-wrap">
                <el-slider v-model="genderForm.genderExpression" :min="0" :max="100" :step="1" class="sr-slider" />
              </div>
              <span class="sr-right">明显</span>
            </div>
            <span class="gender-hint">控制角色在语言风格、生活习惯中体现性别特征的强弱</span>
          </div>
        </div>
      </div>

      <!-- 1. 基础性格 -->
      <div class="section-card">
        <div class="section-title">基础性格</div>
        <div class="slider-grid">
          <SliderRow v-model="form.personalityConfig.familiarity" label="熟悉感" left="客气" right="熟悉" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.formality" label="正式度" left="口语" right="正式" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.warmth" label="亲和度" left="冷静" right="温暖" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.directness" label="直接程度" left="委婉" right="直接" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.rationality" label="理性程度" left="感性" right="理性" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.humor" label="幽默感" left="严肃" right="幽默" :min="0" :max="100" />
        </div>
      </div>

      <!-- 2. 聊天习惯 -->
      <div class="section-card">
        <div class="section-title">聊天习惯</div>
        <div class="slider-grid">
          <SliderRow v-model="form.personalityConfig.verbosity" label="回复长度" left="简短" right="详细" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.shortSentence" label="短句程度" left="段落" right="短句" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.toneWords" label="语气词使用" left="不用" right="多用" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.initiative" label="主动性" left="被动回应" right="主动找话题" :min="0" :max="100" />
            <div class="slider-hint">
              数值越高，AI 越可能主动延续话题或提醒你。
              系统会自动限制频率（每日 {{ dailyLimit }} 条），避免打扰。
            </div>
          <SliderRow v-model="form.personalityConfig.teasing" label="吐槽程度" left="从不禁" right="可吐槽" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.customerServiceAvoidance" label="客服腔抑制" left="官方" right="自然" :min="0" :max="100" />
        </div>
      </div>

      <!-- 3. 陪伴设置 -->
      <div class="section-card">
        <div class="section-title">陪伴设置</div>
        <div class="slider-grid">
          <SliderRow v-model="form.personalityConfig.companionship" label="陪伴感" left="独立" right="陪伴" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.comfortLevel" label="安抚强度" left="中立" right="安抚" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.patience" label="耐心" left="直接" right="耐心" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.preachingAvoidance" label="说教抑制" left="可说教" right="严禁说教" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.boundary" label="边界感" left="放松" right="严谨" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.dependencyAvoidance" label="依赖引导抑制" left="允许" right="严禁" :min="0" :max="100" />
        </div>
      </div>

      <!-- 4. 任务能力 -->
      <div class="section-card">
        <div class="section-title">任务能力</div>
        <div class="slider-grid">
          <SliderRow v-model="form.personalityConfig.execution" label="执行力" left="倾听" right="给方案" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.explanationDepth" label="解释深度" left="简述" right="深入" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.structureLevel" label="结构化程度" left="随性" right="条理" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.judgment" label="判断力" left="谨慎" right="果断" :min="0" :max="100" />
          <SliderRow v-model="form.personalityConfig.clarification" label="追问倾向" left="不追问" right="会追问" :min="0" :max="100" />
        </div>
      </div>

      <!-- 5. 亲密边界 -->
      <el-collapse v-model="activeCollapse" class="section-collapse">
        <el-collapse-item title="亲密边界（高级设置）" name="intimacy">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
            <template #title>
              该设置只控制亲近表达方式，不允许生成色情、露骨或越界内容。
              系统会自动进行安全裁剪。
            </template>
          </el-alert>
          <div class="slider-grid">
            <SliderRow v-model="form.personalityConfig.intimacyExpression" label="亲密表达强度" left="克制" right="可表达" :min="0" :max="100" />
            <SliderRow v-model="form.personalityConfig.flirtiness" label="暧昧倾向" left="零容忍" right="轻微" :min="0" :max="100" />
            <SliderRow v-model="form.personalityConfig.romanticTone" label="恋爱感" left="无" right="轻微" :min="0" :max="100" />
            <SliderRow v-model="form.personalityConfig.suggestivenessAvoidance" label="性暗示规避" left="允许" right="严禁" :min="0" :max="100" />
            <SliderRow v-model="form.personalityConfig.intimacyBoundary" label="亲密边界" left="宽松" right="严格" :min="0" :max="100" />
          </div>
        </el-collapse-item>
      </el-collapse>

      <!-- 睡觉回复设置 -->
      <div class="section-card">
        <div class="section-title">睡觉回复</div>
        <div class="sleep-setting-grid">
          <div class="sleep-item">
            <label class="gender-label">睡觉后是否回复</label>
            <el-switch v-model="sleepForm.sleepReplyEnabled" />
            <span class="gender-hint">开启后角色睡觉时仍可回复，但回复会简短困倦</span>
          </div>
          <div class="sleep-item" v-if="sleepForm.sleepReplyEnabled">
            <label class="gender-label">睡觉回复模式</label>
            <el-select v-model="sleepForm.sleepReplyMode" placeholder="选择模式" size="default" style="width:100%">
              <el-option v-for="opt in SLEEP_REPLY_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">决定角色睡觉后的回复方式</span>
          </div>
          <div class="sleep-item" v-if="!sleepForm.sleepReplyEnabled">
            <label class="gender-label">关闭时行为</label>
            <el-select v-model="sleepForm.sleepReplyMode" placeholder="选择模式" size="default" style="width:100%">
              <el-option v-for="opt in SLEEP_OFF_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">不回复时可显示系统提示，或完全静默</span>
          </div>
        </div>
      </div>

      <!-- 生活场景 -->
      <div class="section-card">
        <div class="section-title">生活场景</div>
        <div class="gender-grid">
          <div class="gender-item">
            <label class="gender-label">选择生活场景</label>
            <el-select v-model="lifeIdentity" placeholder="选择生活场景" size="default" style="width:100%" @change="onLifeIdentityChange">
              <el-option v-for="opt in LIFE_IDENTITY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">选择后将自动显示对应的生活规则配置项</span>
          </div>
          <div class="gender-item" v-if="isCustomLifeIdentity">
            <label class="gender-label">自定义场景描述</label>
            <el-input v-model="lifeIdentityCustom" placeholder="例如：自由插画师、考研党、数字游民..." size="default" />
            <span class="gender-hint">手动输入你的生活场景，角色会据此调整行为</span>
          </div>
        </div>
      </div>

      <!-- 课程/固定日程 -->
      <div class="section-card" v-if="showCourseSection">
        <div class="section-title">
          课程/固定日程
          <el-button size="small" type="primary" @click="showCourseDialog = true" style="margin-left:12px">+ 添加</el-button>
        </div>
        <div v-if="courses.length === 0" class="gender-hint" style="padding:8px 0">暂无固定日程，点击上方按钮添加课程或会议</div>
        <div v-for="c in courses" :key="c.id" class="course-item">
          <div class="course-info">
            <span class="course-title">{{ c.title }}</span>
            <span class="course-time">{{ c.startTime }} - {{ c.endTime }}</span>
            <span class="course-days">{{ c.repeatDays || '每天' }}</span>
            <span class="course-reply">回复：{{ REPLY_MODE_LABELS[c.replyMode] || c.replyMode }}</span>
          </div>
          <div class="course-actions">
            <el-switch v-model="c.enabled" size="small" @change="(v: any) => toggleCourse(c.id, v)" />
            <el-button size="small" @click="editCourse(c)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteCourse(c.id)">删除</el-button>
          </div>
        </div>
      </div>

      <!-- 特殊状态 -->
      <div class="section-card">
        <div class="section-title">
          特殊状态
          <el-button size="small" type="primary" @click="showSpecialDialog = true" style="margin-left:12px">+ 添加</el-button>
        </div>
        <div v-if="specialEvents.length === 0" class="gender-hint" style="padding:8px 0">暂无特殊状态，可添加考试周、兼职、健身、图书馆、生病等</div>
        <div v-for="se in specialEvents" :key="se.id" class="course-item">
          <div class="course-info">
            <span class="course-title">{{ se.title }}</span>
            <el-tag size="small" type="info">{{ SPECIAL_TYPE_LABELS[se.eventType] || se.eventType }}</el-tag>
            <span class="course-time" v-if="se.startDate">{{ se.startDate }} ~ {{ se.endDate }}</span>
            <span class="course-time" v-if="se.startTime">{{ se.startTime }}-{{ se.endTime }}</span>
            <span class="course-reply">回复：{{ se.replyMode === 'NO_REPLY' ? '不回复' : se.replyMode === 'SHORT_REPLY' ? '简短' : '正常' }}</span>
          </div>
          <div class="course-actions">
            <el-switch v-model="se.enabled" size="small" @change="(v: any) => toggleSpecial(se.id, v)" />
            <el-button size="small" @click="editSpecial(se)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteSpecial(se.id)">删除</el-button>
          </div>
        </div>
      </div>

      <!-- 特殊状态编辑对话框 -->
      <el-dialog v-model="showSpecialDialog" :title="editingSpecial ? '编辑特殊状态' : '添加特殊状态'" width="500px" destroy-on-close>
        <div class="course-form">
          <div class="form-item">
            <label>标题</label>
            <el-input v-model="specialForm.title" placeholder="例如：期末考、周末家教" size="default" />
          </div>
          <div class="form-item">
            <label>类型</label>
            <el-select v-model="specialForm.eventType" size="default" style="width:100%">
              <el-option v-for="opt in SPECIAL_EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="form-row">
            <div class="form-item" style="flex:1">
              <label>开始日期</label>
              <el-date-picker v-model="specialForm.startDate" type="date" value-format="YYYY-MM-DD" placeholder="开始" size="default" style="width:100%" />
            </div>
            <div class="form-item" style="flex:1">
              <label>结束日期</label>
              <el-date-picker v-model="specialForm.endDate" type="date" value-format="YYYY-MM-DD" placeholder="结束" size="default" style="width:100%" />
            </div>
          </div>
          <div class="form-row">
            <div class="form-item" style="flex:1">
              <label>开始时间</label>
              <el-time-picker v-model="specialForm.startTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" />
            </div>
            <div class="form-item" style="flex:1">
              <label>结束时间</label>
              <el-time-picker v-model="specialForm.endTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" />
            </div>
          </div>
          <div class="form-item">
            <label>回复模式</label>
            <el-select v-model="specialForm.replyMode" size="default" style="width:100%">
              <el-option v-for="opt in SPECIAL_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="form-item" v-if="specialForm.eventType === 'SICK_REST'">
            <el-checkbox v-model="specialForm.affectSleep" label="影响睡眠（提前睡觉）" />
          </div>
        </div>
        <template #footer>
          <el-button @click="showSpecialDialog = false">取消</el-button>
          <el-button type="primary" @click="saveSpecial" :loading="specialSaving">
            {{ editingSpecial ? '更新' : '添加' }}
          </el-button>
        </template>
      </el-dialog>

      <!-- 上班规则 -->
      <div class="section-card" v-if="showWorkSection">
        <div class="section-title">上班规则</div>
        <div class="work-grid">
          <div class="work-item">
            <label class="gender-label">启用上班状态</label>
            <el-switch v-model="workForm.enabled" />
            <span class="gender-hint">开启后工作日按上班规则运行</span>
          </div>
          <div class="work-item">
            <label class="gender-label">工作日</label>
            <el-checkbox-group v-model="workForm.workDaysArr">
              <el-checkbox v-for="d in WEEKDAY_OPTIONS" :key="d.value" :label="d.value" :value="d.value">{{ d.label }}</el-checkbox>
            </el-checkbox-group>
          </div>
          <div class="work-item">
            <label class="gender-label">上班时间</label>
            <el-time-picker v-model="workForm.workStartTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
          </div>
          <div class="work-item">
            <label class="gender-label">下班时间</label>
            <el-time-picker v-model="workForm.workEndTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
          </div>
          <div class="work-item">
            <label class="gender-label">午休开始</label>
            <el-time-picker v-model="workForm.lunchBreakStartTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
          </div>
          <div class="work-item">
            <label class="gender-label">午休结束</label>
            <el-time-picker v-model="workForm.lunchBreakEndTime" format="HH:mm" value-format="HH:mm" size="default" style="width:140px" />
          </div>
          <div class="work-item">
            <label class="gender-label">通勤时间(分)</label>
            <div style="display:flex;gap:8px;align-items:center">
              <el-input-number v-model="workForm.commuteMinMinutes" :min="5" :max="90" size="default" style="width:100px" />
              <span style="font-size:11px;color:var(--ac-color-text-placeholder)">到</span>
              <el-input-number v-model="workForm.commuteMaxMinutes" :min="5" :max="120" size="default" style="width:100px" />
            </div>
          </div>
          <div class="work-item">
            <label class="gender-label">准备时间(分)</label>
            <div style="display:flex;gap:8px;align-items:center">
              <el-input-number v-model="workForm.prepareMinMinutes" :min="10" :max="60" size="default" style="width:100px" />
              <span style="font-size:11px;color:var(--ac-color-text-placeholder)">到</span>
              <el-input-number v-model="workForm.prepareMaxMinutes" :min="10" :max="90" size="default" style="width:100px" />
            </div>
          </div>
          <div class="work-item">
            <label class="gender-label">回复模式</label>
            <el-select v-model="workForm.replyMode" size="default" style="width:140px">
              <el-option v-for="opt in WORK_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="gender-hint">工作期间的回复方式</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Preview Dialog -->
    <el-dialog v-model="showPromptEditor" title="修改角色提示词" width="700px" destroy-on-close>
      <div class="preview-content">
        <el-input v-model="editingPrompt" type="textarea" :rows="20" class="preview-textarea" />
      </div>
      <template #footer>
        <el-button @click="showPromptEditor = false">关闭</el-button>
        <el-button @click="copyPrompt">复制提示词</el-button>
        <el-button type="primary" @click="saveCharPrompt" :loading="promptSaving">保存</el-button>
      </template>
    </el-dialog>

    <!-- 课程编辑对话框 -->
    <el-dialog v-model="showCourseDialog" :title="editingCourse ? '编辑日程' : '添加日程'" width="480px" destroy-on-close>
      <div class="course-form">
        <div class="form-item">
          <label>标题</label>
          <el-input v-model="courseForm.title" placeholder="例如：高等数学" size="default" />
        </div>
        <div class="form-item">
          <label>类型</label>
          <el-select v-model="courseForm.eventType" size="default" style="width:100%">
            <el-option v-for="opt in EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>开始时间</label>
            <el-time-picker v-model="courseForm.startTime" format="HH:mm" value-format="HH:mm" placeholder="开始" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>结束时间</label>
            <el-time-picker v-model="courseForm.endTime" format="HH:mm" value-format="HH:mm" placeholder="结束" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-item">
          <label>重复星期</label>
          <el-checkbox-group v-model="courseForm.repeatDays">
            <el-checkbox v-for="d in WEEKDAY_OPTIONS" :key="d.value" :label="d.value" :value="d.value">{{ d.label }}</el-checkbox>
          </el-checkbox-group>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>准备时间(最少分)</label>
            <el-input-number v-model="courseForm.prepareMinMinutes" :min="5" :max="60" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>准备时间(最多分)</label>
            <el-input-number v-model="courseForm.prepareMaxMinutes" :min="5" :max="90" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-item">
          <label>回复模式</label>
          <el-select v-model="courseForm.replyMode" size="default" style="width:100%">
            <el-option v-for="opt in REPLY_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <span class="gender-hint">决定该日程期间的回复方式</span>
        </div>
      </div>
      <template #footer>
        <el-button @click="showCourseDialog = false">取消</el-button>
        <el-button type="primary" @click="saveCourse" :loading="courseSaving">
          {{ editingCourse ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, inject, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { Edit, View } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"
import { useCachedApi } from "../../composables/useCachedApi"
import { useRoleProfile, GENDER_OPTIONS, PRONOUN_OPTIONS, ADDRESSING_OPTIONS, GENDER_PRONOUN_MAP } from "../../composables/useRoleProfile"
import { useLifestyleTendency, LIFESTYLE_SLIDERS, LIFESTYLE_GROUPS } from "../../composables/useLifestyleTendency"
import { useSleepSetting, SLEEP_REPLY_MODE_OPTIONS } from "../../composables/useSleepSetting"
import { useFixedEvents, WEEKDAY_OPTIONS, EVENT_TYPE_OPTIONS, REPLY_MODE_OPTIONS, LIFE_IDENTITY_OPTIONS, type FixedEvent } from "../../composables/useFixedEvents"
import { useSpecialEvents, SPECIAL_EVENT_TYPE_OPTIONS, SPECIAL_REPLY_OPTIONS } from "../../composables/useSpecialEvents"
import { useWorkProfile, WORK_REPLY_OPTIONS } from "../../composables/useWorkProfile"
import SliderRow from "../../components/SliderRow.vue"

const { get, post } = useApi()
const { invalidateCache } = useCachedApi()

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))
const refreshHealth = inject<() => void>('refreshHealth', () => {})

const { getRoleProfile, updateRoleProfile } = useRoleProfile()
const { getLifestyleTendency, updateLifestyleTendency, resetLifestyleTendency } = useLifestyleTendency()
const { getSleepSetting, updateSleepSetting } = useSleepSetting()
const { getFixedEvents, createFixedEvent, updateFixedEvent, deleteFixedEvent } = useFixedEvents()
const { getSpecialEvents, createSpecialEvent, updateSpecialEvent, deleteSpecialEvent } = useSpecialEvents()
const { getWorkProfile, updateWorkProfile } = useWorkProfile()

// ==========================================================
// State
// ==========================================================
const charId = ref("")
const saving = ref(false)
const resetting = ref(false)
const promptLoading = ref(false)
const showPromptEditor = ref(false)
const activeCollapse = ref<string[]>([])
const editingPrompt = ref("")
const promptSaving = ref(false)
const courseSaving = ref(false)
const showCourseDialog = ref(false)
const editingCourse = ref<FixedEvent | null>(null)
const courses = ref<FixedEvent[]>([])

// 生活场景
const PRESET_IDENTITIES = ["SCHOOL", "WORK", "UNEMPLOYED", "HOME"]
const lifeIdentity = ref("CUSTOM")
const lifeIdentityCustom = ref("")
const isCustomLifeIdentity = computed(() => !PRESET_IDENTITIES.includes(lifeIdentity.value))
const showCourseSection = computed(() => lifeIdentity.value === "SCHOOL" || isCustomLifeIdentity.value)
const showWorkSection = computed(() => lifeIdentity.value === "WORK" || isCustomLifeIdentity.value)
function onLifeIdentityChange(val: string) {
  lifeIdentity.value = val
  if (PRESET_IDENTITIES.includes(val)) {
    lifeIdentityCustom.value = ""
  }
  saveConfig()
}

const DEFAULT_CONFIG = {
  familiarity: 78, formality: 22, customerServiceAvoidance: 92,
  directness: 75, verbosity: 32, structureLevel: 40, shortSentence: 85, toneWords: 45,
  warmth: 58, emotionalExpression: 45, comfortLevel: 55, preachingAvoidance: 88,
  companionship: 55, boundary: 85, dependencyAvoidance: 85,
  execution: 75, explanationDepth: 55, judgment: 75, clarification: 35,
  intimacyExpression: 25, flirtiness: 0, romanticTone: 0,
  suggestivenessAvoidance: 100, intimacyBoundary: 90,
}

const form = reactive({
  name: "轻熟朋友",
  description: "自然、简短、有反应，有一点熟悉感，但不过度装熟。",
  isDefault: false,
  personalityConfig: { ...DEFAULT_CONFIG },
  chatStyleConfig: null as any,
  sceneRules: null as any,
})

const genderForm = reactive({
  roleName: "小暖",
  gender: "UNSPECIFIED" as string,
  genderLabel: null as string | null,
  pronoun: "TA",
  selfReference: "我",
  userAddressingStyle: "自然称呼" as string | null,
  genderExpression: 30,
})
const genderSaving = ref(false)

const lifestyleForm = reactive({
  punctualityTendency: 50,
  earlyPrepareTendency: 50,
  selfDisciplineTendency: 50,
  sleepinessTendency: 50,
  randomnessTendency: 50,
  activityEnergy: 50,
  socialEnergy: 50,
  careTendency: 50,
  dailyShareTendency: 50,
})
const lifestyleSaving = ref(false)
const lifestyleResetting = ref(false)
const lifestyleConfigured = ref(false)

const sleepForm = reactive({
  sleepReplyEnabled: false,
  sleepReplyMode: "NO_REPLY",
})
const sleepSaving = ref(false)

const workForm = reactive({
  enabled: false,
  workDaysArr: ["MON", "TUE", "WED", "THU", "FRI"] as string[],
  workStartTime: "09:00",
  workEndTime: "18:00",
  lunchBreakStartTime: "12:00",
  lunchBreakEndTime: "13:30",
  commuteMinMinutes: 15,
  commuteMaxMinutes: 45,
  prepareMinMinutes: 20,
  prepareMaxMinutes: 60,
  replyMode: "SHORT_REPLY",
  allowOvertime: false,
  overtimeProbability: 10,
  overtimeMinMinutes: 30,
  overtimeMaxMinutes: 180,
  overtimeReplyMode: "SHORT_REPLY",
  delayedReplyEnabled: false,
  commuteHomeShareEnabled: true,
  commuteHomeShareProbability: 60,
})
const workSaving = ref(false)

const SPECIAL_TYPE_LABELS: Record<string, string> = {
  EXAM_WEEK: "考试周", EXAM: "具体考试", PART_TIME_WORK: "周末兼职",
  EVENING_WORKOUT: "晚上健身", LIBRARY_STUDY: "图书馆学习",
  SICK_REST: "生病休息", CUSTOM: "自定义",
}

const specialEvents = ref<any[]>([])
const showSpecialDialog = ref(false)
const editingSpecial = ref<any>(null)
const specialSaving = ref(false)
const specialForm = reactive({
  title: "", eventType: "EXAM", startDate: null as string | null, endDate: null as string | null,
  startTime: null as string | null, endTime: null as string | null,
  replyMode: "SHORT_REPLY", affectSleep: false,
})

async function loadSpecialEvents() {
  try { specialEvents.value = await getSpecialEvents() } catch { }
}

function editSpecial(se: any) {
  editingSpecial.value = se
  specialForm.title = se.title; specialForm.eventType = se.eventType
  specialForm.startDate = se.startDate; specialForm.endDate = se.endDate
  specialForm.startTime = se.startTime; specialForm.endTime = se.endTime
  specialForm.replyMode = se.replyMode || "SHORT_REPLY"
  specialForm.affectSleep = !!se.affectSleep
  showSpecialDialog.value = true
}

async function saveSpecial() {
  if (!specialForm.title.trim()) { ElMessage.warning("请填写标题"); return }
  specialSaving.value = true
  try {
    const input = { ...specialForm, eventType: specialForm.eventType }
    if (editingSpecial.value) {
      await updateSpecialEvent(editingSpecial.value.id, input, injectedCharacterId?.value ?? undefined)
    } else {
      await createSpecialEvent(input)
    }
    showSpecialDialog.value = false; editingSpecial.value = null
    specialForm.title = ""; specialForm.startDate = null; specialForm.endDate = null
    specialForm.startTime = null; specialForm.endTime = null; specialForm.replyMode = "SHORT_REPLY"; specialForm.affectSleep = false
    await loadSpecialEvents()
    ElMessage.success(editingSpecial.value ? "已更新" : "已添加")
  } catch (e: any) { ElMessage.error(e?.message || "保存失败") }
  finally { specialSaving.value = false }
}

async function deleteSpecial(id: number) {
  try { await deleteSpecialEvent(id); await loadSpecialEvents(); ElMessage.success("已删除") }
  catch { ElMessage.error("删除失败") }
}

async function toggleSpecial(id: number, enabled: boolean) {
  try { await updateSpecialEvent(id, { enabled }) } catch { }
}


// Load work profile
async function loadWorkProfile() {
  try {
    const wp = await getWorkProfile(injectedCharacterId?.value ?? undefined)
    if (wp) {
      workForm.enabled = wp.enabled
      workForm.workDaysArr = wp.workDays ? wp.workDays.split(",") : ["MON","TUE","WED","THU","FRI"]
      workForm.workStartTime = wp.workStartTime
      workForm.workEndTime = wp.workEndTime
      workForm.lunchBreakStartTime = wp.lunchBreakStartTime
      workForm.lunchBreakEndTime = wp.lunchBreakEndTime
      workForm.commuteMinMinutes = wp.commuteMinMinutes
      workForm.commuteMaxMinutes = wp.commuteMaxMinutes
      workForm.prepareMinMinutes = wp.prepareMinMinutes
      workForm.prepareMaxMinutes = wp.prepareMaxMinutes
      workForm.replyMode = wp.replyMode
      workForm.allowOvertime = wp.allowOvertime
      workForm.overtimeProbability = wp.overtimeProbability
      workForm.overtimeMinMinutes = wp.overtimeMinMinutes
      workForm.overtimeMaxMinutes = wp.overtimeMaxMinutes
      workForm.overtimeReplyMode = wp.overtimeReplyMode
      workForm.delayedReplyEnabled = wp.delayedReplyEnabled
      workForm.commuteHomeShareEnabled = wp.commuteHomeShareEnabled
      workForm.commuteHomeShareProbability = wp.commuteHomeShareProbability
    }
  } catch { /* silent */ }
}

async function saveWorkProfile() {
  workSaving.value = true
  try {
    await updateWorkProfile({
      enabled: workForm.enabled,
      workDays: workForm.workDaysArr.join(","),
      workStartTime: workForm.workStartTime,
      workEndTime: workForm.workEndTime,
      lunchBreakStartTime: workForm.lunchBreakStartTime,
      lunchBreakEndTime: workForm.lunchBreakEndTime,
      commuteMinMinutes: workForm.commuteMinMinutes,
      commuteMaxMinutes: workForm.commuteMaxMinutes,
      prepareMinMinutes: workForm.prepareMinMinutes,
      prepareMaxMinutes: workForm.prepareMaxMinutes,
      replyMode: workForm.replyMode,
      allowOvertime: workForm.allowOvertime,
      overtimeProbability: workForm.overtimeProbability,
      overtimeMinMinutes: workForm.overtimeMinMinutes,
      overtimeMaxMinutes: workForm.overtimeMaxMinutes,
      overtimeReplyMode: workForm.overtimeReplyMode,
      delayedReplyEnabled: workForm.delayedReplyEnabled,
      commuteHomeShareEnabled: workForm.commuteHomeShareEnabled,
      commuteHomeShareProbability: workForm.commuteHomeShareProbability,
    } as any, injectedCharacterId?.value ?? undefined)
    ElMessage.success("上班规则已保存")
  } catch {
    ElMessage.error("保存失败")
  } finally {
    workSaving.value = false
  }
}


const SLEEP_OFF_MODE_OPTIONS = [
  { label: "不回复", value: "NO_REPLY" },
  { label: "显示系统提示", value: "SYSTEM_NOTICE" },
]

const REPLY_MODE_LABELS: Record<string, string> = {
  NO_REPLY: "不回复",
  SHORT_REPLY: "简短回复",
  NORMAL_REPLY: "正常回复",
  DELAY_REPLY: "延迟回复",
}

const courseForm = reactive({
  title: "",
  eventType: "CLASS",
  startTime: "",
  endTime: "",
  repeatDays: [] as string[],
  prepareMinMinutes: 10,
  prepareMaxMinutes: 40,
  replyMode: "SHORT_REPLY",
})

watch(() => genderForm.gender, (newGender) => {
  if (newGender !== "CUSTOM" && GENDER_PRONOUN_MAP[newGender]) {
    genderForm.pronoun = GENDER_PRONOUN_MAP[newGender]
  }
})

const dailyLimit = computed(() => {
  const i = form.personalityConfig.initiative
  if (i <= 20) return 1
  if (i <= 50) return 3
  if (i <= 80) return 5
  return 8
})

// ==========================================================
onMounted(async () => {
// Load data
// ==========================================================
    const cid = injectedCharacterId?.value
    if (cid) {
      // Load specific character by ID from parent selection
      try {
        const data = await get<any>("/api/characters/" + cid)
        if (data) {
          charId.value = data.id || cid
          form.name = data.name || form.name
          form.description = data.description || form.description
          form.isDefault = !!data.isDefault
          if (data.personalityConfig) {
            form.personalityConfig = { ...DEFAULT_CONFIG, ...data.personalityConfig }
          }
          if (data.lifeIdentity) {
            if (PRESET_IDENTITIES.includes(data.lifeIdentity)) {
              lifeIdentity.value = data.lifeIdentity
            } else {
              lifeIdentity.value = "CUSTOM"
              lifeIdentityCustom.value = data.lifeIdentity
            }
          }
        }
      } catch { /* character not found */ }
    }
    if (!charId.value) {
      // Fallback: try default character
      try {
        const data = await get<any>("/api/ai/character/default")
        if (data) {
          charId.value = data.id || ""
          form.name = data.name || form.name
          form.description = data.description || form.description
          form.isDefault = !!data.isDefault
          if (data.personalityConfig) {
            form.personalityConfig = { ...DEFAULT_CONFIG, ...data.personalityConfig }
          }
          if (data.lifeIdentity) {
            if (PRESET_IDENTITIES.includes(data.lifeIdentity)) {
              lifeIdentity.value = data.lifeIdentity
            } else {
              lifeIdentity.value = "CUSTOM"
              lifeIdentityCustom.value = data.lifeIdentity
            }
          }
        }
      } catch { /* no default character */ }
      try {
        const chars = await get<any[]>("/api/characters?includeDisabled=true")
        const active = chars.find((c: any) => c.isActive) || chars.find((c: any) => c.isDefault) || chars[0]
        if (active) {
          charId.value = active.id
          form.name = active.name || form.name
          form.description = active.description || form.description
          if (active.lifeIdentity) {
            if (PRESET_IDENTITIES.includes(active.lifeIdentity)) {
              lifeIdentity.value = active.lifeIdentity
            } else {
              lifeIdentity.value = "CUSTOM"
              lifeIdentityCustom.value = active.lifeIdentity
            }
          }
          }
        } catch { /* silent */ }
      }

  // Load role profile
  try {
    const rp = await getRoleProfile(injectedCharacterId?.value ?? undefined)
    if (rp) {
      genderForm.roleName = rp.roleName || "小暖"
      genderForm.gender = rp.gender || "UNSPECIFIED"
      genderForm.genderLabel = rp.genderLabel
      genderForm.pronoun = rp.pronoun || "TA"
      genderForm.selfReference = rp.selfReference || "我"
      genderForm.userAddressingStyle = rp.userAddressingStyle
      genderForm.genderExpression = rp.genderExpression ?? 30
    }
  } catch { /* silent */ }

  // Load lifestyle
  try {
    const lt = await getLifestyleTendency(injectedCharacterId?.value ?? undefined)
    if (lt) {
      lifestyleForm.punctualityTendency = lt.punctualityTendency ?? 50
      lifestyleForm.earlyPrepareTendency = lt.earlyPrepareTendency ?? 50
      lifestyleForm.selfDisciplineTendency = lt.selfDisciplineTendency ?? 50
      lifestyleForm.sleepinessTendency = lt.sleepinessTendency ?? 50
      lifestyleForm.randomnessTendency = lt.randomnessTendency ?? 50
      lifestyleForm.activityEnergy = lt.activityEnergy ?? 50
      lifestyleForm.socialEnergy = lt.socialEnergy ?? 50
      lifestyleForm.careTendency = lt.careTendency ?? 50
      lifestyleForm.dailyShareTendency = lt.dailyShareTendency ?? 50
      lifestyleConfigured.value = (lt as any).manuallyConfigured || false
    }
  } catch { /* silent */ }

  // Load sleep setting
  try {
    const ss = await getSleepSetting(injectedCharacterId?.value ?? undefined)
    if (ss) {
      sleepForm.sleepReplyEnabled = ss.sleepReplyEnabled
      sleepForm.sleepReplyMode = ss.sleepReplyMode
    }
  } catch { /* silent */ }


  // 加载上班规则
  await loadWorkProfile()
  // Load courses
  await loadCourses()
  await loadSpecialEvents()
})

async function loadCourses() {
  try {
    courses.value = await getFixedEvents(injectedCharacterId?.value ?? undefined)
  } catch { /* silent */ }
}

// ==========================================================
// Save
// ==========================================================
async function saveConfig() {
  saving.value = true
  try {
    const payload: any = {
      name: form.name.trim(),
      description: form.description.trim(),
      personalityConfig: form.personalityConfig,
      isDefault: form.isDefault,
      lifeIdentity: isCustomLifeIdentity.value ? lifeIdentityCustom.value || lifeIdentity.value : lifeIdentity.value,
    }
    if (charId.value) payload.id = charId.value
    const result = await post<any>("/api/ai/character/save", payload)
    if (result?.id) charId.value = result.id

    // Save gender profile
    try {
      await updateRoleProfile({
        roleName: form.name.trim(),
          gender: genderForm.gender,
          genderLabel: genderForm.gender === "CUSTOM" ? genderForm.genderLabel : null,
          pronoun: genderForm.pronoun,
          selfReference: genderForm.selfReference,
          userAddressingStyle: genderForm.userAddressingStyle,
          genderExpression: genderForm.genderExpression,
        }, injectedCharacterId?.value ?? undefined)
    } catch { /* silent */ }

    // Save sleep setting
    try {
      await updateSleepSetting({
        sleepReplyEnabled: sleepForm.sleepReplyEnabled,
        sleepReplyMode: sleepForm.sleepReplyMode,
      }, injectedCharacterId?.value ?? undefined)
    } catch { /* silent */ }

    // Save work profile
    try {
      await updateWorkProfile({
        enabled: workForm.enabled,
        workDays: workForm.workDaysArr.join(","),
        workStartTime: workForm.workStartTime,
        workEndTime: workForm.workEndTime,
        lunchBreakStartTime: workForm.lunchBreakStartTime,
        lunchBreakEndTime: workForm.lunchBreakEndTime,
        commuteMinMinutes: workForm.commuteMinMinutes,
        commuteMaxMinutes: workForm.commuteMaxMinutes,
        prepareMinMinutes: workForm.prepareMinMinutes,
        prepareMaxMinutes: workForm.prepareMaxMinutes,
        replyMode: workForm.replyMode,
        allowOvertime: workForm.allowOvertime,
        overtimeProbability: workForm.overtimeProbability,
        overtimeMinMinutes: workForm.overtimeMinMinutes,
        overtimeMaxMinutes: workForm.overtimeMaxMinutes,
        overtimeReplyMode: workForm.overtimeReplyMode,
        delayedReplyEnabled: workForm.delayedReplyEnabled,
        commuteHomeShareEnabled: workForm.commuteHomeShareEnabled,
        commuteHomeShareProbability: workForm.commuteHomeShareProbability,
      } as any, injectedCharacterId?.value ?? undefined)
    } catch { /* silent */ }


    ElMessage.success("保存成功")
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false
  }
}

// Lifestyle save
async function saveLifestyle() {
  lifestyleSaving.value = true
  try {
    await updateLifestyleTendency({
      punctualityTendency: lifestyleForm.punctualityTendency,
      earlyPrepareTendency: lifestyleForm.earlyPrepareTendency,
      selfDisciplineTendency: lifestyleForm.selfDisciplineTendency,
      sleepinessTendency: lifestyleForm.sleepinessTendency,
      randomnessTendency: lifestyleForm.randomnessTendency,
      activityEnergy: lifestyleForm.activityEnergy,
      socialEnergy: lifestyleForm.socialEnergy,
      careTendency: lifestyleForm.careTendency,
      dailyShareTendency: lifestyleForm.dailyShareTendency,
    }, injectedCharacterId?.value ?? undefined)
    lifestyleConfigured.value = true
    ElMessage.success("作息倾向已保存")
  } catch {
    ElMessage.error("保存失败")
  } finally {
    lifestyleSaving.value = false
  }
}

async function resetLifestyle() {
  lifestyleResetting.value = true
  try {
    const data = await resetLifestyleTendency(injectedCharacterId?.value ?? undefined)
    lifestyleForm.punctualityTendency = data.punctualityTendency ?? 50
    lifestyleForm.earlyPrepareTendency = data.earlyPrepareTendency ?? 50
    lifestyleForm.selfDisciplineTendency = data.selfDisciplineTendency ?? 50
    lifestyleForm.sleepinessTendency = data.sleepinessTendency ?? 50
    lifestyleForm.randomnessTendency = data.randomnessTendency ?? 50
    lifestyleForm.activityEnergy = data.activityEnergy ?? 50
    lifestyleForm.socialEnergy = data.socialEnergy ?? 50
    lifestyleForm.careTendency = data.careTendency ?? 50
    lifestyleForm.dailyShareTendency = data.dailyShareTendency ?? 50
    lifestyleConfigured.value = false
    ElMessage.success("已恢复默认作息倾向")
  } catch {
    ElMessage.error("恢复失败")
  } finally {
    lifestyleResetting.value = false
  }
}

// Course management
function editCourse(c: FixedEvent) {
  editingCourse.value = c
  courseForm.title = c.title
  courseForm.eventType = c.eventType
  courseForm.startTime = c.startTime
  courseForm.endTime = c.endTime
  courseForm.repeatDays = c.repeatDays ? c.repeatDays.split(",") : []
  courseForm.prepareMinMinutes = c.prepareMinMinutes
  courseForm.prepareMaxMinutes = c.prepareMaxMinutes
  courseForm.replyMode = c.replyMode
  showCourseDialog.value = true
}

async function saveCourse() {
  if (!courseForm.title.trim() || !courseForm.startTime || !courseForm.endTime) {
    ElMessage.warning("请填写标题和时间")
    return
  }
  courseSaving.value = true
  try {
    const input = {
      title: courseForm.title.trim(),
      eventType: courseForm.eventType,
      startTime: courseForm.startTime,
      endTime: courseForm.endTime,
      repeatDays: courseForm.repeatDays.length > 0 ? courseForm.repeatDays.join(",") : null,
      prepareMinMinutes: courseForm.prepareMinMinutes,
      prepareMaxMinutes: courseForm.prepareMaxMinutes,
      replyMode: courseForm.replyMode,
    }
    if (editingCourse.value) {
      await updateFixedEvent(editingCourse.value.id, input, injectedCharacterId?.value ?? undefined)
    } else {
      await createFixedEvent(input)
    }
    showCourseDialog.value = false
    editingCourse.value = null
    courseForm.title = ""
    courseForm.startTime = ""
    courseForm.endTime = ""
    courseForm.repeatDays = []
    courseForm.prepareMinMinutes = 10
    courseForm.prepareMaxMinutes = 40
    courseForm.replyMode = "SHORT_REPLY"
    await loadCourses()
    ElMessage.success(editingCourse.value ? "已更新" : "已添加")
  } catch (e: any) {
    ElMessage.error(e?.message || "保存失败")
  } finally {
    courseSaving.value = false
  }
}

async function deleteCourse(id: number) {
  try {
    await deleteFixedEvent(id)
    await loadCourses()
    ElMessage.success("已删除")
  } catch {
    ElMessage.error("删除失败")
  }
}

async function toggleCourse(id: number, enabled: boolean) {
  try {
    await updateFixedEvent(id, {})
    await loadCourses()
  } catch { /* silent */ }
}

// ==========================================================
// Reset
// ==========================================================
// --- Set as default character ---
async function setAsDefault(val: boolean) {
  if (!charId.value) return
  try {
    if (val) {
    await post<any>("/api/ai/character/" + charId.value + "/set-default")
    ElMessage.success("已设为默认角色")
    } else {
    await post<any>("/api/ai/character/reset-default")
    ElMessage.success("已取消默认角色")
    }
    // Sync default character to localStorage
    if (val) {
      localStorage.setItem("uai-default-char", JSON.stringify({
        id: charId.value,
        name: form.name,
        identity: form.personalityConfig?.identity || form.description || "",
        updatedAt: Date.now(),
      }))
    } else {
      localStorage.removeItem("uai-default-char")
    }
    // Clear all related caches
    invalidateCache("_api_characters")
    localStorage.removeItem("webchat-char-id")
    // Notify other components to refresh
    window.dispatchEvent(new CustomEvent("default-char-changed"))
    refreshHealth()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || "操作失败")
    form.isDefault = !val // Revert switch state
  }
}

async function resetConfig() {
  resetting.value = true
  try {
    if (charId.value) {
      const result = await post<any>("/api/ai/character/reset-default", { id: charId.value })
      if (result) {
        form.name = result.name || form.name
        form.description = result.description || form.description
        if (result.personalityConfig) {
          form.personalityConfig = { ...DEFAULT_CONFIG, ...result.personalityConfig }
        }
      }
    } else {
      form.personalityConfig = { ...DEFAULT_CONFIG }
      form.name = "轻熟朋友"
      form.description = "自然、简短、有反应，有一点熟悉感，但不过度装熟。"
    }
    ElMessage.success("已重置为默认配置")
  } catch {
    ElMessage.warning("重置失败，请先保存角色后再试")
  } finally {
    resetting.value = false
  }
}

// Preview
async function editCharPrompt() {
  promptLoading.value = true
  try {
    if (charId.value) {
      const char = await get<any>("/api/ai/character/" + charId.value)
      editingPrompt.value = char?.basePrompt || ""
    } else {
      editingPrompt.value = ""
      ElMessage.info("请先保存角色后再编辑提示词")
    }
    showPromptEditor.value = true
  } catch {
    ElMessage.error("加载提示词失败")
  } finally {
    promptLoading.value = false
  }
}

async function saveCharPrompt() {
  if (!charId.value) {
    ElMessage.warning("请先保存角色后再编辑提示词")
    return
  }
  promptSaving.value = true
  try {
    await post<any>("/api/ai/character/save", {
      id: charId.value,
      name: form.name.trim(),
      basePrompt: editingPrompt.value,
    })
    ElMessage.success("提示词已保存")
    showPromptEditor.value = false
  } catch {
    ElMessage.error("保存失败")
  } finally {
    promptSaving.value = false
  }
}

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(editingPrompt.value)
    ElMessage.success("已复制到剪贴板")
  } catch {
    ElMessage.warning("复制失败，请手动选择复制")
  }
}
</script>

<style scoped>
.ai-char-settings {
  padding: 20px 24px;
  max-width: 900px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 4px;
}

.page-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin: 0;
}

/* Top bar */
.top-bar {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px 16px;
  margin: 16px 0;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.info-item label {
  font-size: 13px;
  color: var(--ac-color-text-secondary);
  white-space: nowrap;
}

.char-id {
  font-size: 12px;
  font-family: monospace;
  color: var(--ac-color-text-muted);
}

.action-row {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--ac-color-border-light);
}

/* Sections */
.sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
  color: var(--ac-color-text);
}

.section-collapse {
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
}

.section-collapse :deep(.el-collapse-item__header) {
  font-size: 14px;
  font-weight: 600;
  padding: 14px 16px;
}

.section-collapse :deep(.el-collapse-item__wrap) {
  padding: 0 16px 14px;
}

.slider-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 20px;
}

@media (max-width: 700px) {
  .slider-grid {
    grid-template-columns: 1fr;
  }
  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}

/* Slider Row */
.slider-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sr-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.sr-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
}

.sr-body {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sr-left, .sr-right {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
  min-width: 24px;
}

.sr-left { text-align: right; }

.sr-slider-wrap {
  flex: 1;
}

.sr-slider {
  --el-slider-height: 4px;
}

.slider-hint {
  grid-column: 1 / -1;
  font-size: 10px;
  color: var(--ac-color-text-muted);
  padding: 2px 0 6px;
  border-bottom: 1px solid var(--ac-color-border-light);
  margin-bottom: 2px;
}

.sr-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
  min-width: 22px;
  text-align: right;
}

/* Preview dialog */
.preview-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.preview-meta {
  display: flex;
  gap: 20px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

.preview-textarea :deep(textarea) {
  font-family: monospace;
  font-size: 12px;
  line-height: 1.5;
}

/* Gender section */
.gender-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}

.gender-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.gender-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.gender-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
}

.gender-hint {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
  line-height: 1.3;
}

@media (max-width: 700px) {
  .gender-grid {
    grid-template-columns: 1fr;
  }
}

/* Sleep setting section */
.sleep-setting-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}

.sleep-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

@media (max-width: 700px) {
  .sleep-setting-grid {
    grid-template-columns: 1fr;
  }
}

/* Course section */
.course-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.course-item:last-child { border-bottom: none; }
.course-info { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.course-title { font-size: 13px; font-weight: 500; }
.course-time { font-size: 12px; color: var(--ac-color-text-secondary); }
.course-days { font-size: 11px; color: var(--ac-color-text-muted); }
.course-reply { font-size: 11px; color: var(--ac-color-primary); }
.course-actions { display: flex; gap: 6px; align-items: center; }

.course-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form-item { display: flex; flex-direction: column; gap: 4px; }
.form-item label { font-size: 12px; color: var(--ac-color-text-secondary); }
.form-row { display: flex; gap: 12px; }

/* Work section */
.work-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 20px;
}
.work-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
@media (max-width: 700px) {
  .work-grid { grid-template-columns: 1fr; }
}
</style>