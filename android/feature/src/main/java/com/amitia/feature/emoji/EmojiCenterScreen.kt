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
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.LazyRow
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
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EmojiCenterScreen(
    onBack: () -> Unit,
    onOpenGroup: (String) -> Unit,
    onBatchImport: () -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val state by viewModel.centerState.collectAsStateWithLifecycle()
    var keyword by remember { mutableStateOf("") }
    EmojiCenterContent(
        state = state,
        keyword = keyword,
        onKeywordChange = { keyword = it },
        onBack = onBack,
        onOpenGroup = onOpenGroup,
        onBatchImport = onBatchImport
    )
}

@Composable
fun EmojiCenterContent(
    state: ScreenState<List<EmojiGroupItem>>,
    keyword: String,
    onKeywordChange: (String) -> Unit,
    onBack: () -> Unit,
    onOpenGroup: (String) -> Unit,
    onBatchImport: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "表情包", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "search") {
                AmitiaSearchField(
                    value = keyword,
                    onValueChange = onKeywordChange,
                    placeholder = "搜索表情含义",
                    onClear = { onKeywordChange("") }
                )
            }

            item(key = "stats") {
                val total = (state as? ScreenState.Content)?.data?.sumOf { it.count } ?: 0
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(20.dp),
                    color = MaterialTheme.colorScheme.primaryContainer
                ) {
                    Row(
                        modifier = Modifier.padding(AmitiaSpacing.Base),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(48.dp)
                                .clip(RoundedCornerShape(12.dp))
                                .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.2f)),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Image,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                                modifier = Modifier.size(AmitiaIconSize.Nav)
                            )
                        }
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "$total 个表情包",
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onPrimaryContainer,
                                fontWeight = FontWeight.Medium
                            )
                            Text(
                                text = "分组管理你的表情包资产",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                            )
                        }
                    }
                }
            }

            item(key = "import") {
                PrimaryButton(
                    text = "批量导入",
                    onClick = onBatchImport,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.CloudUpload
                )
            }

            item(key = "header") {
                AmitiaSectionHeader(title = "分组")
            }

            when (state) {
                is ScreenState.Loading -> {
                    item(key = "loading") {
                        Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                            InlineLoading(message = "加载表情包...")
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
                            icon = AmitiaIcons.Image,
                            title = "暂无表情包",
                            description = "点击批量导入添加表情包",
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Content -> {
                    items(state.data, key = { it.id }) { group ->
                        EmojiGroupCard(item = group, onClick = { onOpenGroup(group.id) })
                    }
                }
                is ScreenState.Partial -> {
                    items(state.data, key = { it.id }) { group ->
                        EmojiGroupCard(item = group, onClick = { onOpenGroup(group.id) })
                    }
                }
            }
        }
    }
}

@Composable
private fun EmojiGroupCard(item: EmojiGroupItem, onClick: () -> Unit) {
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
                modifier = Modifier
                    .size(44.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(
                        if (item.isUngrouped) MaterialTheme.colorScheme.surfaceVariant
                        else MaterialTheme.colorScheme.tertiaryContainer
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (item.isUngrouped) AmitiaIcons.Folder else AmitiaIcons.Image,
                    contentDescription = null,
                    tint = if (item.isUngrouped) MaterialTheme.colorScheme.onSurfaceVariant
                    else MaterialTheme.colorScheme.onTertiaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Nav)
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
                        text = "${item.count}个",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = item.lastImported,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

@Preview(name = "Emoji Center - Light", showBackground = true)
@Composable
private fun EmojiCenterLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiCenterContent(
            state = ScreenState.Content(
                listOf(
                    EmojiGroupItem("1", "日常表情", 24, null, "今天"),
                    EmojiGroupItem("2", "情绪表达", 18, null, "昨天"),
                    EmojiGroupItem("3", "未分组", 5, null, "7月15日", isUngrouped = true)
                )
            ),
            keyword = "",
            onKeywordChange = {},
            onBack = {},
            onOpenGroup = {},
            onBatchImport = {}
        )
    }
}

@Preview(name = "Emoji Center - Dark", showBackground = true)
@Composable
private fun EmojiCenterDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiCenterContent(
            state = ScreenState.Empty(),
            keyword = "",
            onKeywordChange = {},
            onBack = {},
            onOpenGroup = {},
            onBatchImport = {}
        )
    }
}
