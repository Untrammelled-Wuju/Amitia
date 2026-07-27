package com.amitia.feature.channel

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar

@Composable
fun ChannelNotificationSettingsScreen(
    onBack: () -> Unit,
    viewModel: ChannelNotificationViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ChannelNotificationContent(
        state = state,
        onBack = onBack,
        onUpdate = viewModel::update,
        onRetry = viewModel::load
    )
}

@Composable
fun ChannelNotificationContent(
    state: ScreenState<ChannelNotificationSettings>,
    onBack: () -> Unit,
    onUpdate: ((ChannelNotificationSettings) -> ChannelNotificationSettings) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "渠道通知设置", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.NotificationsOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Notifications,
                title = "暂无通知设置",
                description = "请先绑定一个渠道",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> NotificationBody(config = state.data, onUpdate = onUpdate)
            is ScreenState.Partial -> NotificationBody(config = state.data, onUpdate = onUpdate)
        }
    }
}

@Composable
private fun NotificationBody(
    config: ChannelNotificationSettings,
    onUpdate: ((ChannelNotificationSettings) -> ChannelNotificationSettings) -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        NotificationHeaderCard(activeCount = countActive(config))
        AmitiaSectionHeader(title = "消息提醒")
        AmitiaSwitchRow(
            title = "新消息提醒",
            subtitle = "收到新消息时发出通知",
            checked = config.newMessage,
            onCheckedChange = { v -> onUpdate { it.copy(newMessage = v) } },
            leadingIcon = AmitiaIcons.Notifications
        )
        AmitiaSwitchRow(
            title = "失败提醒",
            subtitle = "消息投递失败时发出通知",
            checked = config.failureReminder,
            onCheckedChange = { v -> onUpdate { it.copy(failureReminder = v) } },
            leadingIcon = AmitiaIcons.ErrorOutline
        )
        AmitiaSwitchRow(
            title = "离线提醒",
            subtitle = "渠道离线或断开连接时提醒",
            checked = config.offlineReminder,
            onCheckedChange = { v -> onUpdate { it.copy(offlineReminder = v) } },
            leadingIcon = AmitiaIcons.CloudOff
        )
        AmitiaSectionHeader(title = "消息保护")
        AmitiaSwitchRow(
            title = "重复消息保护",
            subtitle = "检测并提示可能重复的消息",
            checked = config.duplicateProtection,
            onCheckedChange = { v -> onUpdate { it.copy(duplicateProtection = v) } },
            leadingIcon = AmitiaIcons.Shield
        )
        AmitiaSectionHeader(title = "角色行为")
        AmitiaSwitchRow(
            title = "角色主动消息",
            subtitle = "允许角色在合适时机主动发起消息",
            checked = config.roleProactive,
            onCheckedChange = { v -> onUpdate { it.copy(roleProactive = v) } },
            leadingIcon = AmitiaIcons.AutoAwesome
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        NotificationTipsCard()
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

private fun countActive(config: ChannelNotificationSettings): Int {
    var count = 0
    if (config.newMessage) count++
    if (config.failureReminder) count++
    if (config.offlineReminder) count++
    if (config.duplicateProtection) count++
    if (config.roleProactive) count++
    return count
}

@Composable
private fun NotificationHeaderCard(activeCount: Int) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.12f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Notifications,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "通知策略",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "$activeCount 项已开启",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.85f)
                )
            }
        }
    }
}

@Composable
private fun NotificationTipsCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Info,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(20.dp)
            )
            Text(
                text = "通知设置仅影响本设备的通知提醒，不会影响消息的接收与处理。关闭重复消息保护可能导致重复推送。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Preview(name = "ChannelNotification - Light", showBackground = true)
@Composable
private fun ChannelNotificationLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelNotificationContent(
            state = ScreenState.Content(ChannelMockData.notificationSettings),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}

@Preview(name = "ChannelNotification - Dark", showBackground = true)
@Composable
private fun ChannelNotificationDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelNotificationContent(
            state = ScreenState.Content(
                ChannelNotificationSettings(
                    newMessage = false,
                    failureReminder = true,
                    offlineReminder = false,
                    duplicateProtection = true,
                    roleProactive = true
                )
            ),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}
