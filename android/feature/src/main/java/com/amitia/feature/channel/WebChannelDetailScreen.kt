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
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaTextField

@Composable
fun WebChannelDetailScreen(
    onBack: () -> Unit,
    viewModel: WebChannelViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    WebChannelDetailContent(
        state = state,
        onBack = onBack,
        onUpdate = viewModel::update,
        onRetry = viewModel::load
    )
}

@Composable
fun WebChannelDetailContent(
    state: ScreenState<WebChannelConfig>,
    onBack: () -> Unit,
    onUpdate: (((WebChannelConfig) -> WebChannelConfig)) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "Web 渠道", onBack = onBack)
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
                icon = AmitiaIcons.Webhook,
                title = "Web 渠道不可用",
                description = "请检查运行时是否已启用 Web 渠道",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> WebBody(config = state.data, onUpdate = onUpdate)
            is ScreenState.Partial -> WebBody(config = state.data, onUpdate = onUpdate)
        }
    }
}

@Composable
private fun WebBody(
    config: WebChannelConfig,
    onUpdate: (((WebChannelConfig) -> WebChannelConfig)) -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        FixedEnabledCard()
        AmitiaSectionHeader(title = "当前会话")
        InfoCard(
            icon = AmitiaIcons.Lan,
            label = "会话标识",
            value = config.currentSession
        )
        AmitiaSectionHeader(title = "消息同步")
        AmitiaSwitchRow(
            title = "消息同步",
            checked = config.messageSync,
            onCheckedChange = { v -> onUpdate { it.copy(messageSync = v) } },
            subtitle = "Web 与其他渠道消息保持一致",
            leadingIcon = AmitiaIcons.Sync
        )
        AmitiaSectionHeader(title = "连续消息合并规则")
        listOf(
            "60 秒内连续消息合并",
            "30 秒内连续消息合并",
            "不合并连续消息"
        ).forEach { rule ->
            AmitiaSelectionRow(
                title = rule,
                selected = config.mergeRule == rule,
                onSelect = { onUpdate { it.copy(mergeRule = rule) } },
                leadingIcon = AmitiaIcons.Layers
            )
        }
        AmitiaSectionHeader(title = "通知")
        AmitiaSwitchRow(
            title = "Web 通知",
            checked = config.notifications,
            onCheckedChange = { v -> onUpdate { it.copy(notifications = v) } },
            subtitle = "Web 端新消息通知",
            leadingIcon = AmitiaIcons.Notifications
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

@Composable
private fun FixedEnabledCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Lock,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "Web 聊天已固定开启",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "Web 渠道为基础渠道，无法关闭",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun InfoCard(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, value: String) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(imageVector = icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(18.dp))
            }
            Text(
                text = label,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f)
            )
            Text(
                text = value,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

@Preview(name = "WebChannelDetail - Light", showBackground = true)
@Composable
private fun WebChannelDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        WebChannelDetailContent(
            state = ScreenState.Content(ChannelMockData.webConfig),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}

@Preview(name = "WebChannelDetail - Dark", showBackground = true)
@Composable
private fun WebChannelDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        WebChannelDetailContent(
            state = ScreenState.Loading,
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}
