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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
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
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.InlineLoading

data class PickerTab(val label: String, val selected: Boolean)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EmojiPickerSheet(
    onDismiss: () -> Unit,
    onSend: (String) -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val state by viewModel.pickerState.collectAsStateWithLifecycle()
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var keyword by remember { mutableStateOf("") }
    var tabs by remember {
        mutableStateOf(
            listOf(
                PickerTab("最近使用", true),
                PickerTab("日常表情", false),
                PickerTab("情绪表达", false),
                PickerTab("角色专属", false)
            )
        )
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = MaterialTheme.colorScheme.background
    ) {
        EmojiPickerContent(
            state = state,
            keyword = keyword,
            onKeywordChange = { keyword = it },
            tabs = tabs,
            onTabSelected = { index ->
                tabs = tabs.mapIndexed { i, tab ->
                    tab.copy(selected = i == index)
                }
            },
            onSend = onSend
        )
    }
}

@Composable
fun EmojiPickerContent(
    state: ScreenState<List<EmojiItem>>,
    keyword: String,
    onKeywordChange: (String) -> Unit,
    tabs: List<PickerTab>,
    onTabSelected: (Int) -> Unit,
    onSend: (String) -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .height(420.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = "选择表情包",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium
            )
            val interactionSource = remember { MutableInteractionSource() }
            Text(
                text = "关闭",
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier
                    .clip(RoundedCornerShape(20.dp))
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = {}
                    )
                    .padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
            )
        }

        AmitiaSearchField(
            value = keyword,
            onValueChange = onKeywordChange,
            placeholder = "搜索含义",
            onClear = { onKeywordChange("") },
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)
        )

        LazyRow(
            modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            items(tabs.size) { index ->
                val tab = tabs[index]
                val interactionSource = remember { MutableInteractionSource() }
                Surface(
                    modifier = Modifier
                        .clip(RoundedCornerShape(20.dp))
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Tab,
                            onClick = { onTabSelected(index) }
                        ),
                    shape = RoundedCornerShape(20.dp),
                    color = if (tab.selected) MaterialTheme.colorScheme.primaryContainer
                    else MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = tab.label,
                        style = MaterialTheme.typography.labelMedium,
                        color = if (tab.selected) MaterialTheme.colorScheme.onPrimaryContainer
                        else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
                    )
                }
            }
        }

        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载表情包...")
                }
            }
            is ScreenState.Error -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text(
                        text = "加载失败",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Image,
                    title = "暂无表情包",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> EmojiPickerGrid(items = state.data, onSend = onSend)
            is ScreenState.Partial -> EmojiPickerGrid(items = state.data, onSend = onSend)
        }
    }
}

@Composable
private fun EmojiPickerGrid(items: List<EmojiItem>, onSend: (String) -> Unit) {
    LazyVerticalGrid(
        columns = GridCells.Fixed(4),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(items, key = { it.id }) { emoji ->
            PickerEmojiCell(emoji = emoji, onSend = { onSend(emoji.id) })
        }
    }
}

@Composable
private fun PickerEmojiCell(emoji: EmojiItem, onSend: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .aspectRatio(1f)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onSend
            ),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
    ) {
        Box(contentAlignment = Alignment.Center) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center
            ) {
                Box(
                    modifier = Modifier
                        .size(40.dp)
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
                Spacer(modifier = Modifier.height(2.dp))
                Text(
                    text = emoji.meaning ?: "未知",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Preview(name = "Emoji Picker - Light", showBackground = true)
@Composable
private fun EmojiPickerLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiPickerContent(
            state = ScreenState.Content(
                listOf(
                    EmojiItem("1", "", "开心"),
                    EmojiItem("2", "", "惊讶"),
                    EmojiItem("3", "", "思考"),
                    EmojiItem("4", "", "无奈"),
                    EmojiItem("5", "", "赞同"),
                    EmojiItem("6", "", "害羞")
                )
            ),
            keyword = "",
            onKeywordChange = {},
            tabs = listOf(
                PickerTab("最近使用", true),
                PickerTab("日常表情", false)
            ),
            onTabSelected = {},
            onSend = {}
        )
    }
}

@Preview(name = "Emoji Picker - Dark", showBackground = true)
@Composable
private fun EmojiPickerDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiPickerContent(
            state = ScreenState.Empty(),
            keyword = "",
            onKeywordChange = {},
            tabs = listOf(PickerTab("最近使用", true)),
            onTabSelected = {},
            onSend = {}
        )
    }
}
