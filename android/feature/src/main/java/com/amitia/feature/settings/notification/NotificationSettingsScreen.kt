package com.amitia.feature.settings.notification

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.feature.settings.NotificationSettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun NotificationSettingsScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val notifications = state.notifications

    NotificationSettingsContent(
        settings = notifications,
        onBack = onBack,
        onChange = { viewModel.updateNotifications(it) }
    )
}

@Composable
private fun NotificationSettingsContent(
    settings: NotificationSettings,
    onBack: () -> Unit,
    onChange: (NotificationSettings) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "通知设置", onBack = onBack) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSection(title = "通知渠道") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "角色消息",
                            subtitle = "角色回复和对话消息",
                            checked = settings.characterMessages,
                            onCheckedChange = { onChange(settings.copy(characterMessages = it)) },
                            leadingIcon = AmitiaIcons.Chat
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "主动消息",
                            subtitle = "角色主动发送的消息",
                            checked = settings.proactiveMessages,
                            onCheckedChange = { onChange(settings.copy(proactiveMessages = it)) },
                            leadingIcon = AmitiaIcons.Notifications
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "日程提醒",
                            subtitle = "日程到期和提醒通知",
                            checked = settings.schedule,
                            onCheckedChange = { onChange(settings.copy(schedule = it)) },
                            leadingIcon = AmitiaIcons.Event
                        )
                    }
                }
            }
            AmitiaSection(title = "异常通知") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "渠道异常",
                            subtitle = "渠道连接断开或异常",
                            checked = settings.channelErrors,
                            onCheckedChange = { onChange(settings.copy(channelErrors = it)) },
                            leadingIcon = AmitiaIcons.Error
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "模型和扩展异常",
                            subtitle = "模型调用失败或扩展错误",
                            checked = settings.modelErrors,
                            onCheckedChange = { onChange(settings.copy(modelErrors = it)) },
                            leadingIcon = AmitiaIcons.BugReport
                        )
                    }
                }
            }
            AmitiaSection(title = "系统通知") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "应用更新",
                            subtitle = "新版本可用时通知",
                            checked = settings.updates,
                            onCheckedChange = { onChange(settings.copy(updates = it)) },
                            leadingIcon = AmitiaIcons.Update
                        )
                    }
                }
            }
            AmitiaSection(title = "免打扰") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "免打扰模式",
                            subtitle = "在指定时间段内静默通知",
                            checked = settings.doNotDisturb,
                            onCheckedChange = { onChange(settings.copy(doNotDisturb = it)) },
                            leadingIcon = AmitiaIcons.DoNotDisturbOn
                        )
                        if (settings.doNotDisturb) {
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            SettingsRow(
                                title = "开始时间",
                                subtitle = settings.dndStart,
                                leadingIcon = AmitiaIcons.Schedule,
                                onClick = {}
                            )
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            SettingsRow(
                                title = "结束时间",
                                subtitle = settings.dndEnd,
                                leadingIcon = AmitiaIcons.Schedule,
                                onClick = {}
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "通知预览") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        androidx.compose.material3.Text(
                            text = "艾米",
                            style = androidx.compose.material3.MaterialTheme.typography.titleSmall,
                            color = androidx.compose.material3.MaterialTheme.colorScheme.onSurface
                        )
                        androidx.compose.material3.Text(
                            text = "好的，我明白了。让我来帮你处理这件事。",
                            style = androidx.compose.material3.MaterialTheme.typography.bodyMedium,
                            color = androidx.compose.material3.MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        androidx.compose.material3.Text(
                            text = "刚刚",
                            style = androidx.compose.material3.MaterialTheme.typography.labelSmall,
                            color = androidx.compose.material3.MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "通知设置页 - 亮色", showBackground = true)
@Composable
private fun NotificationSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        NotificationSettingsContent(
            settings = NotificationSettings(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "通知设置页 - 暗色", showBackground = true)
@Composable
private fun NotificationSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        NotificationSettingsContent(
            settings = NotificationSettings(doNotDisturb = true),
            onBack = {},
            onChange = {}
        )
    }
}
