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
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ModelDiagnosticScreen(
    onBack: () -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.diagnosticsState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadDiagnostics() }
    ModelDiagnosticContent(state = state, onBack = onBack, onRetry = viewModel::loadDiagnostics)
}

@Composable
fun ModelDiagnosticContent(
    state: ScreenState<List<DiagnosticItemUiModel>>,
    onBack: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "模型诊断", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在诊断...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.BugReport,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.BugReport,
                title = "暂无诊断结果",
                description = "请先配置模型后再进行诊断",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> {
                val grouped = state.data.groupBy { it.category }
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    item(key = "summary") { DiagnosticSummary(items = state.data) }
                    grouped.forEach { (category, items) ->
                        item(key = "header_${category.name}") {
                            AmitiaSection(title = category.label) {}
                        }
                        items(items, key = { it.id }) { item ->
                            DiagnosticCard(item = item)
                        }
                    }
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun DiagnosticSummary(items: List<DiagnosticItemUiModel>) {
    val passCount = items.count { it.status == DiagnosticStatus.Pass }
    val warnCount = items.count { it.status == DiagnosticStatus.Warning }
    val failCount = items.count { it.status == DiagnosticStatus.Failed }
    val skipCount = items.count { it.status == DiagnosticStatus.Skipped }

    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.HealthAndSafety,
                    contentDescription = null,
                    tint = if (failCount > 0) AmitiaStateColors.Failed
                    else if (warnCount > 0) AmitiaStateColors.Degraded
                    else AmitiaStateColors.Running,
                    modifier = Modifier.size(24.dp)
                )
                Text(
                    text = "诊断概览",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = if (failCount == 0 && warnCount == 0) "全部正常" else "发现 $failCount 异常, $warnCount 警告",
                    style = MaterialTheme.typography.bodySmall,
                    color = if (failCount > 0) MaterialTheme.colorScheme.error
                    else if (warnCount > 0) MaterialTheme.colorScheme.tertiary
                    else AmitiaStateColors.Running
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                SummaryChip("正常", passCount, AmitiaStateColors.Running, Modifier.weight(1f))
                SummaryChip("警告", warnCount, AmitiaStateColors.Degraded, Modifier.weight(1f))
                SummaryChip("异常", failCount, AmitiaStateColors.Failed, Modifier.weight(1f))
                SummaryChip("跳过", skipCount, AmitiaStateColors.Idle, Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun SummaryChip(label: String, count: Int, color: androidx.compose.ui.graphics.Color, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        color = color.copy(alpha = 0.12f)
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "$count",
                style = MaterialTheme.typography.titleLarge,
                color = color,
                fontWeight = FontWeight.Bold
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun DiagnosticCard(item: DiagnosticItemUiModel) {
    val statusColor = when (item.status) {
        DiagnosticStatus.Pass -> AmitiaStateColors.Running
        DiagnosticStatus.Warning -> AmitiaStateColors.Degraded
        DiagnosticStatus.Failed -> AmitiaStateColors.Failed
        DiagnosticStatus.Skipped -> AmitiaStateColors.Idle
    }
    val statusIcon = when (item.status) {
        DiagnosticStatus.Pass -> AmitiaIcons.CheckCircle
        DiagnosticStatus.Warning -> AmitiaIcons.WarningAmber
        DiagnosticStatus.Failed -> AmitiaIcons.Error
        DiagnosticStatus.Skipped -> AmitiaIcons.ArrowForward
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape).background(statusColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = statusIcon,
                    contentDescription = null,
                    tint = statusColor,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier.weight(1f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Surface(
                        shape = RoundedCornerShape(8.dp),
                        color = statusColor.copy(alpha = 0.15f)
                    ) {
                        Text(
                            text = item.status.label,
                            style = MaterialTheme.typography.labelSmall,
                            color = statusColor,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                Text(
                    text = item.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                item.detail?.let { detail ->
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Text(
                        text = detail,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                    )
                }
                item.suggestion?.let { suggestion ->
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Surface(
                        shape = RoundedCornerShape(8.dp),
                        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Lightbulb,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.size(14.dp)
                            )
                            Text(
                                text = suggestion,
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onTertiaryContainer
                            )
                        }
                    }
                }
            }
        }
    }
}

@Preview(name = "ModelDiagnostic - Light", showBackground = true)
@Composable
private fun ModelDiagnosticLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModelDiagnosticContent(
            state = ScreenState.Content(
                listOf(
                    DiagnosticItemUiModel(
                        id = "auth",
                        title = "API 密钥认证",
                        category = DiagnosticCategory.Auth,
                        status = DiagnosticStatus.Pass,
                        description = "检查所有 Provider 的 API 密钥是否有效"
                    ),
                    DiagnosticItemUiModel(
                        id = "rate_limit",
                        title = "速率限制检查",
                        category = DiagnosticCategory.RateLimit,
                        status = DiagnosticStatus.Warning,
                        description = "检查是否接近 Provider 的速率限制",
                        detail = "当前使用率 85%",
                        suggestion = "建议升级套餐或降低请求频率"
                    ),
                    DiagnosticItemUiModel(
                        id = "fallback",
                        title = "回退链验证",
                        category = DiagnosticCategory.Fallback,
                        status = DiagnosticStatus.Failed,
                        description = "验证回退链配置是否完整",
                        suggestion = "建议配置至少一个回退模型"
                    )
                )
            ),
            onBack = {},
            onRetry = {}
        )
    }
}

@Preview(name = "ModelDiagnostic - Dark", showBackground = true)
@Composable
private fun ModelDiagnosticDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModelDiagnosticContent(
            state = ScreenState.Loading,
            onBack = {},
            onRetry = {}
        )
    }
}
