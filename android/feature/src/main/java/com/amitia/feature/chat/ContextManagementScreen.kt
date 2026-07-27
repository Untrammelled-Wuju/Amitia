package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
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
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ContextManagementScreen(
    conversationId: String,
    onBack: () -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(conversationId) { viewModel.loadContext(conversationId) }
    val state by viewModel.contextState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "上下文管理", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            actions = {
                AmitiaIconButton(icon = AmitiaIcons.Tune, contentDescription = "优化", onClick = {})
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 5, lineHeight = 48)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadContext(conversationId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Layers,
                title = "暂无上下文",
                description = "开始对话后，上下文信息会显示在这里"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val context = (s as ScreenState.Content<ContextSummary>).data
                ContextContent(
                    context = context,
                    onToggle = viewModel::toggleContextItem
                )
            }
        }
    }
}

@Composable
private fun ContextContent(
    context: ContextSummary,
    onToggle: (String) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        item(key = "token_usage") {
            TokenUsageCard(totalTokens = context.totalTokens, maxTokens = context.maxTokens)
        }
        item(key = "context_header") {
            AmitiaSectionHeader(title = "上下文条目 (${context.items.size})")
        }
        items(context.items, key = { it.id }) { item ->
            ContextItemRow(item = item, onToggle = { onToggle(item.id) })
        }
    }
}

@Composable
private fun TokenUsageCard(totalTokens: Int, maxTokens: Int) {
    val progress = (totalTokens.toFloat() / maxTokens).coerceIn(0f, 1f)
    val usagePercent = (progress * 100).toInt()
    val progressColor = when {
        progress < 0.6f -> MaterialTheme.colorScheme.primary
        progress < 0.85f -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.error
    }

    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(
                    imageVector = AmitiaIcons.Memory,
                    contentDescription = null,
                    tint = progressColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
                Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                Text(
                    text = "Token 使用量",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = "$usagePercent%",
                    style = MaterialTheme.typography.titleMedium,
                    color = progressColor,
                    fontWeight = FontWeight.Bold
                )
            }
            LinearProgressIndicator(
                progress = { progress },
                modifier = Modifier.fillMaxWidth().height(8.dp).clip(AmitiaCardShape),
                color = progressColor,
                trackColor = MaterialTheme.colorScheme.surfaceVariant
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "$totalTokens / $maxTokens Tokens",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = "${maxTokens - totalTokens} 剩余",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                )
            }
        }
    }
}

@Composable
private fun ContextItemRow(
    item: ContextItem,
    onToggle: () -> Unit
) {
    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier.size(AmitiaIconSize.Large).clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = contextIconFor(item.type),
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = item.preview,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${item.type.label} | ${item.tokenCount} Tokens",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
            Switch(
                checked = item.included,
                onCheckedChange = { onToggle() },
                enabled = item.removable
            )
        }
    }
}

private fun contextIconFor(type: ContextType) = when (type) {
    ContextType.Character -> AmitiaIcons.Person
    ContextType.WorldBook -> AmitiaIcons.Book
    ContextType.RecentMessage -> AmitiaIcons.Chat
    ContextType.LongTermMemory -> AmitiaIcons.Memory
    ContextType.FileContext -> AmitiaIcons.FileCopy
    ContextType.ToolResult -> AmitiaIcons.Build
}

@Preview(name = "Context Management - Light", showBackground = true)
@Composable
private fun ContextManagementLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            TokenUsageCard(totalTokens = 8200, maxTokens = 32000)
        }
    }
}

@Preview(name = "Context Management - Dark", showBackground = true)
@Composable
private fun ContextManagementDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            TokenUsageCard(totalTokens = 28000, maxTokens = 32000)
        }
    }
}
