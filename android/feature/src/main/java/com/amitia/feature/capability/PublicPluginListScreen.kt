package com.amitia.feature.capability

import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.ExtensionCard
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun PublicPluginListScreen(
    onBack: () -> Unit,
    onOpenPluginDetail: (String) -> Unit,
    onOpenUpdates: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val plugins by viewModel.plugins.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    val publicPlugins = plugins.filter { !it.isSystem }
    PublicPluginListContent(
        plugins = publicPlugins,
        loading = loading,
        onBack = onBack,
        onOpenPluginDetail = onOpenPluginDetail,
        onOpenUpdates = onOpenUpdates,
        onToggle = { id, enabled -> viewModel.togglePlugin(id, enabled) }
    )
}

@Composable
fun PublicPluginListContent(
    plugins: List<PluginInfo>,
    loading: Boolean,
    onBack: () -> Unit,
    onOpenPluginDetail: (String) -> Unit,
    onOpenUpdates: () -> Unit,
    onToggle: (String, Boolean) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "公共插件", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载公共插件...")
            }
            plugins.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Public,
                title = "暂无公共插件",
                description = "导入扩展包或从社区安装后将在此展示",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(plugins, key = { it.id }) { plugin ->
                    PublicPluginItem(
                        plugin = plugin,
                        onClick = { onOpenPluginDetail(plugin.id) },
                        onToggle = { enabled -> onToggle(plugin.id, enabled) }
                    )
                }
                if (plugins.any { it.updateAvailable }) {
                    item(key = "update_entry") {
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.medium,
                            color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                        ) {
                            Row(
                                modifier = Modifier.padding(AmitiaSpacing.Base).clickable { onOpenUpdates() },
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                            ) {
                                Icon(
                                    imageVector = AmitiaIcons.Update,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.tertiary,
                                    modifier = Modifier.size(AmitiaIconSize.Medium)
                                )
                                Text(
                                    text = "有可用更新，前往管理",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun PublicPluginItem(
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
                icon = AmitiaIcons.Extension,
                onClick = onClick,
                onToggle = onToggle
            )
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                PluginMetaChip(label = "来源", value = plugin.source)
                PluginMetaChip(label = "作者", value = plugin.author)
                PluginMetaChip(
                    label = "权限",
                    value = "${plugin.permissions.size} 项"
                )
                if (plugin.updateAvailable) {
                    Surface(shape = RoundedCornerShape(8.dp), color = MaterialTheme.colorScheme.tertiaryContainer) {
                        Text(
                            text = "有更新",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer,
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun PluginMetaChip(label: String, value: String) {
    Surface(shape = RoundedCornerShape(8.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
        Text(
            text = "$label: $value",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
        )
    }
}

@Preview(name = "Public Plugins - Light", showBackground = true)
@Composable
private fun PublicPluginListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PublicPluginListContent(
            plugins = listOf(
                PluginInfo("pub-1", "天气查询", "实时天气", "1.2.0", "社区", "公共插件", true, false, true, com.amitia.core.designsystem.component.AmitiaStatusType.Running, updateAvailable = true, permissions = listOf(PluginPermission("网络访问", "查询天气", true, PermissionRiskLevel.Low, PermissionCategory.Network)))
            ),
            loading = false, onBack = {}, onOpenPluginDetail = {}, onOpenUpdates = {}, onToggle = { _, _ -> }
        )
    }
}

@Preview(name = "Public Plugins - Dark", showBackground = true)
@Composable
private fun PublicPluginListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PublicPluginListContent(
            plugins = emptyList(), loading = true, onBack = {}, onOpenPluginDetail = {}, onOpenUpdates = {}, onToggle = { _, _ -> }
        )
    }
}
