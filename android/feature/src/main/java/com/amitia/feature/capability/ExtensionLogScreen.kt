package com.amitia.feature.capability

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun ExtensionLogScreen(
    onBack: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val logs by viewModel.logs.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    val extensions = remember(logs) { logs.map { it.extensionName }.distinct() }
    var selectedExtension by remember { mutableStateOf("") }
    var masked by remember { mutableStateOf(true) }
    val filtered = if (selectedExtension.isBlank()) logs else logs.filter { it.extensionName == selectedExtension }
    ExtensionLogContent(
        logs = filtered,
        extensions = extensions,
        selectedExtension = selectedExtension,
        masked = masked,
        loading = loading,
        onBack = onBack,
        onSelectExtension = {
            selectedExtension = it
            viewModel.filterLogsByExtension(it)
        },
        onToggleMask = { masked = it }
    )
}

@Composable
fun ExtensionLogContent(
    logs: List<ExtensionLogEntry>,
    extensions: List<String>,
    selectedExtension: String,
    masked: Boolean,
    loading: Boolean,
    onBack: () -> Unit,
    onSelectExtension: (String) -> Unit,
    onToggleMask: (Boolean) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "扩展运行日志", onBack = onBack)
        Column(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)) {
            AmitiaSectionHeader(title = "按扩展筛选")
            if (extensions.isEmpty()) {
                Text(
                    text = "暂无可筛选扩展",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            } else {
                androidx.compose.foundation.lazy.LazyRow(
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    item(key = "all") {
                        FilterChip(
                            label = "全部",
                            selected = selectedExtension.isBlank(),
                            onClick = { onSelectExtension("") }
                        )
                    }
                    items(extensions, key = { it }) { name ->
                        FilterChip(
                            label = name,
                            selected = selectedExtension == name,
                            onClick = { onSelectExtension(name) }
                        )
                    }
                }
            }
            AmitiaSwitchRow(
                title = "默认脱敏",
                subtitle = "隐藏日志中的敏感内容",
                checked = masked,
                onCheckedChange = onToggleMask,
                leadingIcon = AmitiaIcons.VisibilityOff
            )
        }
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载日志...")
            }
            logs.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.History,
                title = "暂无日志",
                description = "扩展运行后将在此展示日志、事件、Hook、任务和错误",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
            ) {
                items(logs, key = { it.id }) { entry ->
                    LogEntryRow(entry = entry, masked = masked)
                }
            }
        }
    }
}

@Composable
private fun FilterChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        modifier = Modifier.clip(RoundedCornerShape(8.dp)),
        shape = RoundedCornerShape(8.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surfaceVariant
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
            else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
        )
    }
}

@Composable
private fun LogEntryRow(entry: ExtensionLogEntry, masked: Boolean) {
    val levelColor = when (entry.level) {
        ExtensionLogLevel.Debug -> MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
        ExtensionLogLevel.Info -> MaterialTheme.colorScheme.onSurfaceVariant
        ExtensionLogLevel.Warning -> MaterialTheme.colorScheme.tertiary
        ExtensionLogLevel.Error -> MaterialTheme.colorScheme.error
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Text(
            text = entry.timestamp,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
            modifier = Modifier.width(64.dp),
            fontFamily = FontFamily.Monospace
        )
        Box(
            modifier = Modifier
                .width(48.dp)
                .clip(RoundedCornerShape(4.dp))
                .background(levelColor.copy(alpha = 0.15f)),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = entry.level.label,
                style = MaterialTheme.typography.labelSmall,
                color = levelColor,
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Xs)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "${entry.extensionName} · ${entry.source}",
                style = MaterialTheme.typography.labelSmall,
                color = levelColor,
                maxLines = 1
            )
            Text(
                text = if (masked) maskContent(entry.message) else entry.message,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3, overflow = TextOverflow.Ellipsis
            )
        }
    }
}

private fun maskContent(content: String): String {
    return content.replace(Regex("(?<=.{2}).(?=.{2})"), "*")
}

@Preview(name = "Extension Log - Light", showBackground = true)
@Composable
private fun ExtensionLogLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ExtensionLogContent(
            logs = listOf(
                ExtensionLogEntry("1", "14:30:01", ExtensionLogLevel.Info, "Plugin", "插件已启动，连接到 192.168.1.1", "天气查询"),
                ExtensionLogEntry("2", "14:31:00", ExtensionLogLevel.Error, "MCP", "连接超时 token=abc123", "数据库查询")
            ),
            extensions = listOf("天气查询", "数据库查询"),
            selectedExtension = "", masked = true, loading = false,
            onBack = {}, onSelectExtension = {}, onToggleMask = {}
        )
    }
}

@Preview(name = "Extension Log - Dark", showBackground = true)
@Composable
private fun ExtensionLogDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ExtensionLogContent(
            logs = emptyList(), extensions = emptyList(), selectedExtension = "", masked = true, loading = true,
            onBack = {}, onSelectExtension = {}, onToggleMask = {}
        )
    }
}
