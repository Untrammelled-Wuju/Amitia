package com.amitia.feature.emoji

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun EmojiGroupDetailScreen(
    groupId: String,
    onBack: () -> Unit,
    onEditEmoji: (String) -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val state by viewModel.groupDetailState.collectAsStateWithLifecycle()
    LaunchedEffect(groupId) { viewModel.loadGroupDetail(groupId) }
    var selectMode by remember { mutableStateOf(false) }
    var selectedIds by remember { mutableStateOf(setOf<String>()) }
    EmojiGroupDetailContent(
        state = state,
        selectMode = selectMode,
        selectedIds = selectedIds,
        onToggleSelectMode = { selectMode = !selectMode },
        onToggleSelect = { id ->
            selectedIds = if (id in selectedIds) selectedIds - id else selectedIds + id
        },
        onBack = onBack,
        onEditEmoji = onEditEmoji
    )
}

@Composable
fun EmojiGroupDetailContent(
    state: ScreenState<List<EmojiItem>>,
    selectMode: Boolean,
    selectedIds: Set<String>,
    onToggleSelectMode: () -> Unit,
    onToggleSelect: (String) -> Unit,
    onBack: () -> Unit,
    onEditEmoji: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = if (selectMode) "已选${selectedIds.size}项" else "表情包分组",
            onBack = onBack,
            actions = {
                val interactionSource = remember { MutableInteractionSource() }
                Text(
                    text = if (selectMode) "完成" else "选择",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.primary,
                    modifier = Modifier
                        .clip(RoundedCornerShape(20.dp))
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onToggleSelectMode
                        )
                        .padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
                )
            }
        )
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载表情包...")
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
                    icon = AmitiaIcons.Image,
                    title = "暂无表情",
                    description = "点击导入添加表情包到此分组",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> EmojiGrid(
                items = state.data,
                selectMode = selectMode,
                selectedIds = selectedIds,
                onToggleSelect = onToggleSelect,
                onEditEmoji = onEditEmoji
            )
            is ScreenState.Partial -> EmojiGrid(
                items = state.data,
                selectMode = selectMode,
                selectedIds = selectedIds,
                onToggleSelect = onToggleSelect,
                onEditEmoji = onEditEmoji
            )
        }
    }
}

@Composable
private fun EmojiGrid(
    items: List<EmojiItem>,
    selectMode: Boolean,
    selectedIds: Set<String>,
    onToggleSelect: (String) -> Unit,
    onEditEmoji: (String) -> Unit
) {
    LazyVerticalGrid(
        columns = GridCells.Fixed(3),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(items, key = { it.id }) { emoji ->
            EmojiGridItem(
                emoji = emoji,
                selectMode = selectMode,
                isSelected = emoji.id in selectedIds,
                onToggleSelect = { onToggleSelect(emoji.id) },
                onClick = { onEditEmoji(emoji.id) }
            )
        }
    }
}

@Composable
private fun EmojiGridItem(
    emoji: EmojiItem,
    selectMode: Boolean,
    isSelected: Boolean,
    onToggleSelect: () -> Unit,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val borderColor = if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface
    Surface(
        modifier = Modifier
            .aspectRatio(1f)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = { if (selectMode) onToggleSelect() else onClick() }
            ),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
        border = androidx.compose.foundation.BorderStroke(if (isSelected) 2.dp else 0.dp, borderColor)
    ) {
        Box(contentAlignment = Alignment.Center) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center
            ) {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(MaterialTheme.colorScheme.surfaceVariant),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Image,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                        modifier = Modifier.size(AmitiaIconSize.Nav)
                    )
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                Text(
                    text = emoji.meaning ?: "待完善",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (emoji.meaning == null) MaterialTheme.colorScheme.tertiary
                    else MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            if (selectMode && isSelected) {
                Box(
                    modifier = Modifier
                        .align(Alignment.TopEnd)
                        .padding(4.dp)
                        .size(20.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(MaterialTheme.colorScheme.primary),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.size(14.dp)
                    )
                }
            }
        }
    }
}

@Preview(name = "Emoji Group Detail - Light", showBackground = true)
@Composable
private fun EmojiGroupDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiGroupDetailContent(
            state = ScreenState.Content(
                listOf(
                    EmojiItem("1", "", "开心", importedAt = "今天"),
                    EmojiItem("2", "", "惊讶", importedAt = "今天"),
                    EmojiItem("3", "", "思考", importedAt = "昨天"),
                    EmojiItem("4", "", null, importedAt = "昨天", needsMeaning = true),
                    EmojiItem("5", "", "无奈", importedAt = "7月20日"),
                    EmojiItem("6", "", "赞同", importedAt = "7月20日")
                )
            ),
            selectMode = false,
            selectedIds = emptySet(),
            onToggleSelectMode = {},
            onToggleSelect = {},
            onBack = {},
            onEditEmoji = {}
        )
    }
}

@Preview(name = "Emoji Group Detail - Dark", showBackground = true)
@Composable
private fun EmojiGroupDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiGroupDetailContent(
            state = ScreenState.Empty(),
            selectMode = true,
            selectedIds = emptySet(),
            onToggleSelectMode = {},
            onToggleSelect = {},
            onBack = {},
            onEditEmoji = {}
        )
    }
}
