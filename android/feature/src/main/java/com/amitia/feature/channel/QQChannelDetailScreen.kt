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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.WarningBanner
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.AmitiaStatusType

@Composable
fun QQChannelDetailScreen(
    onBack: () -> Unit,
    viewModel: QQChannelViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val reconnecting by viewModel.reconnecting.collectAsStateWithLifecycle()
    QQChannelDetailContent(
        state = state,
        reconnecting = reconnecting,
        onBack = onBack,
        onReconnect = viewModel::reconnect,
        onUnbind = { viewModel.unbind(onBack) },
        onRetry = viewModel::load
    )
}

@Composable
fun QQChannelDetailContent(
    state: ScreenState<QQChannelDetail>,
    reconnecting: Boolean,
    onBack: () -> Unit,
    onReconnect: () -> Unit,
    onUnbind: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "QQ 渠道", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Forum,
                title = "QQ 渠道未配置",
                description = "请在渠道中心绑定 QQ",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> QQBody(
                detail = state.data,
                reconnecting = reconnecting,
                onReconnect = onReconnect,
                onUnbind = onUnbind
            )
            is ScreenState.Partial -> QQBody(
                detail = state.data,
                reconnecting = reconnecting,
                onReconnect = onReconnect,
                onUnbind = onUnbind
            )
        }
    }
}

@Composable
private fun QQBody(
    detail: QQChannelDetail,
    reconnecting: Boolean,
    onReconnect: () -> Unit,
    onUnbind: () -> Unit
) {
    val statusType = if (detail.online) AmitiaStatusType.Connected else AmitiaStatusType.Disconnected
    val statusColor = amitiaStatusColor(statusType)
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        QQStatusHeader(title = "QQ", protocol = detail.protocol, bound = detail.bound, online = detail.online, statusColor = statusColor)
        if (detail.messageLinkAbnormal) {
            WarningBanner(
                message = detail.abnormalReason ?: "消息链路异常",
                onAction = onReconnect,
                actionLabel = "重连"
            )
        }
        AmitiaSectionHeader(title = "绑定与二维码")
        QQQrCard(qrCode = detail.qrCode, bound = detail.bound, protocol = detail.protocol)
        AmitiaSectionHeader(title = "运行状态")
        QQInfoRow(icon = AmitiaIcons.Sensors, label = "在线状态", value = if (detail.online) "在线" else "离线", valueColor = statusColor)
        QQInfoRow(icon = AmitiaIcons.Schema, label = "协议", value = detail.protocol)
        QQInfoRow(icon = AmitiaIcons.Favorite, label = "最近心跳", value = detail.lastHeartbeat ?: "无")
        QQInfoRow(icon = AmitiaIcons.Send, label = "最近发送", value = detail.lastSend ?: "无")
        QQInfoRow(icon = AmitiaIcons.ChatBubbleOutline, label = "最近接收", value = detail.lastReceive ?: "无")
        AmitiaSectionHeader(title = "角色分配")
        QQInfoRow(icon = AmitiaIcons.Person, label = "默认角色", value = detail.assignedRole ?: "未分配")
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        LoadingButton(
            text = "重连",
            onClick = onReconnect,
            loading = reconnecting,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Refresh
        )
        SecondaryButton(
            text = "刷新二维码",
            onClick = {},
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.QrCode
        )
        DangerButton(
            text = "解绑 QQ",
            onClick = onUnbind,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.LinkOff
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

@Composable
private fun QQStatusHeader(
    title: String,
    protocol: String,
    bound: Boolean,
    online: Boolean,
    statusColor: androidx.compose.ui.graphics.Color
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Box(
                    modifier = Modifier.size(40.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(imageVector = AmitiaIcons.Forum, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(20.dp))
                }
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Spacer(modifier = Modifier.weight(1f))
                Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
                Text(
                    text = if (bound) if (online) "在线" else "离线" else "未绑定",
                    style = MaterialTheme.typography.labelMedium,
                    color = statusColor
                )
            }
            Text(
                text = "协议：$protocol",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun QQQrCard(qrCode: String?, bound: Boolean, protocol: String) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(160.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                if (bound) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(
                            imageVector = AmitiaIcons.CheckCircle,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(40.dp)
                        )
                        Text(
                            text = "已绑定（$protocol）",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                } else {
                    Icon(
                        imageVector = AmitiaIcons.QrCode,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(80.dp)
                    )
                }
            }
            Text(
                text = qrCode ?: "未生成二维码",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

@Composable
private fun QQInfoRow(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    value: String,
    valueColor: androidx.compose.ui.graphics.Color = MaterialTheme.colorScheme.onSurface
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(MaterialTheme.colorScheme.surface)
            .padding(AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Box(
            modifier = Modifier.size(36.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(imageVector = icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(18.dp))
        }
        Text(text = label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
        Text(text = value, style = MaterialTheme.typography.bodyMedium, color = valueColor, fontWeight = FontWeight.Medium)
    }
}

@Preview(name = "QQChannelDetail - Light", showBackground = true)
@Composable
private fun QQChannelDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        QQChannelDetailContent(
            state = ScreenState.Content(ChannelMockData.qqDetail),
            reconnecting = false,
            onBack = {}, onReconnect = {}, onUnbind = {}, onRetry = {}
        )
    }
}

@Preview(name = "QQChannelDetail - Dark", showBackground = true)
@Composable
private fun QQChannelDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        QQChannelDetailContent(
            state = ScreenState.Empty(),
            reconnecting = false,
            onBack = {}, onReconnect = {}, onUnbind = {}, onRetry = {}
        )
    }
}
