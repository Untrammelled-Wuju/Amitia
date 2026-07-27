package com.amitia.feature.settings.more

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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.LoadingSkeleton

data class MoreEntry(
    val title: String,
    val subtitle: String,
    val icon: ImageVector,
    val route: String
)

data class MoreSection(
    val title: String,
    val entries: List<MoreEntry>
)

@Composable
fun MoreScreen(
    onBack: () -> Unit,
    onNavigate: (String) -> Unit
) {
    val sections = rememberMoreSections()
    val accountName = "访客用户"
    val runMode = "本地模式"
    val runtimeSummary = "5 个服务运行中"

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        AmitiaContentSurface(
            modifier = Modifier.fillMaxWidth()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(AmitiaSpacing.Base),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Box(
                    modifier = Modifier.size(48.dp),
                    contentAlignment = Alignment.Center
                ) {
                    androidx.compose.material3.Icon(
                        imageVector = AmitiaIcons.Person,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(32.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = accountName,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = runMode,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        AmitiaStatusDot(color = AmitiaStateColors.Running)
                        Text(
                            text = runtimeSummary,
                            style = MaterialTheme.typography.bodySmall,
                            color = AmitiaStateColors.Running
                        )
                    }
                }
            }
        }
        sections.forEach { section ->
            AmitiaSection(title = section.title) {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        section.entries.forEachIndexed { index, entry ->
                            SettingsRow(
                                title = entry.title,
                                subtitle = entry.subtitle,
                                leadingIcon = entry.icon,
                                onClick = { onNavigate(entry.route) }
                            )
                            if (index < section.entries.lastIndex) {
                                com.amitia.core.designsystem.component.AmitiaInsetDivider(
                                    leadingInset = AmitiaSpacing.Base + 32.dp + AmitiaSpacing.Base
                                )
                            }
                        }
                    }
                }
            }
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
    }
}

@Composable
private fun MoreScreenLoading() {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        repeat(5) { LoadingSkeleton(lineCount = 3) }
    }
}

@Composable
private fun MoreScreenEmpty() {
    AmitiaEmptyState(
        icon = AmitiaIcons.MoreHoriz,
        title = "暂无可用功能",
        description = "请先完成初始化配置"
    )
}

@Composable
fun MoreScreenScaffold(
    onBack: () -> Unit,
    onNavigate: (String) -> Unit
) {
    com.amitia.core.designsystem.component.AmitiaPageScaffold(
        topBar = {
            AmitiaTopBar(
                title = "更多",
                onBack = onBack
            )
        }
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            MoreScreen(onBack = onBack, onNavigate = onNavigate)
        }
    }
}

private fun rememberMoreSections(): List<MoreSection> = listOf(
    MoreSection(
        title = "日程与主动消息",
        entries = listOf(
            MoreEntry("日程管理", "查看和管理角色日程", AmitiaIcons.Event, "schedule"),
            MoreEntry("主动消息", "角色主动发送的消息配置", AmitiaIcons.Notifications, "proactive")
        )
    ),
    MoreSection(
        title = "渠道",
        entries = listOf(
            MoreEntry("渠道管理", "管理微信、QQ 等消息渠道", AmitiaIcons.Hub, "channels")
        )
    ),
    MoreSection(
        title = "模型与语音",
        entries = listOf(
            MoreEntry("模型管理", "文本、视觉、向量模型配置", AmitiaIcons.SmartToy, "models"),
            MoreEntry("语音中心", "TTS、ASR 语音服务", AmitiaIcons.GraphicEq, "voice")
        )
    ),
    MoreSection(
        title = "能力与扩展",
        entries = listOf(
            MoreEntry("能力扩展", "工具、插件、函数调用", AmitiaIcons.Extension, "extensions"),
            MoreEntry("Computer Use", "计算机使用能力", AmitiaIcons.Devices, "computer_use"),
            MoreEntry("创意工坊", "角色创建、模板、素材", AmitiaIcons.AutoAwesome, "workshop")
        )
    ),
    MoreSection(
        title = "数据与备份",
        entries = listOf(
            MoreEntry("数据与存储", "存储用量、缓存管理", AmitiaIcons.Storage, "data_storage"),
            MoreEntry("备份与恢复", "数据备份、恢复", AmitiaIcons.Backup, "backup"),
            MoreEntry("导入导出", "数据导入导出", AmitiaIcons.ImportExport, "import_export")
        )
    ),
    MoreSection(
        title = "运行与诊断",
        entries = listOf(
            MoreEntry("运行模式", "本地/远程模式切换", AmitiaIcons.CloudDone, "run_mode"),
            MoreEntry("本地运行时", "服务状态、资源监控", AmitiaIcons.Build, "local_runtime"),
            MoreEntry("高级控制台", "运行时高级控制", AmitiaIcons.Terminal, "console"),
            MoreEntry("诊断", "网络诊断、日志", AmitiaIcons.BugReport, "diagnostics")
        )
    ),
    MoreSection(
        title = "设置",
        entries = listOf(
            MoreEntry("设置中心", "所有设置选项", AmitiaIcons.Settings, "settings_center"),
            MoreEntry("高级控制台", "开发者选项", AmitiaIcons.Code, "developer")
        )
    )
)

@Preview(name = "更多页 - 亮色", showBackground = true)
@Composable
private fun MoreScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MoreScreenScaffold(
            onBack = {},
            onNavigate = {}
        )
    }
}

@Preview(name = "更多页 - 暗色", showBackground = true)
@Composable
private fun MoreScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MoreScreenScaffold(
            onBack = {},
            onNavigate = {}
        )
    }
}
