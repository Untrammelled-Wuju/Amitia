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
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.StatusRow

@Composable
fun McpListScreen(
    onBack: () -> Unit,
    onCreate: () -> Unit,
    onOpenDetail: (String) -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val servers by viewModel.mcpServers.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    McpListContent(
        servers = servers,
        loading = loading,
        onBack = onBack,
        onCreate = onCreate,
        onOpenDetail = onOpenDetail
    )
}

@Composable
fun McpListContent(
    servers: List<McpServerInfo>,
    loading: Boolean,
    onBack: () -> Unit,
    onCreate: () -> Unit,
    onOpenDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "MCP 服务器",
            onBack = onBack,
            actions = {
                AmitiaIconButton(
                    icon = AmitiaIcons.Add,
                    contentDescription = "新建",
                    onClick = onCreate
                )
            }
        )
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载 MCP...")
            }
            servers.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Hub,
                title = "暂无 MCP 服务器",
                description = "点击右上角新建 MCP 服务器",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(servers, key = { it.id }) { server ->
                    StatusRow(
                        title = server.name,
                        status = server.status,
                        subtitle = "${server.connectionType} · ${server.toolCount} 个工具 · 来源 ${server.sourceSkill}",
                        leadingIcon = AmitiaIcons.Hub,
                        onClick = { onOpenDetail(server.id) }
                    )
                }
            }
        }
    }
}

@Preview(name = "MCP List - Light", showBackground = true)
@Composable
private fun McpListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        McpListContent(
            servers = listOf(
                McpServerInfo("m1", "文件搜索服务", "stdio", com.amitia.core.designsystem.component.AmitiaStatusType.Connected, 4, "文件检索", roles = listOf("艾米")),
                McpServerInfo("m2", "数据库查询", "sse", com.amitia.core.designsystem.component.AmitiaStatusType.Disconnected, 2, "数据助手")
            ),
            loading = false, onBack = {}, onCreate = {}, onOpenDetail = {}
        )
    }
}

@Preview(name = "MCP List - Dark", showBackground = true)
@Composable
private fun McpListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        McpListContent(
            servers = emptyList(), loading = true, onBack = {}, onCreate = {}, onOpenDetail = {}
        )
    }
}
