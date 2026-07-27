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
import androidx.compose.material.icons.outlined.Book
import androidx.compose.material.icons.outlined.Memory
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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.ConfirmStrategy
import com.amitia.feature.character.model.MemoryConfig
import com.amitia.feature.character.model.WorldBookRef

@Composable
fun CharacterMemoryTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.memoryState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadMemory() }
        )
        is ScreenState.Content -> MemoryContent(
            config = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun MemoryContent(
    config: MemoryConfig,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "memory_types") {
            MemoryTypesCard(config)
        }
        item(key = "world_books") {
            AmitiaSection(title = "世界书绑定", subtitle = "关联的世界书知识库") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    config.worldBooks.forEach { book ->
                        WorldBookRow(book)
                    }
                }
            }
        }
        item(key = "graph") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    AmitiaSwitchRow(
                        title = "记忆图谱",
                        subtitle = "构建记忆之间的关系网络",
                        checked = config.memoryGraphEnabled,
                        onCheckedChange = {},
                        leadingIcon = Icons.Outlined.Memory
                    )
                }
            }
        }
        item(key = "auto_summary") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    AmitiaSwitchRow(
                        title = "自动摘要",
                        subtitle = "定期自动总结对话记忆",
                        checked = config.autoSummary,
                        onCheckedChange = {}
                    )
                }
            }
        }
        item(key = "write_threshold") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "记忆写入阈值",
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "对话达到此轮数后触发记忆写入",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    AmitiaSlider(
                        value = config.writeThreshold.toFloat(),
                        onValueChange = {},
                        valueRange = 1f..10f,
                        steps = 8,
                        valueFormatter = { "${it.toInt()} 轮" }
                    )
                }
            }
        }
        item(key = "confirm_strategy") {
            ConfirmStrategyCard(config.confirmStrategy)
        }
        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "查看记忆",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "保存设置",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun MemoryTypesCard(config: MemoryConfig) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "记忆类型",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            AmitiaSwitchRow(
                title = "长期记忆",
                subtitle = "持久化存储重要信息",
                checked = config.longTermEnabled,
                onCheckedChange = {}
            )
            Spacer(modifier = Modifier.height(4.dp))
            AmitiaSwitchRow(
                title = "情景记忆",
                subtitle = "记录具体的对话场景",
                checked = config.episodicEnabled,
                onCheckedChange = {}
            )
        }
    }
}

@Composable
private fun WorldBookRow(book: WorldBookRef) {
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
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(
                        if (book.enabled) MaterialTheme.colorScheme.primaryContainer
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Book,
                    contentDescription = null,
                    tint = if (book.enabled) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = book.name,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "${book.entryCount} 个词条",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Surface(
                shape = RoundedCornerShape(8.dp),
                color = if (book.enabled) MaterialTheme.colorScheme.tertiaryContainer
                else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = if (book.enabled) "启用" else "禁用",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (book.enabled) MaterialTheme.colorScheme.onTertiaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Composable
private fun ConfirmStrategyCard(strategy: ConfirmStrategy) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "写入确认策略",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(8.dp))
            AmitiaSelectionRow(
                title = "每次确认",
                subtitle = "每次写入记忆前都需要用户确认",
                selected = strategy == ConfirmStrategy.Always,
                onSelect = {}
            )
            AmitiaSelectionRow(
                title = "仅重要记忆",
                subtitle = "仅在写入重要记忆时确认",
                selected = strategy == ConfirmStrategy.Important,
                onSelect = {}
            )
            AmitiaSelectionRow(
                title = "自动写入",
                subtitle = "无需用户确认，自动写入",
                selected = strategy == ConfirmStrategy.Never,
                onSelect = {}
            )
        }
    }
}

@Preview(name = "Memory - Light", showBackground = true)
@Composable
private fun CharacterMemoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            MemoryContent(
                config = MemoryConfig(
                    longTermEnabled = true,
                    episodicEnabled = true,
                    worldBooks = listOf(WorldBookRef("1", "角色世界观", 128, true)),
                    memoryGraphEnabled = true,
                    autoSummary = true,
                    writeThreshold = 3,
                    confirmStrategy = ConfirmStrategy.Important
                )
            )
        }
    }
}

@Preview(name = "Memory - Dark", showBackground = true)
@Composable
private fun CharacterMemoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            MemoryContent(
                config = MemoryConfig(
                    longTermEnabled = false,
                    episodicEnabled = false,
                    worldBooks = listOf(),
                    memoryGraphEnabled = false,
                    autoSummary = false,
                    writeThreshold = 1,
                    confirmStrategy = ConfirmStrategy.Never
                )
            )
        }
    }
}
