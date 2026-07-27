package com.amitia.feature.character.detail

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Hub
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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.ChannelBinding

@Composable
fun CharacterChannelTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.channelState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadChannels() }
        )
        is ScreenState.Content -> ChannelContent(
            channels = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun ChannelContent(
    channels: List<ChannelBinding>,
    modifier: Modifier = Modifier
) {
    val boundChannels = channels.filter { it.bound }
    val unboundChannels = channels.filter { !it.bound }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "summary") {
            ChannelSummaryCard(channels)
        }
        if (boundChannels.isNotEmpty()) {
            item(key = "bound_section") {
                AmitiaSection(title = "已绑定渠道", subtitle = "${boundChannels.size} 个渠道已连接") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        boundChannels.forEach { channel ->
                            ChannelRow(channel)
                        }
                    }
                }
            }
        }
        if (unboundChannels.isNotEmpty()) {
            item(key = "unbound_section") {
                AmitiaSection(title = "可绑定渠道", subtitle = "点击绑定更多渠道") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        unboundChannels.forEach { channel ->
                            ChannelRow(channel)
                        }
                    }
                }
            }
        }
        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "刷新状态",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "保存配置",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun ChannelSummaryCard(channels: List<ChannelBinding>) {
    val boundCount = channels.count { it.bound }
    val onlineCount = channels.count { it.online }
    val errorCount = channels.count { it.errorStatus != null }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            SummaryStat("已绑定", "$boundCount", MaterialTheme.colorScheme.onPrimaryContainer)
            SummaryStat("在线", "$onlineCount", MaterialTheme.colorScheme.onPrimaryContainer)
            SummaryStat("异常", "$errorCount", MaterialTheme.colorScheme.onPrimaryContainer)
        }
    }
}

@Composable
private fun SummaryStat(label: String, value: String, color: androidx.compose.ui.graphics.Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            style = MaterialTheme.typography.headlineSmall,
            color = color,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = color.copy(alpha = 0.7f)
        )
    }
}

@Composable
private fun ChannelRow(channel: ChannelBinding) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(
                        if (channel.bound) MaterialTheme.colorScheme.primaryContainer
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Hub,
                    contentDescription = null,
                    tint = if (channel.bound) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = channel.name,
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.size(8.dp))
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .clip(CircleShape)
                            .background(
                                when {
                                    !channel.bound -> MaterialTheme.colorScheme.outlineVariant
                                    channel.errorStatus != null -> MaterialTheme.colorScheme.error
                                    channel.online -> MaterialTheme.colorScheme.primary
                                    else -> MaterialTheme.colorScheme.outlineVariant
                                }
                            )
                    )
                }
                if (channel.lastSendTime != null) {
                    Text(
                        text = "最后发送：${channel.lastSendTime}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                }
                if (channel.errorStatus != null) {
                    Text(
                        text = channel.errorStatus,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
            Surface(
                shape = RoundedCornerShape(8.dp),
                color = if (channel.bound) MaterialTheme.colorScheme.tertiaryContainer
                else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = if (channel.bound) {
                        if (channel.online) "在线" else "离线"
                    } else "未绑定",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (channel.bound) MaterialTheme.colorScheme.onTertiaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Preview(name = "Channel - Light", showBackground = true)
@Composable
private fun CharacterChannelLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ChannelContent(
                channels = listOf(
                    ChannelBinding("1", "网页端", "Web", true, true, "10分钟前", "5分钟前", null),
                    ChannelBinding("2", "QQ", "QQ", false, false, null, null, "未绑定")
                )
            )
        }
    }
}

@Preview(name = "Channel - Dark", showBackground = true)
@Composable
private fun CharacterChannelDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ChannelContent(channels = listOf())
        }
    }
}
