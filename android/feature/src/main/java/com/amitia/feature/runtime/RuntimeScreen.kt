package com.amitia.feature.runtime

import android.widget.Toast
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Backup
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.CleaningServices
import androidx.compose.material.icons.outlined.CloudDownload
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.RestartAlt
import androidx.compose.material.icons.outlined.Stop
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun RuntimeScreen(
    onBack: () -> Unit,
    viewModel: RuntimeViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    var pendingAction by remember { mutableStateOf<RuntimeAction?>(null) }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Runtime 管理",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    IconButton(onClick = viewModel::refresh) {
                        Icon(Icons.Outlined.Refresh, contentDescription = "刷新")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            HeaderCard(state = state)
            ServiceCard(
                title = "Go 后端",
                serviceState = state.services.backend,
                port = state.ports.backend,
                uptimeMs = state.uptimeMs
            )
            ServiceCard(
                title = "Qdrant",
                serviceState = state.services.qdrant,
                port = state.ports.qdrant,
                uptimeMs = state.uptimeMs
            )
            ServiceCard(
                title = "SurrealDB",
                serviceState = state.services.surrealDb,
                port = state.ports.surrealdb,
                uptimeMs = state.uptimeMs
            )
            DataUsageCard(state = state)
            LogsCard(state = state)
            if (state.lastError != null) {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                    color = MaterialTheme.colorScheme.errorContainer
                ) {
                    Text(
                        text = state.lastError.orEmpty(),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onErrorContainer,
                        modifier = Modifier.padding(12.dp)
                    )
                }
            }
            ActionGrid(
                state = state,
                onAction = { action ->
                    when (action) {
                        RuntimeAction.START -> viewModel.start()
                        RuntimeAction.STOP -> pendingAction = action
                        RuntimeAction.RESTART -> viewModel.restart()
                        RuntimeAction.REPAIR -> pendingAction = action
                        RuntimeAction.UPDATE -> viewModel.update()
                        RuntimeAction.EXPORT_DIAGNOSTICS -> {
                            viewModel.exportDiagnostics { file ->
                                Toast.makeText(
                                    context,
                                    "诊断已导出：${file.absolutePath}",
                                    Toast.LENGTH_LONG
                                ).show()
                            }
                        }
                        RuntimeAction.CLEANUP -> pendingAction = action
                        RuntimeAction.BACKUP -> viewModel.backup()
                        RuntimeAction.RESTORE -> pendingAction = action
                    }
                }
            )
            Spacer(modifier = Modifier.height(24.dp))
        }
    }
    pendingAction?.let { action ->
        RuntimeActionDialog(
            action = action,
            onDismiss = { pendingAction = null },
            onConfirm = {
                when (action) {
                    RuntimeAction.STOP -> viewModel.stop()
                    RuntimeAction.REPAIR -> viewModel.repair()
                    RuntimeAction.CLEANUP -> viewModel.cleanup(confirmed = true)
                    RuntimeAction.RESTORE -> viewModel.restore()
                    else -> Unit
                }
                pendingAction = null
            }
        )
    }
}

@Composable
private fun HeaderCard(state: RuntimeUiState) {
    val dotColor = when (state.runtimeState) {
        is RuntimeState.Running -> AmitiaColors.StateRunning
        is RuntimeState.Degraded -> AmitiaColors.StateDegraded
        is RuntimeState.Failed -> AmitiaColors.StateFailed
        is RuntimeState.Installing, is RuntimeState.Starting, is RuntimeState.Updating,
        is RuntimeState.Stopping -> AmitiaColors.StateInstalling
        else -> AmitiaColors.StateIdle
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            AmitiaStatusDot(color = dotColor, modifier = Modifier.size(12.dp))
            Spacer(modifier = Modifier.padding(8.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = state.runtimeState.readableMessage,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "RootFS 版本: ${state.rootfsVersion ?: "未安装"}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (state.uptimeMs > 0) {
                    Text(
                        text = "运行时长: ${formatUptime(state.uptimeMs)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = AmitiaColors.OnSurfaceMuted
                    )
                }
            }
        }
    }
}

@Composable
private fun ServiceCard(
    title: String,
    serviceState: ServiceState,
    port: Int?,
    uptimeMs: Long
) {
    val statusText = when (serviceState) {
        is ServiceState.Healthy -> "Healthy@${serviceState.port}"
        is ServiceState.Unhealthy -> "Unhealthy: ${serviceState.reason}"
        ServiceState.Stopped -> "Stopped"
        ServiceState.Starting -> "Starting"
    }
    val dotColor = when (serviceState) {
        is ServiceState.Healthy -> AmitiaColors.StateRunning
        is ServiceState.Unhealthy -> AmitiaColors.StateFailed
        ServiceState.Stopped -> AmitiaColors.StateIdle
        ServiceState.Starting -> AmitiaColors.StateInstalling
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            AmitiaStatusDot(color = dotColor)
            Spacer(modifier = Modifier.padding(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = statusText,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Text(
                text = "端口 ${port ?: "-"}",
                style = MaterialTheme.typography.labelMedium,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
    }
}

@Composable
private fun DataUsageCard(state: RuntimeUiState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "数据占用",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "amitia-data: ${formatBytes(state.dataUsage.dataBytes)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "rootfs: ${formatBytes(state.dataUsage.rootfsBytes)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Composable
private fun LogsCard(state: RuntimeUiState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Text(
                text = "日志预览（最近 ${state.logs.size} 条）",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(8.dp))
            if (state.logs.isEmpty()) {
                Text(
                    text = "暂无日志",
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            } else {
                val timeFormat = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
                state.logs.takeLast(20).forEach { entry ->
                    val time = timeFormat.format(Date(entry.timestampMs))
                    Text(
                        text = "[$time] ${entry.level} ${entry.tag}: ${entry.message}",
                        style = MaterialTheme.typography.bodySmall,
                        color = when (entry.level) {
                            "ERROR" -> MaterialTheme.colorScheme.error
                            "WARN" -> AmitiaColors.StateDegraded
                            else -> MaterialTheme.colorScheme.onSurface
                        },
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
    }
}

@Composable
private fun ActionGrid(
    state: RuntimeUiState,
    onAction: (RuntimeAction) -> Unit
) {
    val isRunning = state.runtimeState is RuntimeState.Running
    val isStopped = state.runtimeState is RuntimeState.Stopped || state.runtimeState is RuntimeState.NotInstalled
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        ActionButton(
            icon = Icons.Outlined.PlayArrow,
            label = "启动",
            enabled = isStopped,
            onClick = { onAction(RuntimeAction.START) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.Stop,
            label = "停止",
            enabled = isRunning,
            onClick = { onAction(RuntimeAction.STOP) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.RestartAlt,
            label = "重启",
            enabled = isRunning,
            onClick = { onAction(RuntimeAction.RESTART) },
            modifier = Modifier.weight(1f)
        )
    }
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        ActionButton(
            icon = Icons.Outlined.Build,
            label = "修复",
            enabled = true,
            onClick = { onAction(RuntimeAction.REPAIR) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.CloudDownload,
            label = "更新",
            enabled = true,
            onClick = { onAction(RuntimeAction.UPDATE) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.BugReport,
            label = "诊断",
            enabled = true,
            onClick = { onAction(RuntimeAction.EXPORT_DIAGNOSTICS) },
            modifier = Modifier.weight(1f)
        )
    }
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        ActionButton(
            icon = Icons.Outlined.CleaningServices,
            label = "清理",
            enabled = true,
            onClick = { onAction(RuntimeAction.CLEANUP) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.Backup,
            label = "备份",
            enabled = true,
            onClick = { onAction(RuntimeAction.BACKUP) },
            modifier = Modifier.weight(1f)
        )
        ActionButton(
            icon = Icons.Outlined.RestartAlt,
            label = "恢复",
            enabled = true,
            onClick = { onAction(RuntimeAction.RESTORE) },
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun ActionButton(
    icon: ImageVector,
    label: String,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    OutlinedButton(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled
    ) {
        Icon(icon, contentDescription = null)
        Spacer(modifier = Modifier.size(6.dp))
        Text(text = label)
    }
}

private fun formatUptime(ms: Long): String {
    val seconds = ms / 1000
    val hours = seconds / 3600
    val minutes = (seconds % 3600) / 60
    val secs = seconds % 60
    return "%02d:%02d:%02d".format(hours, minutes, secs)
}

private fun formatBytes(bytes: Long): String {
    if (bytes <= 0) return "0 B"
    val units = arrayOf("B", "KB", "MB", "GB", "TB")
    var size = bytes.toDouble()
    var unitIndex = 0
    while (size >= 1024 && unitIndex < units.lastIndex) {
        size /= 1024
        unitIndex++
    }
    return "%.2f %s".format(size, units[unitIndex])
}
