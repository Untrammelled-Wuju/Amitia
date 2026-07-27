package com.amitia.feature.modelcenter

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ProviderListScreen(
    onBack: () -> Unit,
    onAddProvider: () -> Unit,
    onEditProvider: (String) -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.providersState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadProviders() }
    ProviderListContent(
        state = state,
        onBack = onBack,
        onAddProvider = onAddProvider,
        onEditProvider = onEditProvider,
        onRetry = viewModel::loadProviders
    )
}

@Composable
fun ProviderListContent(
    state: ScreenState<List<ProviderUiModel>>,
    onBack: () -> Unit,
    onAddProvider: () -> Unit,
    onEditProvider: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "Provider 管理",
            onBack = onBack,
            actions = { PrimaryButton(text = "新建", onClick = onAddProvider, leadingIcon = AmitiaIcons.Add) }
        )
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载 Provider...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Hub,
                title = "暂无 Provider",
                description = "添加一个 AI 服务提供商以开始使用模型",
                primaryAction = { PrimaryButton(text = "新建 Provider", onClick = onAddProvider, leadingIcon = AmitiaIcons.Add) },
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(state.data, key = { it.id }) { provider ->
                    ProviderCard(provider = provider, onClick = { onEditProvider(provider.id) })
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun ProviderCard(provider: ProviderUiModel, onClick: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface,
        onClick = onClick
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Hub,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(20.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = provider.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = provider.type,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                ProviderStatusBadge(status = provider.authStatus, available = provider.available)
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
            ) {
                ProviderMetric(label = "可用性", value = if (provider.available) "可用" else "不可用")
                ProviderMetric(label = "角色使用", value = "${provider.roleCount} 个")
                ProviderMetric(label = "最近测试", value = provider.lastTested ?: "未测试")
            }
        }
    }
}

@Composable
private fun ProviderStatusBadge(status: ProviderAuthStatus, available: Boolean) {
    val color = when {
        !available -> AmitiaStateColors.Disconnected
        status == ProviderAuthStatus.Authorized -> AmitiaStateColors.Connected
        status == ProviderAuthStatus.Expired -> AmitiaStateColors.Degraded
        else -> AmitiaStateColors.Pending
    }
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(color))
        Text(
            text = status.label,
            style = MaterialTheme.typography.labelMedium,
            color = color
        )
    }
}

@Composable
private fun ProviderMetric(label: String, value: String) {
    Column {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.labelLarge,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

@Preview(name = "Provider List - Light", showBackground = true)
@Composable
private fun ProviderListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ProviderListContent(
            state = ScreenState.Content(listOf(
                ProviderUiModel("1", "OpenAI", "OpenAI 兼容", ProviderAuthStatus.Authorized, true, "2025-07-20", 3),
                ProviderUiModel("2", "Anthropic", "Claude API", ProviderAuthStatus.Unauthorized, false, null, 0)
            )),
            onBack = {}, onAddProvider = {}, onEditProvider = {}, onRetry = {}
        )
    }
}

@Preview(name = "Provider List - Dark", showBackground = true)
@Composable
private fun ProviderListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ProviderListContent(
            state = ScreenState.Content(listOf(
                ProviderUiModel("1", "OpenAI", "OpenAI 兼容", ProviderAuthStatus.Authorized, true, "2025-07-20", 3)
            )),
            onBack = {}, onAddProvider = {}, onEditProvider = {}, onRetry = {}
        )
    }
}
