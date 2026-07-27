package com.amitia.feature.character.detail

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Code
import androidx.compose.material.icons.outlined.Computer
import androidx.compose.material.icons.outlined.Extension
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.CapabilityCategory
import com.amitia.feature.character.model.CapabilityItem

@Composable
fun CharacterCapabilityTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.capabilityState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadCapabilities() }
        )
        is ScreenState.Content -> CapabilityContent(
            capabilities = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun CapabilityContent(
    capabilities: List<CapabilityItem>,
    modifier: Modifier = Modifier
) {
    val toggleStates = remember {
        mutableStateMapOf<String, Boolean>().apply {
            capabilities.forEach { put(it.id, it.enabled) }
        }
    }

    val grouped = capabilities.groupBy { it.category }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "summary") {
            CapabilitySummaryCard(capabilities, toggleStates)
        }
        grouped.forEach { (category, items) ->
            item(key = "category_${category.name}") {
                AmitiaSection(title = categoryLabel(category), subtitle = "${items.size} 项") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        items.forEach { item ->
                            CapabilityRow(
                                item = item,
                                enabled = toggleStates[item.id] ?: item.enabled,
                                onToggle = { toggleStates[item.id] = it }
                            )
                        }
                    }
                }
            }
        }
        item(key = "actions") {
            PrimaryButton(
                text = "保存配置",
                onClick = {},
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}

@Composable
private fun CapabilitySummaryCard(
    capabilities: List<CapabilityItem>,
    toggleStates: Map<String, Boolean>
) {
    val enabledCount = toggleStates.count { it.value }
    val total = capabilities.size
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.2f)),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "$enabledCount/$total",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "能力配置",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "已启用 $enabledCount 项能力，共 $total 项可用",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                )
            }
        }
    }
}

@Composable
private fun CapabilityRow(
    item: CapabilityItem,
    enabled: Boolean,
    onToggle: (Boolean) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .clip(CircleShape)
                    .background(
                        if (enabled) MaterialTheme.colorScheme.primaryContainer
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = categoryIcon(item.category),
                    contentDescription = null,
                    tint = if (enabled) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = item.name,
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    if (item.version != null) {
                        Spacer(modifier = Modifier.size(4.dp))
                        Text(
                            text = "v${item.version}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                        )
                    }
                }
                Text(
                    text = item.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1
                )
                Text(
                    text = item.scope,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                )
            }
            Switch(
                checked = enabled,
                onCheckedChange = onToggle,
                colors = SwitchDefaults.colors(
                    checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                    checkedTrackColor = MaterialTheme.colorScheme.primary,
                    uncheckedThumbColor = MaterialTheme.colorScheme.surface,
                    uncheckedTrackColor = MaterialTheme.colorScheme.surfaceVariant
                )
            )
        }
    }
}

private fun categoryLabel(category: CapabilityCategory): String = when (category) {
    CapabilityCategory.Skills -> "技能"
    CapabilityCategory.Plugins -> "插件"
    CapabilityCategory.Mcp -> "MCP 服务"
    CapabilityCategory.ComputerUse -> "桌面操作"
    CapabilityCategory.SystemTools -> "系统工具"
}

private fun categoryIcon(category: CapabilityCategory): ImageVector = when (category) {
    CapabilityCategory.Skills -> Icons.Outlined.Build
    CapabilityCategory.Plugins -> Icons.Outlined.Extension
    CapabilityCategory.Mcp -> Icons.Outlined.Code
    CapabilityCategory.ComputerUse -> Icons.Outlined.Computer
    CapabilityCategory.SystemTools -> Icons.Outlined.Settings
}

@Preview(name = "Capability - Light", showBackground = true)
@Composable
private fun CharacterCapabilityLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            CapabilityContent(
                capabilities = listOf(
                    CapabilityItem("1", "天气查询", "天气", CapabilityCategory.Skills, true, "角色专属", "1.0"),
                    CapabilityItem("2", "代码执行", "沙箱", CapabilityCategory.Plugins, false, "继承全局", null)
                )
            )
        }
    }
}

@Preview(name = "Capability - Dark", showBackground = true)
@Composable
private fun CharacterCapabilityDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            CapabilityContent(capabilities = listOf())
        }
    }
}
