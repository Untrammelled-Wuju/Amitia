package com.amitia.feature.schedule

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCharacterAccents
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
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ScheduleHomeScreen(
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    onNewSchedule: () -> Unit,
    onOpenCalendar: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit,
    viewModel: ScheduleHomeViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ScheduleHomeContent(
        state = state,
        onBack = onBack,
        onMenu = onMenu,
        onNewSchedule = onNewSchedule,
        onOpenCalendar = onOpenCalendar,
        onOpenDetail = onOpenDetail,
        onOpenProactiveWindow = onOpenProactiveWindow,
        onRetry = viewModel::load
    )
}

@Composable
fun ScheduleHomeContent(
    state: ScreenState<ScheduleHomeData>,
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    onNewSchedule: () -> Unit,
    onOpenCalendar: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit,
    onRetry: () -> Unit
) {
    Box(modifier = Modifier.fillMaxSize()) {
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.EventOutlined,
                title = "今日暂无日程",
                description = "点击下方按钮创建你的第一个日程",
                modifier = Modifier.fillMaxSize(),
                primaryAction = { PrimaryButton(text = "新建日程", onClick = onNewSchedule, leadingIcon = AmitiaIcons.Add) }
            )
            is ScreenState.Content -> ScheduleHomeBody(
                data = state.data,
                onBack = onBack,
                onMenu = onMenu,
                onNewSchedule = onNewSchedule,
                onOpenDetail = onOpenDetail,
                onOpenProactiveWindow = onOpenProactiveWindow
            )
            is ScreenState.Partial -> ScheduleHomeBody(
                data = state.data,
                onBack = onBack,
                onMenu = onMenu,
                onNewSchedule = onNewSchedule,
                onOpenDetail = onOpenDetail,
                onOpenProactiveWindow = onOpenProactiveWindow
            )
        }
        if (state is ScreenState.Content || state is ScreenState.Partial) {
            FloatingAddButton(
                onClick = onNewSchedule,
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = AmitiaContentPadding.Horizontal, bottom = AmitiaSpacing.Lg)
            )
        }
    }
}

@Composable
private fun ScheduleHomeBody(
    data: ScheduleHomeData,
    onBack: () -> Unit,
    onMenu: () -> Unit,
    onNewSchedule: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit
) {
    var selectedDateIndex by remember { mutableStateOf(1) }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            horizontal = AmitiaContentPadding.Horizontal,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { PageBackTopBar(onMenu = onMenu) }
        item {
            DateStrip(
                selectedIndex = selectedDateIndex,
                onSelect = { selectedDateIndex = it }
            )
        }
        item { QuoteCard(scheduleCount = data.today.size) }
        item { ScheduleSectionTitle(title = "今日安排", onAdd = onNewSchedule) }
        if (data.today.isEmpty()) {
            item { EmptyInlineHint(text = "今日暂无日程安排") }
        } else {
            items(data.today.size) { index ->
                AgendaItem(
                    item = data.today[index],
                    onClick = { onOpenDetail(data.today[index].id) }
                )
            }
        }
        item { ScheduleSectionTitle(title = "Amitia 的提醒") }
        item {
            MemoryReminderCard(
                window = data.proactiveWindow,
                onClick = onOpenProactiveWindow
            )
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun PageBackTopBar(onMenu: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .statusBarsPadding()
            .padding(vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        AmitiaGlassSurface(
            level = GlassLevel.Chip,
            modifier = Modifier.size(42.dp),
            shape = RoundedCornerShape(15.dp)
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button,
                        onClick = onMenu
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Menu,
                    contentDescription = "菜单",
                    tint = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
        }
        Text(
            text = "日程与提醒",
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

private val mockDates = listOf(
    "一" to "27",
    "二" to "28",
    "三" to "29",
    "四" to "30",
    "五" to "31",
    "六" to "1"
)

@Composable
private fun DateStrip(selectedIndex: Int, onSelect: (Int) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        mockDates.forEachIndexed { index, (weekday, day) ->
            DateChip(
                weekday = weekday,
                day = day,
                isActive = index == selectedIndex,
                onClick = { onSelect(index) }
            )
        }
    }
}

@Composable
private fun DateChip(weekday: String, day: String, isActive: Boolean, onClick: () -> Unit) {
    val bgColor = if (isActive) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface
    val dayColor = if (isActive) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurface
    val weekdayColor = if (isActive) MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f) else MaterialTheme.colorScheme.onSurfaceVariant
    val borderColor = if (isActive) Color.Transparent else MaterialTheme.colorScheme.outline

    Column(
        modifier = Modifier
            .widthIn(min = 54.dp)
            .height(68.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(bgColor)
            .border(1.dp, borderColor, RoundedCornerShape(18.dp))
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = null,
                role = Role.Tab,
                onClick = onClick
            )
            .padding(horizontal = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(
            text = weekday,
            style = MaterialTheme.typography.labelSmall,
            color = weekdayColor
        )
        Text(
            text = day,
            style = MaterialTheme.typography.titleMedium,
            color = dayColor,
            fontSize = 17.sp,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun QuoteCard(scheduleCount: Int) {
    AmitiaGlassSurface(
        level = GlassLevel.Sheet,
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(21.dp)
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "今天 · $scheduleCount 项安排",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "空闲 5 小时",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "上午有一场项目同步，下午专注开发，晚间复盘今日进展。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun ScheduleSectionTitle(title: String, onAdd: (() -> Unit)? = null) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = title,
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        if (onAdd != null) {
            Text(
                text = "新增",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.primary,
                fontSize = 12.sp,
                modifier = Modifier.clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    role = Role.Button,
                    onClick = onAdd
                )
            )
        }
    }
}

@Composable
private fun AgendaItem(item: ScheduleItem, onClick: () -> Unit) {
    val borderColor = if (item.status == ScheduleStatus.Ongoing) {
        MaterialTheme.colorScheme.primary
    } else {
        AmitiaCharacterAccents.WarmAmber
    }

    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.Top
    ) {
        Text(
            text = item.startTime,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.width(50.dp)
        )
        Surface(
            modifier = Modifier
                .weight(1f)
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    role = Role.Button,
                    onClick = onClick
                ),
            shape = RoundedCornerShape(20.dp),
            color = MaterialTheme.colorScheme.surface
        ) {
            Row(modifier = Modifier.height(IntrinsicSize.Min)) {
                Box(
                    modifier = Modifier
                        .width(3.dp)
                        .fillMaxHeight()
                        .background(borderColor)
                )
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xxs))
                    Text(
                        text = "${item.role} · ${item.source.label}" + (item.channel?.let { " · $it" } ?: ""),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        fontSize = 11.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
    }
}

@Composable
private fun MemoryReminderCard(window: ProactiveMessageWindow, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
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
                    text = "主动消息",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                    fontSize = 10.sp
                )
                Text(
                    text = "${window.startTime} - ${window.endTime}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                    fontSize = 10.sp
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            Text(
                text = "主动消息时间窗",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxs))
            Text(
                text = "每日 ${window.frequencyPerDay} 次，间隔不少于 ${window.minIntervalMinutes} 分钟。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 12.sp
            )
        }
    }
}

@Composable
private fun FloatingAddButton(onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .size(52.dp)
            .shadow(8.dp, RoundedCornerShape(20.dp))
            .clip(RoundedCornerShape(20.dp))
            .background(MaterialTheme.colorScheme.primary)
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.Add,
            contentDescription = "新增日程",
            tint = MaterialTheme.colorScheme.onPrimary,
            modifier = Modifier.size(24.dp)
        )
    }
}

@Composable
private fun EmptyInlineHint(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.bodySmall,
        color = AmitiaColors.OnSurfaceMuted,
        modifier = Modifier.padding(vertical = AmitiaSpacing.Sm)
    )
}

@Preview(name = "ScheduleHome - Light", showBackground = true)
@Composable
private fun ScheduleHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ScheduleHomeContent(
            state = ScreenState.Content(
                ScheduleHomeData(
                    today = ScheduleMockData.todaySchedules,
                    upcoming = ScheduleMockData.upcomingSchedules,
                    weekOverview = ScheduleMockData.weekOverview
                )
            ),
            onBack = {}, onNewSchedule = {}, onOpenCalendar = {}, onOpenDetail = {}, onOpenProactiveWindow = {}, onRetry = {}
        )
    }
}

@Preview(name = "ScheduleHome - Dark", showBackground = true)
@Composable
private fun ScheduleHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ScheduleHomeContent(
            state = ScreenState.Loading,
            onBack = {}, onNewSchedule = {}, onOpenCalendar = {}, onOpenDetail = {}, onOpenProactiveWindow = {}, onRetry = {}
        )
    }
}
