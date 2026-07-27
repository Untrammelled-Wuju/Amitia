package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.ExperimentalLayoutApi
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
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.StatusRow

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun PluginDetailScreen(
    pluginId: String,
    onBack: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val plugins by viewModel.plugins.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    PluginDetailContent(
        plugin = plugins.firstOrNull { it.id == pluginId },
        loading = loading,
        onBack = onBack,
        onUpdate = {},
        onUninstall = {}
    )
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun PluginDetailContent(
    plugin: PluginInfo?,
    loading: Boolean,
    onBack: () -> Unit,
    onUpdate: () -> Unit,
    onUninstall: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = plugin?.name ?: "插件详情", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载详情...")
            }
            plugin == null -> AmitiaErrorState(
                icon = AmitiaIcons.Error,
                title = "未找到插件",
                description = "该插件可能已被移除",
                onRetry = onBack,
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "basic") {
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
                                modifier = Modifier.size(48.dp).clip(CircleShape)
                                    .background(MaterialTheme.colorScheme.primaryContainer),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = AmitiaIcons.Extension,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                                    modifier = Modifier.size(AmitiaIconSize.Nav)
                                )
                            }
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    text = plugin.name,
                                    style = MaterialTheme.typography.titleMedium,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis
                                )
                                Text(
                                    text = "v${plugin.version} · ${plugin.author}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                                Text(
                                    text = "来源：${plugin.source}",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
                item(key = "status") {
                    StatusRow(
                        title = "运行状态",
                        status = plugin.status,
                        leadingIcon = AmitiaIcons.PlayArrow
                    )
                }
                item(key = "roles_header") { AmitiaSectionHeader(title = "使用角色") }
                item(key = "roles") {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        if (plugin.roles.isEmpty()) {
                            Text(
                                text = "未绑定角色",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else {
                            plugin.roles.forEach { role -> RoleChip(role) }
                        }
                    }
                }
                item(key = "contrib_header") { AmitiaSectionHeader(title = "提供的能力") }
                items(buildList {
                    add("工具" to plugin.tools)
                    add("事件" to plugin.events)
                    add("Hook" to plugin.hooks)
                    add("任务" to plugin.tasks)
                    add("UI Contribution" to plugin.uiContributions)
                }) { (label, list) ->
                    ContributionRow(label = label, items = list)
                }
                if (plugin.permissions.isNotEmpty()) {
                    item(key = "perm_header") { AmitiaSectionHeader(title = "权限") }
                    items(plugin.permissions, key = { it.name }) { perm ->
                        SettingsRow(
                            title = perm.name,
                            subtitle = "${perm.description} · ${perm.riskLevel.label}",
                            leadingIcon = AmitiaIcons.Lock
                        )
                    }
                }
                if (plugin.isSystem && plugin.impactDescription != null) {
                    item(key = "impact") {
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.medium,
                            color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.3f)
                        ) {
                            Row(
                                modifier = Modifier.padding(AmitiaSpacing.Base),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                            ) {
                                Icon(
                                    imageVector = AmitiaIcons.Warning,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.error,
                                    modifier = Modifier.size(AmitiaIconSize.Medium)
                                )
                                Text(
                                    text = "禁用影响：${plugin.impactDescription}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onErrorContainer
                                )
                            }
                        }
                    }
                }
                item(key = "actions") {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        if (plugin.updateAvailable) {
                            PrimaryButton(
                                text = "更新",
                                onClick = onUpdate,
                                leadingIcon = AmitiaIcons.Update,
                                modifier = Modifier.weight(1f)
                            )
                        }
                        if (plugin.canUninstall) {
                            DangerButton(
                                text = "卸载",
                                onClick = onUninstall,
                                leadingIcon = AmitiaIcons.Delete,
                                modifier = Modifier.weight(1f)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun RoleChip(role: String) {
    Surface(
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Text(
            text = role,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
        )
    }
}

@Composable
private fun ContributionRow(label: String, items: List<String>) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontWeight = FontWeight.Medium
        )
        if (items.isEmpty()) {
            Text(
                text = "无",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
        } else {
            Text(
                text = items.joinToString("、"),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Preview(name = "Plugin Detail - Light", showBackground = true)
@Composable
private fun PluginDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PluginDetailContent(
            plugin = PluginInfo(
                "pub-1", "天气查询", "实时天气信息查询", "1.2.0", "社区", "公共插件",
                true, false, true, com.amitia.core.designsystem.component.AmitiaStatusType.Running,
                tools = listOf("query_weather"), events = listOf("on_weather_updated"),
                roles = listOf("艾米"), updateAvailable = true,
                permissions = listOf(PluginPermission("网络访问", "查询天气数据", true, PermissionRiskLevel.Low, PermissionCategory.Network))
            ),
            loading = false, onBack = {}, onUpdate = {}, onUninstall = {}
        )
    }
}

@Preview(name = "Plugin Detail - Dark", showBackground = true)
@Composable
private fun PluginDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PluginDetailContent(
            plugin = null, loading = true, onBack = {}, onUpdate = {}, onUninstall = {}
        )
    }
}
