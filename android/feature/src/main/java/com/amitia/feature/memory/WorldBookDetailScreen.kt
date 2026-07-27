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
import androidx.compose.runtime.LaunchedEffect
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
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun WorldBookDetailScreen(
    bookId: String,
    onBack: () -> Unit,
    onEditEntry: (String) -> Unit,
    onAddEntry: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.worldBookDetailState.collectAsStateWithLifecycle()
    LaunchedEffect(bookId) { viewModel.loadWorldBookDetail(bookId) }
    WorldBookDetailContent(state = state, onBack = onBack, onEditEntry = onEditEntry, onAddEntry = onAddEntry)
}

@Composable
fun WorldBookDetailContent(
    state: ScreenState<WorldBookDetail>,
    onBack: () -> Unit,
    onEditEntry: (String) -> Unit,
    onAddEntry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "世界书详情", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载详情...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = "加载失败",
                    description = state.error.message,
                    onRetry = {},
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.MenuBook,
                    title = "未找到世界书",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> WorldBookDetailBody(detail = state.data, onEditEntry = onEditEntry, onAddEntry = onAddEntry)
            is ScreenState.Partial -> WorldBookDetailBody(detail = state.data, onEditEntry = onEditEntry, onAddEntry = onAddEntry)
        }
    }
}

@Composable
private fun WorldBookDetailBody(detail: WorldBookDetail, onEditEntry: (String) -> Unit, onAddEntry: () -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item(key = "info") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Text(
                        text = detail.name,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Text(
                        text = detail.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                    ) {
                        InfoChip(label = "触发", value = detail.triggerRule)
                        InfoChip(label = "优先级", value = detail.priority.toString())
                        InfoChip(label = "范围", value = detail.scope)
                        InfoChip(label = "状态", value = if (detail.enabled) "启用" else "停用")
                    }
                }
            }
        }

        item(key = "header") {
            AmitiaSectionHeader(
                title = "条目列表",
                trailing = {
                    val interactionSource = remember { MutableInteractionSource() }
                    Row(
                        modifier = Modifier
                            .clip(RoundedCornerShape(20.dp))
                            .clickable(
                                interactionSource = interactionSource,
                                indication = null,
                                role = Role.Button,
                                onClick = onAddEntry
                            )
                            .background(MaterialTheme.colorScheme.primaryContainer)
                            .padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Add,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                        Text(
                            text = "新建",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onPrimaryContainer
                        )
                    }
                }
            )
        }

        if (detail.entries.isEmpty()) {
            item(key = "empty_entries") {
                AmitiaEmptyState(
                    icon = AmitiaIcons.MenuBook,
                    title = "暂无条目",
                    description = "点击新建添加世界书条目",
                    modifier = Modifier.fillMaxWidth()
                )
            }
        } else {
            items(detail.entries, key = { it.id }) { entry ->
                WorldBookEntryCard(entry = entry, onClick = { onEditEntry(entry.id) })
            }
        }
    }
}

@Composable
private fun InfoChip(label: String, value: String) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun WorldBookEntryCard(entry: WorldBookEntry, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(6.dp, 36.dp)
                    .clip(RoundedCornerShape(3.dp))
                    .background(if (entry.enabled) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.onSurfaceVariant)
            )
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(
                    text = entry.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = entry.content,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
                if (entry.keywords.isNotEmpty()) {
                    Text(
                        text = "关键词：${entry.keywords.joinToString("、")}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
                    )
                }
            }
            Text(
                text = "P${entry.priority}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Preview(name = "WorldBook Detail - Light", showBackground = true)
@Composable
private fun WorldBookDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        WorldBookDetailContent(
            state = ScreenState.Content(
                WorldBookDetail(
                    id = "1",
                    name = "基础世界观",
                    description = "包含故事背景、地理环境等基础设定",
                    enabled = true,
                    triggerRule = "关键词匹配",
                    priority = 1,
                    scope = "艾米",
                    entries = listOf(
                        WorldBookEntry("1", "城市设定", listOf("城市", "地点"), "故事发生在一座沿海城市", true, 1, "艾米"),
                        WorldBookEntry("2", "时间设定", listOf("时间", "年代"), "现代都市背景", true, 2, "艾米")
                    )
                )
            ),
            onBack = {},
            onEditEntry = {},
            onAddEntry = {}
        )
    }
}

@Preview(name = "WorldBook Detail - Dark", showBackground = true)
@Composable
private fun WorldBookDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        WorldBookDetailContent(
            state = ScreenState.Loading,
            onBack = {},
            onEditEntry = {},
            onAddEntry = {}
        )
    }
}
