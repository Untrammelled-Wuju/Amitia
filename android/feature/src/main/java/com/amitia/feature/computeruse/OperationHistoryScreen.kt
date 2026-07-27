package com.amitia.feature.computeruse

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
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun OperationHistoryScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val history by viewModel.history.collectAsStateWithLifecycle()
    val masked by viewModel.historyMasked.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    OperationHistoryContent(
        history = history,
        masked = masked,
        loading = loading,
        onBack = onBack,
        onToggleMasking = viewModel::toggleHistoryMasking
    )
}

@Composable
fun OperationHistoryContent(
    history: List<OperationHistoryEntry>,
    masked: Boolean,
    loading: Boolean,
    onBack: () -> Unit,
    onToggleMasking: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "操作历史",
            onBack = onBack,
            actions = {
                AmitiaIconButton(
                    icon = if (masked) AmitiaIcons.Visibility else AmitiaIcons.VisibilityOff,
                    contentDescription = if (masked) "显示敏感内容" else "隐藏敏感内容",
                    onClick = onToggleMasking
                )
            }
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "hint") {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
                ) {
                    Text(
                        text = if (masked) "敏感内容已脱敏，点击右上角图标可显示" else "敏感内容已显示，请注意环境安全",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
            if (loading) {
                item(key = "loading") {
                    Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                        InlineLoading(message = "加载操作历史...")
                    }
                }
            } else if (history.isEmpty()) {
                item(key = "empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.History,
                        title = "暂无操作记录",
                        description = "Computer Use 执行的操作将记录在这里",
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            } else {
                item(key = "header") {
                    AmitiaSectionHeader(title = "历史操作", trailing = {
                        Text(
                            text = "${history.size} 条",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    })
                }
                items(history, key = { it.id }) { entry ->
                    OperationHistoryRow(entry = entry, masked = masked)
                }
            }
        }
    }
}

@Composable
private fun OperationHistoryRow(entry: OperationHistoryEntry, masked: Boolean) {
    val resultColor = when (entry.result) {
        OperationResult.Success -> MaterialTheme.colorScheme.tertiary
        OperationResult.Failed -> MaterialTheme.colorScheme.error
        OperationResult.Blocked -> MaterialTheme.colorScheme.error
        OperationResult.Cancelled -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    val resultIcon = when (entry.result) {
        OperationResult.Success -> AmitiaIcons.CheckCircle
        OperationResult.Failed -> AmitiaIcons.Error
        OperationResult.Blocked -> AmitiaIcons.Block
        OperationResult.Cancelled -> AmitiaIcons.Close
    }
    val displayApp = if (entry.sensitive && masked) maskText(entry.app) else entry.app
    val displayOperation = if (entry.sensitive && masked) maskText(entry.operation) else entry.operation
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape).background(resultColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(imageVector = resultIcon, contentDescription = null, tint = resultColor, modifier = Modifier.size(AmitiaIconSize.Medium))
            }
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = entry.time,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                    Text(
                        text = entry.role,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
                    )
                    if (entry.sensitive) {
                        Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.3f)) {
                            Text(
                                text = "敏感",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.error,
                                modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)
                            )
                        }
                    }
                }
                Text(
                    text = "$displayApp · $displayOperation",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 2, overflow = TextOverflow.Ellipsis,
                    fontWeight = FontWeight.Medium
                )
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = "结果：${entry.result.label}",
                        style = MaterialTheme.typography.labelSmall,
                        color = resultColor
                    )
                    Text(
                        text = "审批：${entry.approvalMethod.label}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

private fun maskText(text: String): String {
    return if (text.length <= 2) "****" else text.first() + "****" + text.last()
}

@Preview(name = "Operation History - Light", showBackground = true)
@Composable
private fun OperationHistoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        OperationHistoryContent(
            history = listOf(
                OperationHistoryEntry("h1", "14:30", "艾米", "文件管理器", "创建文件夹", OperationResult.Success, ApprovalMethod.ManualOnce, sensitive = false),
                OperationHistoryEntry("h2", "14:28", "艾米", "浏览器", "打开网页", OperationResult.Success, ApprovalMethod.Auto, sensitive = false),
                OperationHistoryEntry("h3", "14:25", "艾米", "支付宝", "发起转账", OperationResult.Blocked, ApprovalMethod.Denied, sensitive = true)
            ),
            masked = true, loading = false, onBack = {}, onToggleMasking = {}
        )
    }
}

@Preview(name = "Operation History - Dark", showBackground = true)
@Composable
private fun OperationHistoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        OperationHistoryContent(
            history = emptyList(), masked = false, loading = true, onBack = {}, onToggleMasking = {}
        )
    }
}
