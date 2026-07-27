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
import androidx.compose.foundation.layout.width
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
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

data class TimelineFilter(val label: String, val selected: Boolean)

@Composable
fun MemoryTimelineScreen(
    onBack: () -> Unit,
    onMemoryDetail: (String) -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.timelineState.collectAsStateWithLifecycle()
    var filters by remember {
        mutableStateOf(
            listOf(
                TimelineFilter("全部", true),
                TimelineFilter("对话", false),
                TimelineFilter("推断", false),
                TimelineFilter("初始", false)
            )
        )
    }
    MemoryTimelineContent(
        state = state,
        filters = filters,
        onFilterToggle = { index ->
            filters = filters.mapIndexed { i, f ->
                if (i == 0) f.copy(selected = index == 0)
                else if (index == 0) f.copy(selected = false)
                else if (i == index) f.copy(selected = !f.selected)
                else f
            }
        },
        onBack = onBack,
        onMemoryDetail = onMemoryDetail
    )
}

@Composable
fun MemoryTimelineContent(
    state: ScreenState<List<MemoryTimelineGroup>>,
    filters: List<TimelineFilter>,
    onFilterToggle: (Int) -> Unit,
    onBack: () -> Unit,
    onMemoryDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "记忆时间线", onBack = onBack)
        LazyRow(
            modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            items(filters.size) { index ->
                val filter = filters[index]
                val interactionSource = remember { MutableInteractionSource() }
                Surface(
                    modifier = Modifier
                        .clip(AmitiaPillShape)
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Tab,
                            onClick = { onFilterToggle(index) }
                        ),
                    shape = AmitiaPillShape,
                    color = if (filter.selected) MaterialTheme.colorScheme.primaryContainer
                    else MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = filter.label,
                        style = MaterialTheme.typography.labelMedium,
                        color = if (filter.selected) MaterialTheme.colorScheme.onPrimaryContainer
                        else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
                    )
                }
            }
        }
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载时间线...")
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
                    icon = AmitiaIcons.Schedule,
                    title = "暂无记忆",
                    description = "随着对话的进行，记忆会自动保存到这里",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> TimelineGroupList(groups = state.data, onMemoryDetail = onMemoryDetail)
            is ScreenState.Partial -> TimelineGroupList(groups = state.data, onMemoryDetail = onMemoryDetail)
        }
    }
}

@Composable
private fun TimelineGroupList(groups: List<MemoryTimelineGroup>, onMemoryDetail: (String) -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        groups.forEach { group ->
            item(key = "header_${group.title}") {
                AmitiaSectionHeader(title = group.title)
            }
            items(group.items, key = { it.id }) { entry ->
                TimelineEntryCard(entry = entry, onClick = { onMemoryDetail(entry.id) })
            }
        }
    }
}

@Composable
private fun TimelineEntryCard(entry: MemoryTimelineEntry, onClick: () -> Unit) {
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
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            TimelineDot(importance = entry.importance)
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                Text(
                    text = entry.content,
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
                        text = entry.timestamp,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
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
            ImportancePill(level = entry.importance)
        }
    }
}

@Composable
private fun TimelineDot(importance: Int) {
    val color = when {
        importance >= 4 -> MaterialTheme.colorScheme.error
        importance >= 3 -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Box(
        modifier = Modifier
            .padding(top = 4.dp)
            .size(12.dp)
            .clip(RoundedCornerShape(6.dp))
            .background(color)
    )
}

@Composable
private fun ImportancePill(level: Int) {
    val color = when {
        level >= 4 -> MaterialTheme.colorScheme.error
        level >= 3 -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        shape = RoundedCornerShape(8.dp),
        color = color.copy(alpha = 0.12f)
    ) {
        Text(
            text = "P$level",
            style = MaterialTheme.typography.labelSmall,
            color = color,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
        )
    }
}

@Preview(name = "Timeline - Light", showBackground = true)
@Composable
private fun MemoryTimelineLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemoryTimelineContent(
            state = ScreenState.Content(
                listOf(
                    MemoryTimelineGroup("今天", listOf(
                        MemoryTimelineEntry("1", "用户提到今天有重要会议", "14:30", "对话", "艾米", 4),
                        MemoryTimelineEntry("2", "用户偏好简洁回复风格", "13:15", "推断", "艾米", 3)
                    )),
                    MemoryTimelineGroup("最近七天", listOf(
                        MemoryTimelineEntry("3", "用户周末喜欢看电影放松", "7月25日", "对话", "艾米", 2)
                    ))
                )
            ),
            filters = listOf(
                TimelineFilter("全部", true),
                TimelineFilter("对话", false),
                TimelineFilter("推断", false)
            ),
            onFilterToggle = {},
            onBack = {},
            onMemoryDetail = {}
        )
    }
}

@Preview(name = "Timeline - Dark", showBackground = true)
@Composable
private fun MemoryTimelineDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemoryTimelineContent(
            state = ScreenState.Empty(),
            filters = listOf(TimelineFilter("全部", true)),
            onFilterToggle = {},
            onBack = {},
            onMemoryDetail = {}
        )
    }
}
