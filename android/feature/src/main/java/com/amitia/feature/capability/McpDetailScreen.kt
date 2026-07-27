package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
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
import com.amitia.core.designsystem.component.SettingsRow

@Composable
fun McpDetailScreen(
    serverId: String,
    onBack: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val servers by viewModel.mcpServers.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    McpDetailContent(
        server = servers.firstOrNull { it.id == serverId },
        loading = loading,
        onBack = onBack,
        onToggle = { enabled -> viewModel.toggleMcp(serverId, enabled) }
    )
}

@Composable
fun McpDetailContent(
    server: McpServerInfo?,
    loading: Boolean,
    onBack: () -> Unit,
    onToggle: (Boolean) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = server?.name ?: "MCP 详情", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载详情...")
            }
            server == null -> AmitiaEmptyState(
                icon = AmitiaIcons.Hub,
                title = "未找到 MCP 服务",
                description = "该服务可能已被移除",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "config") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            Text(
                                text = server.name,
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                            Text(
                                text = "连接方式：${server.connectionType}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "来源 Skill：${server.sourceSkill}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "工具数量：${server.toolCount}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
                item(key = "toggle") {
                    AmitiaSwitchRow(
                        title = "启用服务",
                        subtitle = "启用后将可被角色调用",
                        checked = server.status == com.amitia.core.designsystem.component.AmitiaStatusType.Connected,
                        onCheckedChange = onToggle,
                        leadingIcon = AmitiaIcons.ToggleOn
                    )
                }
                if (server.tools.isNotEmpty()) {
                    item(key = "tools_header") { AmitiaSectionHeader(title = "工具列表") }
                    items(server.tools, key = { it.name }) { tool ->
                        SettingsRow(
                            title = tool.name,
                            subtitle = tool.description,
                            leadingIcon = AmitiaIcons.Build
                        )
                    }
                }
                if (server.resources.isNotEmpty()) {
                    item(key = "res_header") { AmitiaSectionHeader(title = "资源") }
                    items(server.resources, key = { it }) { res ->
                        SettingsRow(title = res, leadingIcon = AmitiaIcons.Storage)
                    }
                }
                if (server.promptTemplates.isNotEmpty()) {
                    item(key = "prompt_header") { AmitiaSectionHeader(title = "提示模板") }
                    items(server.promptTemplates, key = { it }) { template ->
                        SettingsRow(title = template, leadingIcon = AmitiaIcons.MenuBook)
                    }
                }
                if (server.recentCalls.isNotEmpty()) {
                    item(key = "calls_header") { AmitiaSectionHeader(title = "最近调用") }
                    items(server.recentCalls, key = { it.toolName + it.timestamp }) { call ->
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.medium,
                            color = MaterialTheme.colorScheme.surface
                        ) {
                            Row(
                                modifier = Modifier.padding(AmitiaSpacing.Base),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                            ) {
                                Icon(
                                    imageVector = if (call.success) AmitiaIcons.CheckCircle else AmitiaIcons.Error,
                                    contentDescription = null,
                                    tint = if (call.success) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.error
                                )
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        text = call.toolName,
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurface,
                                        fontWeight = FontWeight.Medium
                                    )
                                    Text(
                                        text = "${call.timestamp} · ${call.duration}",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        maxLines = 1, overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                }
                if (server.errors.isNotEmpty()) {
                    item(key = "errors_header") { AmitiaSectionHeader(title = "错误") }
                    items(server.errors, key = { it }) { err ->
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
                                    tint = MaterialTheme.colorScheme.error
                                )
                                Text(
                                    text = err,
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onErrorContainer
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Preview(name = "MCP Detail - Light", showBackground = true)
@Composable
private fun McpDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        McpDetailContent(
            server = McpServerInfo(
                "m1", "文件搜索服务", "stdio",
                com.amitia.core.designsystem.component.AmitiaStatusType.Connected, 4, "文件检索",
                roles = listOf("艾米"),
                tools = listOf(McpTool("search", "搜索文件"), McpTool("read", "读取文件")),
                recentCalls = listOf(McpCallRecord("search", "14:30", true, "120ms")),
                errors = listOf("连接超时重试成功")
            ),
            loading = false, onBack = {}, onToggle = {}
        )
    }
}

@Preview(name = "MCP Detail - Dark", showBackground = true)
@Composable
private fun McpDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        McpDetailContent(
            server = null, loading = true, onBack = {}, onToggle = {}
        )
    }
}
