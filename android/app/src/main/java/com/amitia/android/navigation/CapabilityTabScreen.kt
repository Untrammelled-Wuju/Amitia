package com.amitia.android.navigation

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudSync
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.Memory
import androidx.compose.material.icons.outlined.Tune
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.component.AmitiaEntryCard

@Composable
fun CapabilityTabScreen(
    onOpenModels: () -> Unit,
    onOpenChannels: () -> Unit,
    onOpenMemory: () -> Unit,
    onOpenRuntime: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "能力",
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground
        )
        CapabilityEntry(
            icon = Icons.Outlined.Tune,
            title = "模型配置",
            subtitle = "管理 LLM、Embedding、TTS、ASR、视觉、图像生成模型",
            onClick = onOpenModels
        )
        CapabilityEntry(
            icon = Icons.Outlined.Hub,
            title = "渠道状态",
            subtitle = "微信、QQ、Web 等接入渠道的连接与绑定状态",
            onClick = onOpenChannels
        )
        CapabilityEntry(
            icon = Icons.Outlined.Memory,
            title = "记忆管理",
            subtitle = "长期 / 情景 / 初始 / 世界书 / 时间线 / 图谱",
            onClick = onOpenMemory
        )
        CapabilityEntry(
            icon = Icons.Outlined.CloudSync,
            title = "Runtime 管理",
            subtitle = "本地核心 / 远程模式 / 服务状态 / 诊断导出",
            onClick = onOpenRuntime
        )
    }
}

@Composable
private fun CapabilityEntry(
    icon: ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    AmitiaEntryCard(
        onClick = onClick,
        leading = {
            Icon(
                imageVector = icon,
                contentDescription = title,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(22.dp)
            )
        },
        title = title,
        subtitle = subtitle
    )
}
