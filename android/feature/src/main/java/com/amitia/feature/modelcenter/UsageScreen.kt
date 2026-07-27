package com.amitia.feature.modelcenter

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
import androidx.compose.ui.graphics.vector.ImageVector
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
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun UsageScreen(
    onBack: () -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.usageState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadUsage() }
    UsageContent(state = state, onBack = onBack, onRetry = viewModel::loadUsage)
}

@Composable
fun UsageContent(
    state: ScreenState<UsageStatsUiModel>,
    onBack: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "用量统计", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "加载用量数据...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Analytics,
                title = "暂无用量数据",
                description = "模型调用后将在此展示用量统计",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "time_range") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(16.dp),
                        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
                    ) {
                        Row(
                            modifier = Modifier.padding(AmitiaSpacing.Base),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Schedule,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.primary,
                                modifier = Modifier.size(20.dp)
                            )
                            Text(
                                text = state.data.timeRange,
                                style = MaterialTheme.typography.titleSmall,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                    }
                }
                item(key = "stats_grid") {
                    StatsGrid(stats = state.data)
                }
                item(key = "char_header") {
                    AmitiaSection(title = "角色用量分布") {
                        if (state.data.characterDistribution.isEmpty()) {
                            Text(
                                text = "暂无角色用量数据",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(vertical = AmitiaSpacing.Base)
                            )
                        }
                    }
                }
                items(state.data.characterDistribution, key = { it.characterName }) { charUsage ->
                    CharacterUsageRow(charUsage)
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun StatsGrid(stats: UsageStatsUiModel) {
    val items = listOf(
        StatItem("请求数", "${stats.totalRequests}", AmitiaIcons.Send, AmitiaStateColors.Running),
        StatItem("Token 用量", "${stats.totalTokens}", AmitiaIcons.Memory, AmitiaStateColors.Degraded),
        StatItem("平均延迟", "${stats.avgLatencyMs}ms", AmitiaIcons.Speed, AmitiaStateColors.Running),
        StatItem("失败率", "${(stats.failureRate * 100).toInt()}%", AmitiaIcons.Error, if (stats.failureRate > 0.1f) AmitiaStateColors.Failed else AmitiaStateColors.Running)
    )
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
        items.chunked(2).forEach { rowItems ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                rowItems.forEach { item ->
                    StatCard(
                        item = item,
                        modifier = Modifier.weight(1f)
                    )
                }
                if (rowItems.size == 1) Spacer(modifier = Modifier.weight(1f))
            }
        }
    }
}

private data class StatItem(
    val label: String,
    val value: String,
    val icon: ImageVector,
    val color: androidx.compose.ui.graphics.Color
)

@Composable
private fun StatCard(item: StatItem, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Box(
                    modifier = Modifier.size(28.dp).clip(CircleShape).background(item.color.copy(alpha = 0.15f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = item.icon,
                        contentDescription = null,
                        tint = item.color,
                        modifier = Modifier.size(16.dp)
                    )
                }
                Text(
                    text = item.label,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = item.value,
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Composable
private fun CharacterUsageRow(charUsage: CharacterUsageUiModel) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier.size(32.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Person,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(16.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = charUsage.characterName,
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "${charUsage.requestCount} 次请求 · ${charUsage.tokenCount} tokens",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = "${(charUsage.percentage * 100).toInt()}%",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            Box(
                modifier = Modifier.fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Box(
                    modifier = Modifier.fillMaxWidth(charUsage.percentage).height(4.dp)
                        .clip(RoundedCornerShape(2.dp)).background(MaterialTheme.colorScheme.primary)
                )
            }
        }
    }
}

@Preview(name = "Usage - Light", showBackground = true)
@Composable
private fun UsageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        UsageContent(
            state = ScreenState.Content(
                UsageStatsUiModel(
                    totalRequests = 15234,
                    totalTokens = 8945623,
                    avgLatencyMs = 234,
                    failureRate = 0.03f,
                    timeRange = "近 7 天",
                    characterDistribution = listOf(
                        CharacterUsageUiModel("艾米", 5234, 3200000, 0.34f),
                        CharacterUsageUiModel("林夕", 4000, 2800000, 0.26f),
                        CharacterUsageUiModel("星河", 3000, 1800000, 0.20f)
                    )
                )
            ),
            onBack = {},
            onRetry = {}
        )
    }
}

@Preview(name = "Usage - Dark", showBackground = true)
@Composable
private fun UsageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        UsageContent(
            state = ScreenState.Loading,
            onBack = {},
            onRetry = {}
        )
    }
}
