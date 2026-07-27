package com.amitia.feature.schedule

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSegmentedTabs
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.amitiaStatusColor

@Composable
fun ScheduleCalendarScreen(
    onBack: () -> Unit,
    onOpenDetail: (String) -> Unit,
    viewModel: ScheduleCalendarViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ScheduleCalendarContent(
        state = state,
        onBack = onBack,
        onOpenDetail = onOpenDetail,
        onSwitchView = viewModel::switchView,
        onSelectDay = viewModel::selectDay,
        onRetry = viewModel::load
    )
}

@Composable
fun ScheduleCalendarContent(
    state: ScreenState<CalendarData>,
    onBack: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onSwitchView: (CalendarViewMode) -> Unit,
    onSelectDay: (Int) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "日历", onBack = onBack)
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
                icon = AmitiaIcons.CalendarMonth,
                title = "暂无日程数据",
                description = "日历加载完成后将在此显示",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> CalendarBody(
                data = state.data,
                onSwitchView = onSwitchView,
                onSelectDay = onSelectDay,
                onOpenDetail = onOpenDetail
            )
            is ScreenState.Partial -> CalendarBody(
                data = state.data,
                onSwitchView = onSwitchView,
                onSelectDay = onSelectDay,
                onOpenDetail = onOpenDetail
            )
        }
    }
}

@Composable
private fun CalendarBody(
    data: CalendarData,
    onSwitchView: (CalendarViewMode) -> Unit,
    onSelectDay: (Int) -> Unit,
    onOpenDetail: (String) -> Unit
) {
    val viewModes = CalendarViewMode.entries.map { it.label }
    val selectedIndex = CalendarViewMode.entries.indexOf(data.viewMode)
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item {
            AmitiaSegmentedTabs(
                tabs = viewModes,
                selectedIndex = selectedIndex,
                onSelected = { onSwitchView(CalendarViewMode.entries[it]) },
                modifier = Modifier.fillMaxWidth()
            )
        }
        item { CalendarLegend() }
        item { CalendarGrid(days = data.days, onSelectDay = onSelectDay) }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xs)) }
        item { AmitiaSectionHeader(title = "当日日程") }
        if (data.selectedDaySchedules.isEmpty()) {
            item {
                Text(
                    text = "所选日期暂无日程",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(vertical = AmitiaSpacing.Sm)
                )
            }
        } else {
            items(data.selectedDaySchedules.size) { index ->
                CalendarScheduleRow(item = data.selectedDaySchedules[index], onClick = { onOpenDetail(data.selectedDaySchedules[index].id) })
            }
        }
    }
}

@Composable
private fun CalendarLegend() {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically
    ) {
        LegendItem(color = MaterialTheme.colorScheme.primary, label = "我的日程")
        LegendItem(color = MaterialTheme.colorScheme.tertiary, label = "角色日程", icon = AmitiaIcons.Person)
    }
}

@Composable
private fun LegendItem(color: androidx.compose.ui.graphics.Color, label: String, icon: androidx.compose.ui.graphics.vector.ImageVector? = null) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        if (icon != null) {
            Icon(imageVector = icon, contentDescription = null, tint = color, modifier = Modifier.size(14.dp))
        } else {
            Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(color))
        }
        Text(text = label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun CalendarGrid(days: List<CalendarDay>, onSelectDay: (Int) -> Unit) {
    val weekDays = listOf("一", "二", "三", "四", "五", "六", "日")
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Sm)) {
            Row(modifier = Modifier.fillMaxWidth()) {
                weekDays.forEach { day ->
                    Text(
                        text = day,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.weight(1f).padding(vertical = AmitiaSpacing.Xs)
                    )
                }
            }
            days.chunked(7).forEach { week ->
                Row(modifier = Modifier.fillMaxWidth()) {
                    week.forEach { day ->
                        CalendarDayCell(day = day, modifier = Modifier.weight(1f), onClick = { onSelectDay(day.day) })
                    }
                }
            }
        }
    }
}

@Composable
private fun CalendarDayCell(day: CalendarDay, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val bg = when {
        day.isToday -> MaterialTheme.colorScheme.primary
        !day.isCurrentMonth -> androidx.compose.ui.graphics.Color.Transparent
        else -> androidx.compose.ui.graphics.Color.Transparent
    }
    val textColor = when {
        day.isToday -> MaterialTheme.colorScheme.onPrimary
        day.isCurrentMonth -> MaterialTheme.colorScheme.onSurface
        else -> MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.4f)
    }
    Box(
        modifier = modifier
            .aspectRatio(1f)
            .padding(2.dp)
            .clip(CircleShape)
            .background(bg)
            .clickable(enabled = day.isCurrentMonth, onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(
                text = if (day.isCurrentMonth) day.day.toString() else "",
                style = MaterialTheme.typography.bodyMedium,
                color = textColor,
                fontWeight = if (day.isToday) FontWeight.Medium else FontWeight.Normal
            )
            if (day.scheduleCount > 0 && day.isCurrentMonth) {
                Row(horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                    if (day.hasRoleSchedule) {
                        Box(
                            modifier = Modifier
                                .size(4.dp)
                                .clip(CircleShape)
                                .background(
                                    if (day.isToday) MaterialTheme.colorScheme.onPrimary
                                    else MaterialTheme.colorScheme.tertiary
                                )
                        )
                    }
                    if (day.scheduleCount > (if (day.hasRoleSchedule) 1 else 0)) {
                        Box(
                            modifier = Modifier
                                .size(4.dp)
                                .clip(CircleShape)
                                .background(
                                    if (day.isToday) MaterialTheme.colorScheme.onPrimary
                                    else MaterialTheme.colorScheme.primary
                                )
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun CalendarScheduleRow(item: ScheduleItem, onClick: () -> Unit) {
    val statusColor = amitiaStatusColor(item.status.statusType)
    Surface(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (item.isRoleSchedule) AmitiaIcons.Person else AmitiaIcons.Event,
                    contentDescription = null,
                    tint = if (item.isRoleSchedule) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${item.startTime} - ${item.endTime} · ${item.role}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
        }
    }
}

@Preview(name = "ScheduleCalendar - Light", showBackground = true)
@Composable
private fun ScheduleCalendarLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ScheduleCalendarContent(
            state = ScreenState.Content(
                CalendarData(
                    viewMode = CalendarViewMode.Month,
                    days = (1..35).map { idx ->
                        val d = idx - 2
                        CalendarDay(d.coerceAtLeast(1), d in 1..31, d == 17, if (d == 17) 3 else 0, d == 17)
                    },
                    selectedDaySchedules = ScheduleMockData.todaySchedules
                )
            ),
            onBack = {}, onOpenDetail = {}, onSwitchView = {}, onSelectDay = {}, onRetry = {}
        )
    }
}

@Preview(name = "ScheduleCalendar - Dark", showBackground = true)
@Composable
private fun ScheduleCalendarDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ScheduleCalendarContent(
            state = ScreenState.Empty(),
            onBack = {}, onOpenDetail = {}, onSwitchView = {}, onSelectDay = {}, onRetry = {}
        )
    }
}
