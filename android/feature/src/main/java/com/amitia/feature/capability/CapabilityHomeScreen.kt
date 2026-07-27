package com.amitia.feature.capability

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
import androidx.compose.foundation.background
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.amitiaStatusColor

@Composable
fun CapabilityHomeScreen(
    onOpenSkills: () -> Unit,
    onOpenPlugins: () -> Unit,
    onOpenMcp: () -> Unit,
    onOpenComputerUse: () -> Unit,
    onOpenExtensionCenter: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val overview by viewModel.overview.collectAsStateWithLifecycle()
    val enabled by viewModel.enabledCapabilities.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    CapabilityHomeContent(
        overview = overview,
        enabled = enabled,
        loading = loading,
        onOpenSkills = onOpenSkills,
        onOpenPlugins = onOpenPlugins,
        onOpenMcp = onOpenMcp,
        onOpenComputerUse = onOpenComputerUse,
        onOpenExtensionCenter = onOpenExtensionCenter
    )
}

@Composable
fun CapabilityHomeContent(
    overview: CapabilityOverview?,
    enabled: List<EnabledCapability>,
    loading: Boolean,
    onOpenSkills: () -> Unit,
    onOpenPlugins: () -> Unit,
    onOpenMcp: () -> Unit,
    onOpenComputerUse: () -> Unit,
    onOpenExtensionCenter: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "能力")
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "overview") {
                OverviewStatsCard(overview = overview)
            }
            item(key = "entries") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    CapabilityEntryRow(AmitiaIcons.Lightbulb, "Skills", "技能与可复用能力", onOpenSkills)
                    CapabilityEntryRow(AmitiaIcons.Extension, "Plugins", "系统与公共插件", onOpenPlugins)
                    CapabilityEntryRow(AmitiaIcons.Hub, "MCP", "MCP 服务器与工具", onOpenMcp)
                    CapabilityEntryRow(AmitiaIcons.Devices, "Computer Use", "设备控制与审批", onOpenComputerUse)
                    CapabilityEntryRow(AmitiaIcons.Category, "扩展中心", "插件、扩展包管理", onOpenExtensionCenter)
                }
            }
            item(key = "enabled_header") {
                AmitiaSectionHeader(title = "已启用能力", trailing = {
                    Text(
                        text = "${enabled.size} 项",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                })
            }
            when {
                loading -> item(key = "loading") {
                    Box(
                        modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl),
                        contentAlignment = Alignment.Center
                    ) { InlineLoading(message = "加载能力...") }
                }
                enabled.isEmpty() -> item(key = "empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.ExtensionOutlined,
                        title = "暂无启用能力",
                        description = "启用插件或 Skill 后将在此展示",
                        modifier = Modifier.fillMaxWidth()
                    )
                }
                else -> items(enabled, key = { it.id }) { item ->
                    EnabledCapabilityRow(capability = item)
                }
            }
        }
    }
}

@Composable
private fun OverviewStatsCard(overview: CapabilityOverview?) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalArrangement = Arrangement.SpaceEvenly
        ) {
            StatItem("Skills", overview?.skillCount ?: 0)
            StatItem("Plugins", overview?.pluginCount ?: 0)
            StatItem("MCP", overview?.mcpCount ?: 0)
            StatItem("已启用", overview?.enabledCount ?: 0)
        }
    }
}

@Composable
private fun StatItem(label: String, count: Int) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = count.toString(),
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.primary,
            fontWeight = FontWeight.Bold
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun CapabilityEntryRow(
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
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        },
        title = title,
        subtitle = subtitle
    )
}

@Composable
private fun EnabledCapabilityRow(capability: EnabledCapability) {
    val statusColor = amitiaStatusColor(
        when (capability.status) {
            com.amitia.core.designsystem.component.AmitiaStatusType.Running,
            com.amitia.core.designsystem.component.AmitiaStatusType.Connected -> com.amitia.core.designsystem.component.AmitiaStatusType.Running
            else -> capability.status
        }
    )
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
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                val icon = when (capability.type) {
                    CapabilityType.Skill -> AmitiaIcons.Lightbulb
                    CapabilityType.Plugin -> AmitiaIcons.Extension
                    CapabilityType.Mcp -> AmitiaIcons.Hub
                    CapabilityType.System -> AmitiaIcons.Settings
                }
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = capability.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = capability.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                AmitiaStatusDot(color = statusColor)
                Text(
                    text = capability.type.name,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(start = AmitiaSpacing.Xs)
                )
            }
        }
    }
}

@Preview(name = "Capability Home - Light", showBackground = true)
@Composable
private fun CapabilityHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CapabilityHomeContent(
            overview = CapabilityOverview(8, 12, 3, 18, 5),
            enabled = listOf(
                EnabledCapability("1", "对话记忆", "长期记忆与上下文管理", CapabilityType.System, com.amitia.core.designsystem.component.AmitiaStatusType.Running),
                EnabledCapability("2", "天气查询", "实时天气信息查询", CapabilityType.Plugin, com.amitia.core.designsystem.component.AmitiaStatusType.Running)
            ),
            loading = false,
            onOpenSkills = {}, onOpenPlugins = {}, onOpenMcp = {}, onOpenComputerUse = {}, onOpenExtensionCenter = {}
        )
    }
}

@Preview(name = "Capability Home - Dark", showBackground = true)
@Composable
private fun CapabilityHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CapabilityHomeContent(
            overview = null,
            enabled = emptyList(),
            loading = true,
            onOpenSkills = {}, onOpenPlugins = {}, onOpenMcp = {}, onOpenComputerUse = {}, onOpenExtensionCenter = {}
        )
    }
}
