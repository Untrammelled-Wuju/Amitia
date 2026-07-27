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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun PendingMemoryScreen(
    onBack: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.pendingState.collectAsStateWithLifecycle()
    var selectedIds by remember { mutableStateOf(setOf<String>()) }
    PendingMemoryContent(
        state = state,
        selectedIds = selectedIds,
        onToggleSelect = { id ->
            selectedIds = if (id in selectedIds) selectedIds - id else selectedIds + id
        },
        onSelectAll = { items ->
            selectedIds = if (selectedIds.size == items.size) emptySet() else items.map { it.id }.toSet()
        },
        onBack = onBack
    )
}

@Composable
fun PendingMemoryContent(
    state: ScreenState<List<PendingMemoryItem>>,
    selectedIds: Set<String>,
    onToggleSelect: (String) -> Unit,
    onSelectAll: (List<PendingMemoryItem>) -> Unit,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "待确认记忆", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载待确认记忆...")
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
                    icon = AmitiaIcons.HelpOutlined,
                    title = "暂无待确认记忆",
                    description = "AI建议的记忆会出现在这里等待确认",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> {
                PendingMemoryList(
                    items = state.data,
                    selectedIds = selectedIds,
                    onToggleSelect = onToggleSelect,
                    onSelectAll = { onSelectAll(state.data) }
                )
            }
            is ScreenState.Partial -> {
                PendingMemoryList(
                    items = state.data,
                    selectedIds = selectedIds,
                    onToggleSelect = onToggleSelect,
                    onSelectAll = { onSelectAll(state.data) }
                )
            }
        }
    }
}

@Composable
private fun PendingMemoryList(
    items: List<PendingMemoryItem>,
    selectedIds: Set<String>,
    onToggleSelect: (String) -> Unit,
    onSelectAll: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item(key = "batch_header") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "${selectedIds.size}/${items.size} 已选",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                val interactionSource = remember { MutableInteractionSource() }
                Text(
                    text = if (selectedIds.size == items.size) "取消全选" else "全选",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                    modifier = Modifier
                        .clip(RoundedCornerShape(20.dp))
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onSelectAll
                        )
                        .padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
                )
            }
        }
        items(items, key = { it.id }) { item ->
            PendingMemoryCard(
                item = item,
                isSelected = item.id in selectedIds,
                onToggleSelect = { onToggleSelect(item.id) }
            )
        }
        if (selectedIds.isNotEmpty()) {
            item(key = "batch_actions") {
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    PrimaryButton(
                        text = "批量接受",
                        onClick = {},
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Check
                    )
                    DangerButton(
                        text = "批量拒绝",
                        onClick = {},
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Close
                    )
                }
            }
        }
    }
}

@Composable
private fun PendingMemoryCard(
    item: PendingMemoryItem,
    isSelected: Boolean,
    onToggleSelect: () -> Unit
) {
    val borderColor = if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface,
        border = androidx.compose.foundation.BorderStroke(if (isSelected) 2.dp else 0.dp, borderColor)
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                verticalAlignment = Alignment.Top,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                val interactionSource = remember { MutableInteractionSource() }
                Box(
                    modifier = Modifier
                        .size(24.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .background(if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant)
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Checkbox,
                            onClick = onToggleSelect
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    if (isSelected) {
                        Icon(
                            imageVector = AmitiaIcons.Check,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.size(16.dp)
                        )
                    }
                }
                Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(
                        text = item.content,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 3,
                        overflow = TextOverflow.Ellipsis
                    )
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Text(
                            text = item.suggestType,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.primary,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = "·",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Text(
                            text = item.source,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Text(
                            text = "·",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Text(
                            text = item.timestamp,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                PrimaryButton(
                    text = "接受",
                    onClick = {},
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Check
                )
                SecondaryButton(
                    text = "编辑后接受",
                    onClick = {},
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Edit
                )
                SecondaryButton(
                    text = "拒绝",
                    onClick = {},
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Close
                )
            }
        }
    }
}

@Preview(name = "Pending - Light", showBackground = true)
@Composable
private fun PendingMemoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PendingMemoryContent(
            state = ScreenState.Content(
                listOf(
                    PendingMemoryItem("1", "用户下周有出差计划", "今日对话推断", "长期记忆", "14:35"),
                    PendingMemoryItem("2", "用户喜欢喝咖啡", "昨日对话", "用户偏好", "昨天 10:20"),
                    PendingMemoryItem("3", "用户的猫叫橘子", "前日对话", "用户事实", "7月25日")
                )
            ),
            selectedIds = setOf("1"),
            onToggleSelect = {},
            onSelectAll = {},
            onBack = {}
        )
    }
}

@Preview(name = "Pending - Dark", showBackground = true)
@Composable
private fun PendingMemoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PendingMemoryContent(
            state = ScreenState.Empty(),
            selectedIds = emptySet(),
            onToggleSelect = {},
            onSelectAll = {},
            onBack = {}
        )
    }
}
