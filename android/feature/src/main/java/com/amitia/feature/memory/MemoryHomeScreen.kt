package com.amitia.feature.memory

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun MemoryHomeScreen(
    onSearch: () -> Unit,
    onTimeline: () -> Unit,
    onLongTerm: () -> Unit,
    onWorldBook: () -> Unit,
    onGraph: () -> Unit,
    onPending: () -> Unit,
    onMemoryDetail: (String) -> Unit,
    onMenu: () -> Unit = {},
    onCreate: () -> Unit = {},
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
        onMemoryDetail = onMemoryDetail,
        onMenu = onMenu,
        onCreate = onCreate
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
    onMemoryDetail: (String) -> Unit,
    onMenu: () -> Unit = {},
    onCreate: () -> Unit = {}
) {
    var selectedTab by remember { mutableStateOf(0) }
    Box(modifier = Modifier.fillMaxSize()) {
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaContentPadding.Horizontal)
        ) {
            item(key = "topline") {
                MemoryTopLine(onMenu = onMenu, onSearch = onSearch, onNew = onCreate)
            }
            item(key = "segmented") {
                MemorySegmentedControl(
                    selectedIndex = selectedTab,
                    onSelect = { index ->
                        selectedTab = index
                        when (index) {
                            0 -> onTimeline()
                            1 -> onLongTerm()
                            2 -> onGraph()
                        }
                    }
                )
            }
            when (state) {
                is ScreenState.Loading -> {
                    item(key = "loading") {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 80.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            InlineLoading(message = "加载记忆...")
                        }
                    }
                }
                is ScreenState.Error -> {
                    item(key = "error") {
                        AmitiaErrorState(
                            icon = AmitiaIcons.Error,
                            title = "加载失败",
                            description = (state as ScreenState.Error).error.message,
                            onRetry = {},
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 40.dp)
                        )
                    }
                }
                is ScreenState.Empty -> {
                    item(key = "empty") {
                        AmitiaEmptyState(
                            icon = AmitiaIcons.Memory,
                            title = "暂无记忆",
                            description = "随着对话的进行，记忆会自动保存到这里",
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 40.dp)
                        )
                    }
                }
                is ScreenState.Content -> {
                    val data = (state as ScreenState.Content<List<MemoryTimelineGroup>>).data
                    data.forEachIndexed { groupIndex, group ->
                        item(key = "day_${groupIndex}") {
                            TimelineDayHeader(title = group.title)
                        }
                        items(group.items, key = { "mem_${groupIndex}_${it.id}" }) { entry ->
                            TimelineItem(
                                entry = entry,
                                onClick = { onMemoryDetail(entry.id) }
                            )
                        }
                    }
                    item(key = "bottom_spacer") {
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
                    }
                }
                is ScreenState.Partial -> {
                    val data = (state as ScreenState.Partial<List<MemoryTimelineGroup>>).data
                    data.forEachIndexed { groupIndex, group ->
                        item(key = "day_${groupIndex}") {
                            TimelineDayHeader(title = group.title)
                        }
                        items(group.items, key = { "mem_${groupIndex}_${it.id}" }) { entry ->
                            TimelineItem(
                                entry = entry,
                                onClick = { onMemoryDetail(entry.id) }
                            )
                        }
                    }
                    item(key = "bottom_spacer") {
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
                    }
                }
            }
        }
    }
}

@Composable
private fun MemoryTopLine(
    onMenu: () -> Unit,
    onSearch: () -> Unit,
    onNew: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        TopLineIconButton(
            icon = AmitiaIcons.Menu,
            contentDescription = "菜单",
            onClick = onMenu
        )
        Column(
            modifier = Modifier.weight(1f),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "记忆",
                fontSize = 27.sp,
                fontWeight = FontWeight(620),
                color = MaterialTheme.colorScheme.onBackground
            )
            Text(
                text = "关系不是记录，而是持续发生。",
                fontSize = 13.sp,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
        Row(
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            TopLineIconButton(
                icon = AmitiaIcons.Search,
                contentDescription = "搜索记忆",
                onClick = onSearch
            )
            TopLineIconButton(
                icon = AmitiaIcons.Add,
                contentDescription = "新建记忆",
                onClick = onNew
            )
        }
    }
}

@Composable
private fun TopLineIconButton(
    icon: ImageVector,
    contentDescription: String?,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    AmitiaGlassSurface(
        level = GlassLevel.Chip,
        modifier = Modifier.size(44.dp),
        shape = RoundedCornerShape(16.dp)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    role = Role.Button,
                    onClick = onClick
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun MemorySegmentedControl(
    selectedIndex: Int,
    onSelect: (Int) -> Unit
) {
    val tabs = listOf("时间线", "卡片", "关系图谱")
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier.padding(4.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            tabs.forEachIndexed { index, label ->
                val isActive = index == selectedIndex
                Surface(
                    onClick = { onSelect(index) },
                    modifier = Modifier
                        .weight(1f)
                        .height(36.dp),
                    shape = RoundedCornerShape(12.dp),
                    color = if (isActive) MaterialTheme.colorScheme.surface else Color.Transparent,
                    shadowElevation = if (isActive) 2.dp else 0.dp
                ) {
                    Box(
                        modifier = Modifier.fillMaxSize(),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = label,
                            fontSize = 12.sp,
                            color = if (isActive) MaterialTheme.colorScheme.onSurface else AmitiaColors.OnSurfaceMuted,
                            fontWeight = if (isActive) FontWeight.SemiBold else FontWeight.Normal
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun TimelineDayHeader(title: String) {
    Text(
        text = title,
        fontSize = 12.sp,
        fontWeight = FontWeight(650),
        color = AmitiaColors.OnSurfaceMuted,
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 10.dp, bottom = 5.dp, start = 2.dp, end = 2.dp)
    )
}

@Composable
private fun TimelineItem(
    entry: MemoryTimelineEntry,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Max)
            .padding(bottom = 9.dp),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        TimelineAxis()
        MemoryCard(entry = entry, onClick = onClick)
    }
}

@Composable
private fun TimelineAxis() {
    Column(
        modifier = Modifier
            .width(16.dp)
            .fillMaxHeight(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier
                .size(13.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.6f)),
            contentAlignment = Alignment.Center
        ) {
            Box(
                modifier = Modifier
                    .size(9.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary)
            )
        }
        Box(
            modifier = Modifier
                .width(1.dp)
                .weight(1f)
                .background(MaterialTheme.colorScheme.outlineVariant)
        )
    }
}

@Composable
private fun MemoryCard(
    entry: MemoryTimelineEntry,
    onClick: () -> Unit
) {
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
        shape = RoundedCornerShape(21.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = entry.source,
                    fontSize = 10.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                )
                Text(
                    text = entry.timestamp,
                    fontSize = 10.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = entry.content,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = entry.character,
                fontSize = 12.sp,
                color = AmitiaColors.OnSurfaceMuted,
                lineHeight = 19.sp,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
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
            onMemoryDetail = {},
            onMenu = {},
            onCreate = {}
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
            onMemoryDetail = {},
            onMenu = {},
            onCreate = {}
        )
    }
}
