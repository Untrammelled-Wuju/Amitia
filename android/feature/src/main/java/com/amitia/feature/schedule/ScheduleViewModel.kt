package com.amitia.feature.schedule

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.designsystem.EmptyReason
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class ScheduleHomeData(
    val today: List<ScheduleItem> = emptyList(),
    val upcoming: List<ScheduleItem> = emptyList(),
    val weekOverview: WeekOverview = WeekOverview(0, 0, 0, 0),
    val proactiveWindow: ProactiveMessageWindow = ScheduleMockData.proactiveWindow
)

@HiltViewModel
class ScheduleHomeViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ScheduleHomeData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ScheduleHomeData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(400)
                ScheduleHomeData(
                    today = ScheduleMockData.todaySchedules,
                    upcoming = ScheduleMockData.upcomingSchedules,
                    weekOverview = ScheduleMockData.weekOverview,
                    proactiveWindow = ScheduleMockData.proactiveWindow
                )
            }.onSuccess { data ->
                _state.value = if (data.today.isEmpty() && data.upcoming.isEmpty()) {
                    ScreenState.Empty(EmptyReason.NoData)
                } else {
                    ScreenState.Content(data)
                }
            }.onFailure { e ->
                _state.value = ScreenState.Error(
                    UiError(title = "加载日程失败", message = e.message ?: "未知错误")
                )
            }
        }
    }
}

data class ScheduleDetailData(
    val item: ScheduleItem
)

@HiltViewModel
class ScheduleDetailViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ScheduleDetailData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ScheduleDetailData>> = _state.asStateFlow()

    fun load(scheduleId: String) {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                val item = (
                    ScheduleMockData.todaySchedules +
                        ScheduleMockData.upcomingSchedules
                    ).firstOrNull { it.id == scheduleId }
                    ?: ScheduleMockData.todaySchedules.first().copy(id = scheduleId)
                ScheduleDetailData(item)
            }.onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载详情失败", message = it.message ?: "")) }
        }
    }

    fun delete(onDone: () -> Unit) {
        viewModelScope.launch { onDone() }
    }
}

data class CalendarData(
    val viewMode: CalendarViewMode = CalendarViewMode.Month,
    val days: List<CalendarDay> = emptyList(),
    val selectedDaySchedules: List<ScheduleItem> = emptyList()
)

@HiltViewModel
class ScheduleCalendarViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<CalendarData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<CalendarData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                kotlinx.coroutines.delay(300)
                CalendarData(
                    viewMode = CalendarViewMode.Month,
                    days = buildMockMonth(),
                    selectedDaySchedules = ScheduleMockData.todaySchedules
                )
            }.onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载日历失败", message = it.message ?: "")) }
        }
    }

    fun switchView(mode: CalendarViewMode) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(current.copy(viewMode = mode))
    }

    fun selectDay(day: Int) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(
            current.copy(
                selectedDaySchedules = if (day == 17) ScheduleMockData.todaySchedules else emptyList()
            )
        )
    }

    private fun buildMockMonth(): List<CalendarDay> {
        val today = 17
        return (1..35).map { index ->
            val day = index - 2
            CalendarDay(
                day = day.coerceAtLeast(1),
                isCurrentMonth = day in 1..31,
                isToday = day == today,
                scheduleCount = when (day) {
                    today -> 3
                    18 -> 2
                    20 -> 1
                    25 -> 2
                    else -> 0
                },
                hasRoleSchedule = day == today || day == 25
            )
        }
    }
}
