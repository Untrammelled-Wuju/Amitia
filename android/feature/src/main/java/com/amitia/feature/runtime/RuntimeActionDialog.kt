package com.amitia.feature.runtime

import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable

@Composable
fun RuntimeActionDialog(
    action: RuntimeAction,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit
) {
    val title = when (action) {
        RuntimeAction.CLEANUP -> "清理 RootFS"
        RuntimeAction.RESTORE -> "数据恢复"
        RuntimeAction.STOP -> "停止运行时"
        RuntimeAction.REPAIR -> "修复运行时"
        else -> action.name
    }
    val message = when (action) {
        RuntimeAction.CLEANUP -> "将清理 RootFS 目录与运行时缓存，不会删除 amitia-data（对话、记忆、角色）。"
        RuntimeAction.RESTORE -> "将从最近备份恢复数据，过程中服务可能重启。"
        RuntimeAction.STOP -> "将停止运行时所有服务，需要时可以重新启动。"
        RuntimeAction.REPAIR -> "将尝试修复运行时状态，已运行的服务会被重启。"
        else -> "确认执行 $title ？"
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(text = title, style = MaterialTheme.typography.titleMedium) },
        text = {
            Text(
                text = message,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text(
                    text = "确认",
                    color = if (action == RuntimeAction.CLEANUP || action == RuntimeAction.RESTORE || action == RuntimeAction.STOP) {
                        MaterialTheme.colorScheme.error
                    } else {
                        MaterialTheme.colorScheme.primary
                    }
                )
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(text = "取消")
            }
        }
    )
}
