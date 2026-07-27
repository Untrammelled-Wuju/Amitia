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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.WarningBanner

private enum class McpConnectionType(val label: String) {
    Stdio("Stdio"), Sse("SSE"), WebSocket("WebSocket")
}

@Composable
fun McpCreateScreen(
    onBack: () -> Unit,
    onCreated: () -> Unit
) {
    var name by remember { mutableStateOf("") }
    var selectedType by remember { mutableStateOf(0) }
    var endpoint by remember { mutableStateOf("") }
    var command by remember { mutableStateOf("") }
    var tested by remember { mutableStateOf(false) }
    McpCreateContent(
        name = name,
        endpoint = endpoint,
        command = command,
        selectedType = selectedType,
        tested = tested,
        onBack = onBack,
        onNameChange = { name = it },
        onEndpointChange = { endpoint = it },
        onCommandChange = { command = it },
        onTypeSelected = { selectedType = it; tested = false },
        onTest = { tested = true },
        onCreate = onCreated
    )
}

@Composable
fun McpCreateContent(
    name: String,
    endpoint: String,
    command: String,
    selectedType: Int,
    tested: Boolean,
    onBack: () -> Unit,
    onNameChange: (String) -> Unit,
    onEndpointChange: (String) -> Unit,
    onCommandChange: (String) -> Unit,
    onTypeSelected: (Int) -> Unit,
    onTest: () -> Unit,
    onCreate: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "新建 MCP", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "基本信息")
            AmitiaTextField(
                value = name,
                onValueChange = onNameChange,
                label = "服务名称",
                placeholder = "请输入 MCP 服务名称"
            )
            AmitiaSectionHeader(title = "连接方式")
            McpConnectionType.entries.forEachIndexed { index, type ->
                AmitiaSelectionRow(
                    title = type.label,
                    selected = selectedType == index,
                    onSelect = { onTypeSelected(index) }
                )
            }
            AmitiaSectionHeader(title = "连接配置")
            when (McpConnectionType.entries[selectedType]) {
                McpConnectionType.Stdio -> {
                    AmitiaTextField(
                        value = command,
                        onValueChange = onCommandChange,
                        label = "启动命令",
                        placeholder = "例如：node server.js"
                    )
                }
                McpConnectionType.Sse, McpConnectionType.WebSocket -> {
                    AmitiaTextField(
                        value = endpoint,
                        onValueChange = onEndpointChange,
                        label = "服务地址",
                        placeholder = "例如：http://localhost:8080/sse"
                    )
                }
            }
            if (tested) {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                ) {
                    Row(
                        modifier = Modifier.padding(AmitiaSpacing.Base),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.CheckCircle,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.tertiary
                        )
                        Text(
                            text = "连接测试成功",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
                AmitiaSectionHeader(title = "权限摘要")
                PermissionSummaryItem("网络访问", "连接 MCP 服务器")
                when (McpConnectionType.entries[selectedType]) {
                    McpConnectionType.Stdio -> PermissionSummaryItem("进程启动", "执行启动命令")
                    else -> PermissionSummaryItem("网络访问", "维持长连接")
                }
            } else {
                WarningBanner(
                    message = "请先进行连接测试，确认服务可用后再创建"
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                SecondaryButton(
                    text = "测试连接",
                    onClick = onTest,
                    leadingIcon = AmitiaIcons.PlayArrow,
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "创建",
                    onClick = onCreate,
                    leadingIcon = AmitiaIcons.Check,
                    enabled = tested && name.isNotBlank(),
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun PermissionSummaryItem(name: String, description: String) {
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
                imageVector = AmitiaIcons.Security,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary
            )
            Column {
                Text(
                    text = name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1, overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Preview(name = "MCP Create - Light", showBackground = true)
@Composable
private fun McpCreateLightPreview() {
    AmitiaTheme(darkTheme = false) {
        McpCreateContent(
            name = "文件搜索服务",
            endpoint = "http://localhost:8080/sse",
            command = "",
            selectedType = 1,
            tested = true,
            onBack = {},
            onNameChange = {},
            onEndpointChange = {},
            onCommandChange = {},
            onTypeSelected = {},
            onTest = {},
            onCreate = {}
        )
    }
}

@Preview(name = "MCP Create - Dark", showBackground = true)
@Composable
private fun McpCreateDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        McpCreateContent(
            name = "", endpoint = "", command = "", selectedType = 0, tested = false,
            onBack = {}, onNameChange = {}, onEndpointChange = {}, onCommandChange = {},
            onTypeSelected = {}, onTest = {}, onCreate = {}
        )
    }
}
