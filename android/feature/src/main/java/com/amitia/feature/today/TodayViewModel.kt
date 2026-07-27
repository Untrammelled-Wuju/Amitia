package com.amitia.feature.today

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.model.CharacterDto
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class TodayViewModel @Inject constructor() : ViewModel() {

    private val _todayState = MutableStateFlow<ScreenState<TodaySummary>>(ScreenState.Loading)
    val todayState: StateFlow<ScreenState<TodaySummary>> = _todayState.asStateFlow()

    private val _activityState = MutableStateFlow<ScreenState<List<TodayActivity>>>(ScreenState.Loading)
    val activityState: StateFlow<ScreenState<List<TodayActivity>>> = _activityState.asStateFlow()

    private val _notificationState = MutableStateFlow<ScreenState<List<NotificationItem>>>(ScreenState.Loading)
    val notificationState: StateFlow<ScreenState<List<NotificationItem>>> = _notificationState.asStateFlow()

    private val _searchState = MutableStateFlow<GlobalSearchUiState>(GlobalSearchUiState())
    val searchState: StateFlow<GlobalSearchUiState> = _searchState.asStateFlow()

    private val _issuesState = MutableStateFlow<ScreenState<List<RuntimeIssue>>>(ScreenState.Loading)
    val issuesState: StateFlow<ScreenState<List<RuntimeIssue>>> = _issuesState.asStateFlow()

    private val _filter = MutableStateFlow(ActivityFilter.All)
    val filter: StateFlow<ActivityFilter> = _filter.asStateFlow()

    private val _notificationFilter = MutableStateFlow(NotificationFilter.All)
    val notificationFilter: StateFlow<NotificationFilter> = _notificationFilter.asStateFlow()

    init {
        loadAll()
    }

    fun loadAll() {
        loadToday()
        loadActivities()
        loadNotifications()
        loadIssues()
    }

    fun loadToday() {
        viewModelScope.launch {
            _todayState.value = ScreenState.Loading
            delay(300)
            runCatching { sampleToday() }
                .onSuccess { _todayState.value = ScreenState.Content(it) }
                .onFailure { _todayState.value = ScreenState.Error(sampleError("加载今日数据失败")) }
        }
    }

    fun loadActivities() {
        viewModelScope.launch {
            _activityState.value = ScreenState.Loading
            delay(300)
            runCatching { sampleActivities() }
                .onSuccess { _activityState.value = ScreenState.Content(it) }
                .onFailure { _activityState.value = ScreenState.Error(sampleError("加载活动动态失败")) }
        }
    }

    fun loadNotifications() {
        viewModelScope.launch {
            _notificationState.value = ScreenState.Loading
            delay(300)
            runCatching { sampleNotifications() }
                .onSuccess { _notificationState.value = ScreenState.Content(it) }
                .onFailure { _notificationState.value = ScreenState.Error(sampleError("加载通知失败")) }
        }
    }

    fun loadIssues() {
        viewModelScope.launch {
            _issuesState.value = ScreenState.Loading
            delay(300)
            runCatching { sampleIssues() }
                .onSuccess {
                    _issuesState.value = if (it.isEmpty()) ScreenState.Empty()
                    else ScreenState.Content(it)
                }
                .onFailure { _issuesState.value = ScreenState.Error(sampleError("加载异常失败")) }
        }
    }

    fun setActivityFilter(filter: ActivityFilter) { _filter.value = filter }

    fun setNotificationFilter(filter: NotificationFilter) { _notificationFilter.value = filter }

    fun markNotificationRead(id: String) {
        val current = (_notificationState.value as? ScreenState.Content)?.data ?: return
        _notificationState.value = ScreenState.Content(
            current.map { if (it.id == id) it.copy(read = true) else it }
        )
    }

    fun markAllNotificationsRead() {
        val current = (_notificationState.value as? ScreenState.Content)?.data ?: return
        _notificationState.value = ScreenState.Content(current.map { it.copy(read = true) })
    }

    fun updateQuery(query: String) {
        _searchState.value = _searchState.value.copy(query = query)
        if (query.isBlank()) {
            _searchState.value = _searchState.value.copy(results = emptyList(), searched = false)
            return
        }
        viewModelScope.launch {
            delay(250)
            _searchState.value = _searchState.value.copy(searching = true)
            runCatching { sampleSearchResults(query) }
                .onSuccess {
                    _searchState.value = _searchState.value.copy(
                        results = it, searching = false, searched = true
                    )
                }
                .onFailure {
                    _searchState.value = _searchState.value.copy(searching = false, searched = true)
                }
        }
    }

    fun clearSearch() {
        _searchState.value = GlobalSearchUiState()
    }

    fun retryFix(issueId: String) {
        val current = (_issuesState.value as? ScreenState.Content)?.data ?: return
        _issuesState.value = ScreenState.Content(current.filterNot { it.id == issueId })
    }
}

data class GlobalSearchUiState(
    val query: String = "",
    val searching: Boolean = false,
    val searched: Boolean = false,
    val results: List<SearchResultGroup> = emptyList()
)

enum class ActivityFilter(val label: String) {
    All("全部"), Character("角色"), Channel("渠道"), Memory("记忆"), Schedule("日程")
}

enum class NotificationFilter(val label: String) {
    All("全部"), CharacterMessage("角色消息"), Schedule("日程提醒"),
    Channel("渠道异常"), Update("扩展更新"), System("系统与安全")
}

private fun sampleError(message: String) = com.amitia.core.designsystem.UiError(
    title = "出错了", message = message, retryable = true
)

private fun sampleToday() = TodaySummary(
    greeting = "下午好",
    periodLabel = "14:32 · 周三",
    character = CharacterDto(
        id = "char_1", name = "艾米", isCurrent = true,
        description = "温柔知性的陪伴者", personality = "温暖、细腻、善于倾听",
        avatar = null, greeting = "今天也辛苦了，需要我陪你聊聊吗？"
    ),
    characterMood = "心情宁静，正在整理你昨天分享的笔记",
    characterActivity = "在翻阅你最近收藏的文章",
    proactivePreview = "想和你确认一下明天的会议安排，方便聊聊吗？",
    nextSchedule = ScheduleItem("s1", "产品评审会议", "16:00 - 17:00", description = "三号会议室"),
    recentMemory = MemoryBrief("m1", "用户偏好", "喜欢简洁的回复和深色界面", 4),
    online = true
)

private fun sampleActivities() = listOf(
    TodayActivity("a1", "艾米发来一条消息", "想和你确认明天会议的时间", "14:20", ActivityCategory.Character, "艾米", ActivityIconType.Chat),
    TodayActivity("a2", "微信渠道消息已接收", "来自群聊「设计组」的3条新消息", "13:55", ActivityCategory.Channel, null, ActivityIconType.Hub),
    TodayActivity("a3", "新增一条记忆", "用户偏好简洁的回复风格", "12:30", ActivityCategory.Memory, null, ActivityIconType.Memory),
    TodayActivity("a4", "日程提醒", "下午16:00产品评审会议", "09:00", ActivityCategory.Schedule, null, ActivityIconType.Event),
    TodayActivity("a5", "记忆索引已更新", "完成本周记忆的向量化", "08:15", ActivityCategory.System, null, ActivityIconType.Settings)
)

private fun sampleNotifications() = listOf(
    NotificationItem("n1", "艾米", "想和你确认明天会议的时间", "14:20", NotificationCategory.CharacterMessage, read = false),
    NotificationItem("n2", "日程提醒", "产品评审会议将在30分钟后开始", "15:30", NotificationCategory.Schedule, read = false),
    NotificationItem("n3", "微信渠道连接异常", "Token已过期，请重新授权", "13:00", NotificationCategory.Channel, read = false),
    NotificationItem("n4", "新版本可用", "天气扩展已更新至1.3.0", "昨天", NotificationCategory.Update, read = true),
    NotificationItem("n5", "安全提醒", "检测到新设备登录", "昨天", NotificationCategory.System, read = true)
)

private fun sampleIssues() = listOf(
    RuntimeIssue("i1", "微信渠道未连接", "授权Token已过期，需要重新登录授权", IssueLevel.Warning),
    RuntimeIssue("i2", "存储空间不足", "可用存储低于1GB，可能影响记忆写入", IssueLevel.Critical)
)

private fun sampleSearchResults(query: String): List<SearchResultGroup> = listOf(
    SearchResultGroup("g1", "对话", listOf(
        SearchResultItem("c1", "与艾米的对话", "包含「$query」", SearchItemType.Conversation)
    )),
    SearchResultGroup("g2", "记忆", listOf(
        SearchResultItem("m1", "用户偏好", "「$query」相关记忆", SearchItemType.Memory)
    )),
    SearchResultGroup("g3", "角色", listOf(
        SearchResultItem("ch1", "艾米", "温柔知性助手", SearchItemType.Character)
    ))
)
