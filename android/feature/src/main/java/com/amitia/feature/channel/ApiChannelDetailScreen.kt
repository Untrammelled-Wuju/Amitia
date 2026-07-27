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
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ApiChannelDetailScreen(
    onBack: () -> Unit,
    viewModel: ApiChannelViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ApiChannelDetailContent(
        state = state,
        onBack = onBack,
        onUpdate = viewModel::update,
        onRetry = viewModel::load
    )
}

@Composable
fun ApiChannelDetailContent(
    state: ScreenState<ApiChannelConfig>,
    onBack: () -> Unit,
    onUpdate: (((ApiChannelConfig) -> ApiChannelConfig)) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "API 渠道", onBack = onBack)
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
                icon = AmitiaIcons.Code,
                title = "API 渠道未配置",
                description = "请创建一个 API 渠道接入",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> ApiBody(config = state.data, onUpdate = onUpdate)
            is ScreenState.Partial -> ApiBody(config = state.data, onUpdate = onUpdate)
        }
    }
}

@Composable
private fun ApiBody(
    config: ApiChannelConfig,
    onUpdate: (((ApiChannelConfig) -> ApiChannelConfig)) -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        ApiHeaderCard(name = config.name, enabled = config.enabled)
        AmitiaSectionHeader(title = "接入信息")
        AmitiaTextField(
            value = config.name,
            onValueChange = { v -> onUpdate { it.copy(name = v) } },
            label = "渠道名称",
            placeholder = "API 接入",
            leadingIcon = AmitiaIcons.Label
        )
        AmitiaTextField(
            value = config.baseUrl,
            onValueChange = { v -> onUpdate { it.copy(baseUrl = v) } },
            label = "Base URL",
            placeholder = "https://api.example.com/v1",
            leadingIcon = AmitiaIcons.Link
        )
        com.amitia.core.designsystem.component.AmitiaPasswordField(
            value = config.apiKey,
            onValueChange = { v -> onUpdate { it.copy(apiKey = v) } },
            label = "API Key",
            placeholder = "输入密钥"
        )
        AmitiaTextField(
            value = config.webhookUrl,
            onValueChange = { v -> onUpdate { it.copy(webhookUrl = v) } },
            label = "Webhook URL",
            placeholder = "https://example.com/webhook",
            leadingIcon = AmitiaIcons.Webhook
        )
        AmitiaSectionHeader(title = "速率与开关")
        AmitiaNumberField(
            value = config.rateLimit.toString(),
            onValueChange = { v -> v.toIntOrNull()?.let { n -> onUpdate { it.copy(rateLimit = n) } } },
            label = "速率限制",
            placeholder = "60",
            unit = "次/分",
            onIncrement = { onUpdate { it.copy(rateLimit = (it.rateLimit + 10).coerceAtMost(600)) } },
            onDecrement = { onUpdate { it.copy(rateLimit = (it.rateLimit - 10).coerceAtLeast(10)) } }
        )
        AmitiaSwitchRow(
            title = "启用该渠道",
            checked = config.enabled,
            onCheckedChange = { v -> onUpdate { it.copy(enabled = v) } },
            subtitle = "关闭后该 API 渠道将停止接收与投递",
            leadingIcon = AmitiaIcons.ToggleOn
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        PrimaryButton(
            text = "保存配置",
            onClick = {},
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Check
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

@Composable
private fun ApiHeaderCard(name: String, enabled: Boolean) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
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
                    imageVector = AmitiaIcons.Code,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = name,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = if (enabled) "已启用" else "已停用",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Preview(name = "ApiChannelDetail - Light", showBackground = true)
@Composable
private fun ApiChannelDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ApiChannelDetailContent(
            state = ScreenState.Content(ChannelMockData.apiConfig),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}

@Preview(name = "ApiChannelDetail - Dark", showBackground = true)
@Composable
private fun ApiChannelDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ApiChannelDetailContent(
            state = ScreenState.Error(com.amitia.core.designsystem.UiError(title = "加载失败", message = "网络异常")),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}
