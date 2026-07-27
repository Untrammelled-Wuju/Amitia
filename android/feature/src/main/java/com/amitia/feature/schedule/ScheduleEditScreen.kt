package com.amitia.feature.schedule

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton

@Composable
fun ScheduleEditScreen(
    scheduleId: String?,
    onBack: () -> Unit,
    viewModel: ScheduleEditViewModel = hiltViewModel()
) {
    val form by viewModel.form.collectAsStateWithLifecycle()
    val saving by viewModel.saving.collectAsStateWithLifecycle()
    ScheduleEditContent(
        form = form,
        saving = saving,
        isEdit = scheduleId != null,
        onBack = onBack,
        onUpdate = viewModel::update,
        onSave = { viewModel.save(onBack) }
    )
}

@Composable
fun ScheduleEditContent(
    form: ScheduleFormState,
    saving: Boolean,
    isEdit: Boolean,
    onBack: () -> Unit,
    onUpdate: (((ScheduleFormState) -> ScheduleFormState)) -> Unit,
    onSave: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = if (isEdit) "编辑日程" else "新建日程", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaTextField(
                value = form.title,
                onValueChange = { v -> onUpdate { it.copy(title = v) } },
                label = "标题",
                placeholder = "例如：项目进度同步"
            )
            AmitiaTextField(
                value = form.date,
                onValueChange = { v -> onUpdate { it.copy(date = v) } },
                label = "日期",
                placeholder = "选择日期",
                leadingIcon = AmitiaIcons.CalendarToday
            )
            androidx.compose.foundation.layout.Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                androidx.compose.foundation.layout.Column(modifier = Modifier.weight(1f)) {
                    AmitiaTextField(
                        value = form.startTime,
                        onValueChange = { v -> onUpdate { it.copy(startTime = v) } },
                        label = "开始时间",
                        placeholder = "09:00",
                        leadingIcon = AmitiaIcons.Schedule
                    )
                }
                androidx.compose.foundation.layout.Column(modifier = Modifier.weight(1f)) {
                    AmitiaTextField(
                        value = form.endTime,
                        onValueChange = { v -> onUpdate { it.copy(endTime = v) } },
                        label = "结束时间",
                        placeholder = "10:00",
                        leadingIcon = AmitiaIcons.Schedule
                    )
                }
            }
            AmitiaTextField(
                value = form.role,
                onValueChange = { v -> onUpdate { it.copy(role = v) } },
                label = "关联角色",
                placeholder = "选择角色",
                leadingIcon = AmitiaIcons.Person
            )
            AmitiaSectionHeader(title = "触发方式")
            ScheduleTriggerType.entries.forEach { type ->
                AmitiaSelectionRow(
                    title = type.label,
                    selected = form.triggerType == type,
                    onSelect = { onUpdate { it.copy(triggerType = type) } },
                    leadingIcon = when (type) {
                        ScheduleTriggerType.Single -> AmitiaIcons.Event
                        ScheduleTriggerType.Repeat -> AmitiaIcons.Sync
                        ScheduleTriggerType.TimeWindow -> AmitiaIcons.Timer
                        ScheduleTriggerType.LifeState -> AmitiaIcons.Bedtime
                        ScheduleTriggerType.RoleProactive -> AmitiaIcons.AutoAwesome
                    }
                )
            }
            when (form.triggerType) {
                ScheduleTriggerType.Repeat -> {
                    AmitiaTextField(
                        value = form.repeatRule,
                        onValueChange = { v -> onUpdate { it.copy(repeatRule = v) } },
                        label = "重复规则",
                        placeholder = "例如：每周一、三、五",
                        leadingIcon = AmitiaIcons.Sync
                    )
                }
                ScheduleTriggerType.TimeWindow -> {
                    AmitiaSectionHeader(title = "时间窗随机触发")
                    AmitiaSwitchRow(
                        title = "在时间窗内随机触发",
                        checked = form.timeWindowRandom,
                        onCheckedChange = { v -> onUpdate { it.copy(timeWindowRandom = v) } },
                        subtitle = "在设定时间窗内随机选取时刻",
                        leadingIcon = AmitiaIcons.Timer
                    )
                    AmitiaNumberField(
                        value = form.repeatRule,
                        onValueChange = { v -> onUpdate { it.copy(repeatRule = v) } },
                        label = "时间窗（小时）",
                        placeholder = "3",
                        unit = "h"
                    )
                }
                ScheduleTriggerType.LifeState -> {
                    AmitiaSectionHeader(title = "生活状态触发")
                    AmitiaSwitchRow(
                        title = "匹配生活状态后触发",
                        checked = form.lifeStateTrigger,
                        onCheckedChange = { v -> onUpdate { it.copy(lifeStateTrigger = v) } },
                        subtitle = "如进入“上课”“上班”等状态时触发",
                        leadingIcon = AmitiaIcons.Bedtime
                    )
                }
                ScheduleTriggerType.RoleProactive -> {
                    AmitiaSectionHeader(title = "角色主动消息")
                    AmitiaSwitchRow(
                        title = "允许角色主动发起",
                        checked = form.roleProactive,
                        onCheckedChange = { v -> onUpdate { it.copy(roleProactive = v) } },
                        subtitle = "角色可在合适时机主动发起本次日程",
                        leadingIcon = AmitiaIcons.AutoAwesome
                    )
                }
                ScheduleTriggerType.Single -> Unit
            }
            AmitiaSectionHeader(title = "渠道与提醒")
            AmitiaTextField(
                value = form.channel,
                onValueChange = { v -> onUpdate { it.copy(channel = v) } },
                label = "投递渠道",
                placeholder = "Web / 微信 / QQ",
                leadingIcon = AmitiaIcons.Hub
            )
            AmitiaTextField(
                value = form.reminder,
                onValueChange = { v -> onUpdate { it.copy(reminder = v) } },
                label = "提醒",
                placeholder = "提前 15 分钟",
                leadingIcon = AmitiaIcons.Notifications
            )
            AmitiaMultilineField(
                value = form.repeatRule.takeIf { form.triggerType == ScheduleTriggerType.Single } ?: "",
                onValueChange = { v -> onUpdate { it.copy(repeatRule = v) } },
                label = "备注",
                placeholder = "可选的补充说明",
                minLines = 2,
                maxLines = 4
            )
            LoadingButton(
                text = if (isEdit) "保存修改" else "创建日程",
                onClick = onSave,
                loading = saving,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Preview(name = "ScheduleEdit - Light", showBackground = true)
@Composable
private fun ScheduleEditLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ScheduleEditContent(
            form = ScheduleFormState(title = "项目进度同步", triggerType = ScheduleTriggerType.TimeWindow, timeWindowRandom = true),
            saving = false,
            isEdit = false,
            onBack = {},
            onUpdate = {},
            onSave = {}
        )
    }
}

@Preview(name = "ScheduleEdit - Dark", showBackground = true)
@Composable
private fun ScheduleEditDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ScheduleEditContent(
            form = ScheduleFormState(title = "晚间复盘", triggerType = ScheduleTriggerType.RoleProactive, roleProactive = true),
            saving = true,
            isEdit = true,
            onBack = {},
            onUpdate = {},
            onSave = {}
        )
    }
}
