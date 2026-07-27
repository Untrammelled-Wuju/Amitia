package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaDangerDialog
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.ExtensionCard
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun SystemPluginListScreen(
    onBack: () -> Unit,
    onOpenPluginDetail: (String) -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val plugins by viewModel.plugins.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    val systemPlugins = plugins.filter { it.isSystem }
    SystemPluginListContent(
        plugins = systemPlugins,
        loading = loading,
        onBack = onBack,
        onOpenPluginDetail = onOpenPluginDetail,
        onToggle = { id, enabled -> viewModel.togglePlugin(id, enabled) }
    )
}

@Composable
fun SystemPluginListContent(
    plugins: List<PluginInfo>,
    loading: Boolean,
    onBack: () -> Unit,
    onOpenPluginDetail: (String) -> Unit,
    onToggle: (String, Boolean) -> Unit
) {
    var pendingDisable by remember { mutableStateOf<PluginInfo?>(null) }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "系统插件", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载系统插件...")
            }
            plugins.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Security,
                title = "暂无系统插件",
                description = "系统内置插件将在此展示",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "hint") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                    ) {
                        Text(
                            text = "系统插件由 Amitia 内置，不可卸载。关键插件禁用前会说明影响。",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
                item(key = "header") {
                    AmitiaSectionHeader(title = "内置插件", trailing = {
                        Text(
                            text = "${plugins.size} 个",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    })
                }
                items(plugins, key = { it.id }) { plugin ->
                    SystemPluginItem(
                        plugin = plugin,
                        onClick = { onOpenPluginDetail(plugin.id) },
                        onToggle = { enabled ->
                            if (!enabled && plugin.impactDescription != null) {
                                pendingDisable = plugin
                            } else {
                                onToggle(plugin.id, enabled)
                            }
                        }
                    )
                }
            }
        }
    }
    pendingDisable?.let { plugin ->
        AmitiaDangerDialog(
            onDismiss = { pendingDisable = null },
            onConfirm = {
                onToggle(plugin.id, false)
                pendingDisable = null
            },
            title = "禁用 ${plugin.name}",
            message = "禁用系统插件可能影响核心功能",
            impactDescription = plugin.impactDescription,
            confirmText = "确认禁用"
        )
    }
}

@Composable
private fun SystemPluginItem(
    plugin: PluginInfo,
    onClick: () -> Unit,
    onToggle: (Boolean) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            ExtensionCard(
                name = plugin.name,
                description = plugin.description,
                version = plugin.version,
                enabled = plugin.enabled,
                icon = AmitiaIcons.Security,
                onClick = onClick,
                onToggle = onToggle
            )
            androidx.compose.foundation.layout.Row(
                modifier = Modifier.fillMaxWidth().padding(top = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                BadgeChip(text = "不可卸载", color = MaterialTheme.colorScheme.tertiary)
                BadgeChip(text = "系统内置", color = MaterialTheme.colorScheme.primaryContainer)
                if (plugin.impactDescription != null) {
                    BadgeChip(text = "关键插件", color = MaterialTheme.colorScheme.errorContainer)
                }
            }
        }
    }
}

@Composable
private fun BadgeChip(text: String, color: androidx.compose.ui.graphics.Color) {
    Surface(shape = RoundedCornerShape(8.dp), color = color.copy(alpha = 0.6f)) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
        )
    }
}

@Preview(name = "System Plugins - Light", showBackground = true)
@Composable
private fun SystemPluginListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SystemPluginListContent(
            plugins = listOf(
                PluginInfo("sys-1", "对话记忆", "管理长期记忆", "1.0.0", "Amitia", "系统内置", true, true, false, com.amitia.core.designsystem.component.AmitiaStatusType.Running, impactDescription = "禁用后角色将无法记忆对话内容"),
                PluginInfo("sys-2", "渠道接入", "渠道连接", "1.0.0", "Amitia", "系统内置", true, true, false, com.amitia.core.designsystem.component.AmitiaStatusType.Running)
            ),
            loading = false, onBack = {}, onOpenPluginDetail = {}, onToggle = { _, _ -> }
        )
    }
}

@Preview(name = "System Plugins - Dark", showBackground = true)
@Composable
private fun SystemPluginListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SystemPluginListContent(
            plugins = emptyList(), loading = false, onBack = {}, onOpenPluginDetail = {}, onToggle = { _, _ -> }
        )
    }
}
