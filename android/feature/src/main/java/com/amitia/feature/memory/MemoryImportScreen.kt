package com.amitia.feature.memory

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
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
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun MemoryImportScreen(
    onBack: () -> Unit,
    onProceedToMapping: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.importFilesState.collectAsStateWithLifecycle()
    MemoryImportContent(state = state, onBack = onBack, onProceedToMapping = onProceedToMapping)
}

@Composable
fun MemoryImportContent(
    state: ScreenState<List<ImportFileItem>>,
    onBack: () -> Unit,
    onProceedToMapping: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "导入记忆", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "upload_zone") {
                val interactionSource = remember { MutableInteractionSource() }
                Surface(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = {}
                        ),
                    shape = RoundedCornerShape(20.dp),
                    color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                    border = androidx.compose.foundation.BorderStroke(
                        2.dp,
                        MaterialTheme.colorScheme.primary.copy(alpha = 0.3f)
                    )
                ) {
                    Column(
                        modifier = Modifier.padding(AmitiaSpacing.Xxl),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(56.dp)
                                .clip(RoundedCornerShape(16.dp))
                                .background(MaterialTheme.colorScheme.primaryContainer),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.CloudUpload,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                                modifier = Modifier.size(AmitiaIconSize.Huge)
                            )
                        }
                        Text(
                            text = "选择文件导入",
                            style = MaterialTheme.typography.titleSmall,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Text(
                            text = "支持 JSON、CSV、Amitia备份等格式",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            item(key = "formats_header") {
                AmitiaSectionHeader(title = "支持的格式")
            }

            item(key = "formats") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                    FormatItem(name = "结构化聊天记录", desc = "JSON / CSV 格式的聊天历史")
                    FormatItem(name = "Amitia 备份", desc = "Amitia 导出的备份文件")
                    FormatItem(name = "第三方格式", desc = "SillyTavern、TavernAI 等角色卡")
                }
            }

            when (state) {
                is ScreenState.Loading -> {
                    item(key = "loading") {
                        Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                            InlineLoading(message = "加载文件...")
                        }
                    }
                }
                is ScreenState.Error -> {
                    item(key = "error") {
                        AmitiaErrorState(
                            icon = AmitiaIcons.Error,
                            title = "加载失败",
                            description = state.error.message,
                            onRetry = {},
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Empty -> {
                    item(key = "empty") {
                        AmitiaEmptyState(
                            icon = AmitiaIcons.Folder,
                            title = "暂无文件",
                            description = "点击上方区域选择要导入的文件",
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Content -> {
                    item(key = "files_header") {
                        AmitiaSectionHeader(title = "已选文件")
                    }
                    items(state.data, key = { it.id }) { file ->
                        ImportFileCard(item = file)
                    }
                    item(key = "proceed") {
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        PrimaryButton(
                            text = "下一步：字段映射",
                            onClick = onProceedToMapping,
                            modifier = Modifier.fillMaxWidth(),
                            leadingIcon = AmitiaIcons.ArrowForward
                        )
                    }
                }
                is ScreenState.Partial -> {
                    items(state.data, key = { it.id }) { file ->
                        ImportFileCard(item = file)
                    }
                }
            }
        }
    }
}

@Composable
private fun FormatItem(name: String, desc: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = AmitiaSpacing.Xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Icon(
            imageVector = AmitiaIcons.CheckCircle,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.tertiary,
            modifier = Modifier.size(AmitiaIconSize.Small)
        )
        Column {
            Text(
                text = name,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = desc,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun ImportFileCard(item: ImportFileItem) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
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
                    .clip(RoundedCornerShape(10.dp))
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.FileCopy,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = item.format,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = item.size,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Preview(name = "Import - Light", showBackground = true)
@Composable
private fun MemoryImportLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemoryImportContent(
            state = ScreenState.Content(
                listOf(
                    ImportFileItem("1", "chat_history.json", "1.2 MB", "JSON"),
                    ImportFileItem("2", "amitia_backup.zip", "5.8 MB", "Amitia备份"),
                    ImportFileItem("3", "messages.csv", "820 KB", "CSV")
                )
            ),
            onBack = {},
            onProceedToMapping = {}
        )
    }
}

@Preview(name = "Import - Dark", showBackground = true)
@Composable
private fun MemoryImportDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemoryImportContent(
            state = ScreenState.Empty(),
            onBack = {},
            onProceedToMapping = {}
        )
    }
}
