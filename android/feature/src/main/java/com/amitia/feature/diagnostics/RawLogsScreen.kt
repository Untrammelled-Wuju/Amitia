package com.amitia.feature.diagnostics

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.LoadingSkeleton

private enum class LogFilter { All, Debug, Info, Warning, Error }

@Composable
fun RawLogsScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var searchQuery by remember { mutableStateOf("") }
    var filter by remember { mutableStateOf(LogFilter.All) }
    var paused by remember { mutableStateOf(false) }

    val filteredLogs = when (val ps = state.logs) {
        is ScreenState.Content -> ps.data.filter { entry ->
            val matchesLevel = when (filter) {
                LogFilter.All -> true
                LogFilter.Debug -> entry.level == DiagLogLevel.Debug
                LogFilter.Info -> entry.level == DiagLogLevel.Info
                LogFilter.Warning -> entry.level == DiagLogLevel.Warning
                LogFilter.Error -> entry.level == DiagLogLevel.Error
            }
            val matchesSearch = searchQuery.isBlank() ||
                entry.message.contains(searchQuery, ignoreCase = true) ||
                entry.source.contains(searchQuery, ignoreCase = true)
            matchesLevel && matchesSearch
        }
        else -> emptyList()
    }

    Column(modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl)) {
        DiagSectionTitle("原始日志")
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        OutlinedTextField(
            value = searchQuery,
            onValueChange = { searchQuery = it },
            modifier = Modifier.fillMaxWidth(),
            placeholder = { Text("搜索日志...") },
            leadingIcon = {
                Icon(
                    imageVector = AmitiaIcons.Search,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            },
            singleLine = true,
            colors = TextFieldDefaults.colors(
                focusedContainerColor = Color.Transparent,
                unfocusedContainerColor = Color.Transparent,
                focusedIndicatorColor = MaterialTheme.colorScheme.primary,
                unfocusedIndicatorColor = MaterialTheme.colorScheme.outlineVariant
            )
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            LogFilter.entries.forEach { level ->
                val selected = filter == level
                val label = when (level) {
                    LogFilter.All -> "全部"
                    LogFilter.Debug -> "DEBUG"
                    LogFilter.Info -> "INFO"
                    LogFilter.Warning -> "WARN"
                    LogFilter.Error -> "ERROR"
                }
                Surface(
                    shape = AmitiaPillShape,
                    color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant,
                    modifier = Modifier.clickable { filter = level }
                ) {
                    Text(
                        text = label,
                        style = MaterialTheme.typography.labelSmall,
                        color = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
                    )
                }
            }
            Spacer(modifier = Modifier.weight(1f))
            Surface(
                shape = AmitiaPillShape,
                color = if (paused) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.surfaceVariant,
                modifier = Modifier.clickable { paused = !paused }
            ) {
                Text(
                    text = if (paused) "已暂停" else "暂停滚动",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (paused) MaterialTheme.colorScheme.onTertiaryContainer else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
                )
            }
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        when (state.logs) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 5)
            is ScreenState.Content -> {
                if (filteredLogs.isEmpty()) {
                    Text(
                        text = "没有匹配的日志",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                } else {
                    LazyColumn(
                        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
                        modifier = Modifier.weight(1f)
                    ) {
                        item {
                            Surface(
                                shape = AmitiaPillShape,
                                color = MaterialTheme.colorScheme.surfaceVariant,
                                modifier = Modifier.clickable { }
                            ) {
                                Row(
                                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                                ) {
                                    Icon(
                                        imageVector = AmitiaIcons.Download,
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                        modifier = Modifier.size(16.dp)
                                    )
                                    Text(
                                        text = "导出脱敏日志",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                }
                            }
                        }
                        items(filteredLogs) { log -> LogCard(log) }
                    }
                }
            }
            else -> Unit
        }
    }
}

@Composable
private fun LogCard(log: DiagLogEntry) {
    val levelColor = when (log.level) {
        DiagLogLevel.Debug -> MaterialTheme.colorScheme.onSurfaceVariant
        DiagLogLevel.Info -> MaterialTheme.colorScheme.tertiary
        DiagLogLevel.Warning -> MaterialTheme.colorScheme.secondary
        DiagLogLevel.Error -> MaterialTheme.colorScheme.error
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Box(
                    modifier = Modifier
                        .size(6.dp)
                        .background(levelColor, CircleShape)
                )
                Text(
                    text = log.level.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = levelColor,
                    fontFamily = FontFamily.Monospace
                )
                Text(
                    text = log.source,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = log.timestamp,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    fontFamily = FontFamily.Monospace
                )
            }
            Text(
                text = log.message,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface,
                fontFamily = FontFamily.Monospace
            )
            if (!log.sanitized) {
                Text(
                    text = "未脱敏",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error
                )
            }
        }
    }
}

@Preview(name = "RawLogs - Light", showBackground = true)
@Composable
private fun RawLogsScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            RawLogsScreen()
        }
    }
}

@Preview(name = "RawLogs - Dark", showBackground = true)
@Composable
private fun RawLogsScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            RawLogsScreen()
        }
    }
}
