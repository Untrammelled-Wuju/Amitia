package com.amitia.feature.channel

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.amitiaStatusColor

@Composable
fun ChannelHomeScreen(
    onBack: () -> Unit,
    onOpenChannel: (ChannelType) -> Unit,
    onCreateChannel: () -> Unit,
    viewModel: ChannelHomeViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ChannelHomeContent(
        state = state,
        onBack = onBack,
        onOpenChannel = onOpenChannel,
        onCreateChannel = onCreateChannel,
        onRetry = viewModel::load
    )
}

@Composable
fun ChannelHomeContent(
    state: ScreenState<ChannelHomeData>,
    onBack: () -> Unit,
    onOpenChannel: (ChannelType) -> Unit,
    onCreateChannel: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "渠道中心",
            onBack = onBack,
            actions = {
                AmitiaIconButton(icon = AmitiaIcons.Refresh, contentDescription = "刷新", onClick = onRetry)
            }
        )
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
                icon = AmitiaIcons.Hub,
                title = "尚无渠道",
                description = "接入 Web、微信、QQ 或 API 渠道以开始使用",
                modifier = Modifier.fillMaxSize(),
                primaryAction = { PrimaryButton(text = "新建渠道", onClick = onCreateChannel, leadingIcon = AmitiaIcons.Add) }
            )
            is ScreenState.Content -> ChannelHomeBody(
                data = state.data,
                onOpenChannel = onOpenChannel,
                onCreateChannel = onCreateChannel
            )
            is ScreenState.Partial -> ChannelHomeBody(
                data = state.data,
                onOpenChannel = onOpenChannel,
                onCreateChannel = onCreateChannel
            )
        }
    }
}

@Composable
private fun ChannelHomeBody(
    data: ChannelHomeData,
    onOpenChannel: (ChannelType) -> Unit,
    onCreateChannel: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { OverviewCard(data = data) }
        item { AmitiaSectionHeader(title = "系统渠道") }
        items(data.systemChannels.size) { index ->
            val c = data.systemChannels[index]
            ChannelSummaryCard(channel = c, onClick = { onOpenChannel(c.type) })
        }
        if (data.publicChannels.isNotEmpty()) {
            item { AmitiaSectionHeader(title = "公共插件") }
            items(data.publicChannels.size) { index ->
                val c = data.publicChannels[index]
                ChannelSummaryCard(channel = c, onClick = { onOpenChannel(c.type) })
            }
        }
        item { Spacer(modifier = Modifier.size(AmitiaSpacing.Sm)) }
        item {
            PrimaryButton(
                text = "新建渠道",
                onClick = onCreateChannel,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Add
            )
        }
        item { Spacer(modifier = Modifier.size(AmitiaSpacing.Lg)) }
    }
}

@Composable
private fun OverviewCard(data: ChannelHomeData) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(modifier = Modifier.padding(AmitiaSpacing.Base), horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)) {
            MetricColumn("已绑定", data.totalBound.toString(), AmitiaIcons.Hub)
            MetricColumn("在线", data.totalActive.toString(), AmitiaIcons.Sensors)
            MetricColumn("异常", (data.totalBound - data.totalActive).toString(), AmitiaIcons.WarningAmber)
        }
    }
}

@Composable
private fun RowScope.MetricColumn(label: String, value: String, icon: ImageVector) {
    Column(
        modifier = Modifier.weight(1f),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Icon(imageVector = icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(20.dp))
        Text(text = value, style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Medium)
        Text(text = label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun ChannelSummaryCard(channel: ChannelSummary, onClick: () -> Unit) {
    val statusColor = amitiaStatusColor(channel.statusType)
    Surface(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
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
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = channelIcon(channel.type),
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = channel.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = channel.lastActivity ?: "未连接",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (channel.error != null) {
                    Text(
                        text = channel.error,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
            Column(horizontalAlignment = Alignment.End) {
                Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
                Text(
                    text = if (channel.bound) if (channel.online) "在线" else "离线" else "未绑定",
                    style = MaterialTheme.typography.labelSmall,
                    color = statusColor
                )
            }
        }
    }
}

fun channelIcon(type: ChannelType): ImageVector = when (type) {
    ChannelType.Web -> AmitiaIcons.Webhook
    ChannelType.WeChat -> AmitiaIcons.Chat
    ChannelType.QQ -> AmitiaIcons.Forum
    ChannelType.Api -> AmitiaIcons.Code
    ChannelType.ThirdParty -> AmitiaIcons.Extension
}

@Preview(name = "ChannelHome - Light", showBackground = true)
@Composable
private fun ChannelHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelHomeContent(
            state = ScreenState.Content(ChannelMockData.home),
            onBack = {}, onOpenChannel = {}, onCreateChannel = {}, onRetry = {}
        )
    }
}

@Preview(name = "ChannelHome - Dark", showBackground = true)
@Composable
private fun ChannelHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelHomeContent(
            state = ScreenState.Loading,
            onBack = {}, onOpenChannel = {}, onCreateChannel = {}, onRetry = {}
        )
    }
}
