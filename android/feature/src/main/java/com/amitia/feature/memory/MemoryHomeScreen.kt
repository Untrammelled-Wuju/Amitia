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
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.graphics.vector.ImageVector
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
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.model.MemoryDto

@Composable
fun MemoryHomeScreen(
    onSearch: () -> Unit,
    onTimeline: () -> Unit,
    onLongTerm: () -> Unit,
    onWorldBook: () -> Unit,
    onGraph: () -> Unit,
    onPending: () -> Unit,
    onMemoryDetail: (String) -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.timelineState.collectAsStateWithLifecycle()
    MemoryHomeContent(
        state = state,
        onSearch = onSearch,
        onTimeline = onTimeline,
        onLongTerm = onLongTerm,
        onWorldBook = onWorldBook,
        onGraph = onGraph,
        onPending = onPending,
        onMemoryDetail = onMemoryDetail
    )
}

@Composable
fun MemoryHomeContent(
    state: ScreenState<List<MemoryTimelineGroup>>,
    onSearch: () -> Unit,
    onTimeline: () -> Unit,
    onLongTerm: () -> Unit,
    onWorldBook: () -> Unit,
    onGraph: () -> Unit,
    onPending: () -> Unit,
    onMemoryDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "记忆")
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "search") {
                val interactionSource = remember { MutableInteractionSource() }
                Surface(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onSearch
                        ),
                    shape = RoundedCornerShape(28.dp),
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Search,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                        Text(
                            text = "搜索记忆",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            item(key = "entries") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    MemoryEntryRow(
                        icon = AmitiaIcons.Schedule,
                        title = "时间线",
                        subtitle = "按时间浏览所有记忆",
                        onClick = onTimeline
                    )
                    MemoryEntryRow(
                        icon = AmitiaIcons.Psychology,
                        title = "长期记忆",
                        subtitle = "用户事实、偏好与关系",
                        onClick = onLongTerm
                    )
                    MemoryEntryRow(
                        icon = AmitiaIcons.MenuBook,
                        title = "世界书",
                        subtitle = "世界观与背景设定",
                        onClick = onWorldBook
                    )
                    MemoryEntryRow(
                        icon = AmitiaIcons.Hub,
                        title = "记忆图谱",
                        subtitle = "可视化记忆关联",
                        onClick = onGraph
                    )
                    MemoryEntryRow(
                        icon = AmitiaIcons.HelpOutlined,
                        title = "待确认记忆",
                        subtitle = "AI建议写入的记忆",
                        onClick = onPending
                    )
                }
            }

            item(key = "recent_header") {
                AmitiaSectionHeader(title = "最近重要记忆")
            }

            when (state) {
                is ScreenState.Loading -> {
                    item(key = "loading") {
                        Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                            InlineLoading(message = "加载记忆...")
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
                            icon = AmitiaIcons.Memory,
                            title = "暂无记忆",
                            description = "随着对话的进行，记忆会自动保存到这里",
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Content -> {
                    val recentItems = state.data.flatMap { it.items }.take(5)
                    items(recentItems, key = { it.id }) { entry ->
                        RecentMemoryCard(entry = entry, onClick = { onMemoryDetail(entry.id) })
                    }
                }
                is ScreenState.Partial -> {
                    val recentItems = state.data.flatMap { it.items }.take(5)
                    items(recentItems, key = { it.id }) { entry ->
                        RecentMemoryCard(entry = entry, onClick = { onMemoryDetail(entry.id) })
                    }
                }
            }
        }
    }
}

@Composable
private fun MemoryEntryRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    AmitiaEntryCard(
        onClick = onClick,
        leading = {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        },
        title = title,
        subtitle = subtitle
    )
}

@Composable
private fun RecentMemoryCard(entry: MemoryTimelineEntry, onClick: () -> Unit) {
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
                    .size(6.dp, 36.dp)
                    .clip(RoundedCornerShape(3.dp))
                    .background(importanceColor(entry.importance))
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = entry.content,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = entry.timestamp,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = entry.source,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = entry.character,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            ImportanceBadge(level = entry.importance)
        }
    }
}

@Composable
private fun ImportanceBadge(level: Int) {
    val color = importanceColor(level)
    Surface(
        shape = RoundedCornerShape(8.dp),
        color = color.copy(alpha = 0.12f)
    ) {
        Text(
            text = "P$level",
            style = MaterialTheme.typography.labelSmall,
            color = color,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xxs)
        )
    }
}

private fun importanceColor(level: Int): androidx.compose.ui.graphics.Color {
    return when {
        level >= 4 -> androidx.compose.ui.graphics.Color(0xFFE85D4E)
        level >= 3 -> androidx.compose.ui.graphics.Color(0xFFF5A623)
        else -> androidx.compose.ui.graphics.Color(0xFF6B7280)
    }
}

@Preview(name = "Memory Home - Light", showBackground = true)
@Composable
private fun MemoryHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemoryHomeContent(
            state = ScreenState.Content(
                listOf(
                    MemoryTimelineGroup("今天", listOf(
                        MemoryTimelineEntry("1", "用户提到今天有重要会议", "14:30", "对话", "艾米", 4),
                        MemoryTimelineEntry("2", "用户偏好简洁回复风格", "13:15", "推断", "艾米", 3)
                    ))
                )
            ),
            onSearch = {},
            onTimeline = {},
            onLongTerm = {},
            onWorldBook = {},
            onGraph = {},
            onPending = {},
            onMemoryDetail = {}
        )
    }
}

@Preview(name = "Memory Home - Dark", showBackground = true)
@Composable
private fun MemoryHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemoryHomeContent(
            state = ScreenState.Empty(),
            onSearch = {},
            onTimeline = {},
            onLongTerm = {},
            onWorldBook = {},
            onGraph = {},
            onPending = {},
            onMemoryDetail = {}
        )
    }
}
