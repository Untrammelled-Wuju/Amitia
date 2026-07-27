package com.amitia.feature.memory

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.model.MemoryDto
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class MemoryPagesViewModel @Inject constructor() : ViewModel() {

    private val _timelineState = MutableStateFlow<ScreenState<List<MemoryTimelineGroup>>>(ScreenState.Loading)
    val timelineState: StateFlow<ScreenState<List<MemoryTimelineGroup>>> = _timelineState.asStateFlow()

    private val _searchState = MutableStateFlow<ScreenState<List<MemoryDto>>>(ScreenState.Loading)
    val searchState: StateFlow<ScreenState<List<MemoryDto>>> = _searchState.asStateFlow()

    private val _longTermState = MutableStateFlow<ScreenState<List<LongTermMemoryGroup>>>(ScreenState.Loading)
    val longTermState: StateFlow<ScreenState<List<LongTermMemoryGroup>>> = _longTermState.asStateFlow()

    private val _episodicState = MutableStateFlow<ScreenState<List<EpisodicMemoryItem>>>(ScreenState.Loading)
    val episodicState: StateFlow<ScreenState<List<EpisodicMemoryItem>>> = _episodicState.asStateFlow()

    private val _worldBookListState = MutableStateFlow<ScreenState<List<WorldBookGroup>>>(ScreenState.Loading)
    val worldBookListState: StateFlow<ScreenState<List<WorldBookGroup>>> = _worldBookListState.asStateFlow()

    private val _worldBookDetailState = MutableStateFlow<ScreenState<WorldBookDetail>>(ScreenState.Loading)
    val worldBookDetailState: StateFlow<ScreenState<WorldBookDetail>> = _worldBookDetailState.asStateFlow()

    private val _graphState = MutableStateFlow<ScreenState<MemoryGraphView>>(ScreenState.Loading)
    val graphState: StateFlow<ScreenState<MemoryGraphView>> = _graphState.asStateFlow()

    private val _pendingState = MutableStateFlow<ScreenState<List<PendingMemoryItem>>>(ScreenState.Loading)
    val pendingState: StateFlow<ScreenState<List<PendingMemoryItem>>> = _pendingState.asStateFlow()

    private val _conflictState = MutableStateFlow<ScreenState<List<MemoryConflictItem>>>(ScreenState.Loading)
    val conflictState: StateFlow<ScreenState<List<MemoryConflictItem>>> = _conflictState.asStateFlow()

    private val _importFilesState = MutableStateFlow<ScreenState<List<ImportFileItem>>>(ScreenState.Loading)
    val importFilesState: StateFlow<ScreenState<List<ImportFileItem>>> = _importFilesState.asStateFlow()

    private val _settingsState = MutableStateFlow(MemorySettingsConfig())
    val settingsState: StateFlow<MemorySettingsConfig> = _settingsState.asStateFlow()

    private val _exportConfig = MutableStateFlow(ExportConfig())
    val exportConfig: StateFlow<ExportConfig> = _exportConfig.asStateFlow()

    init {
        loadTimeline()
        loadLongTerm()
        loadEpisodic()
        loadWorldBookList()
        loadGraph()
        loadPending()
        loadConflict()
        loadImportFiles()
    }

    fun loadTimeline() {
        viewModelScope.launch {
            _timelineState.value = ScreenState.Loading
            delay(500)
            _timelineState.value = ScreenState.Content(sampleTimeline())
        }
    }

    fun search(filters: MemorySearchFilters) {
        viewModelScope.launch {
            _searchState.value = ScreenState.Loading
            delay(400)
            if (filters.keyword.isBlank()) {
                _searchState.value = ScreenState.Empty()
                return@launch
            }
            _searchState.value = ScreenState.Content(sampleSearchResults())
        }
    }

    fun loadLongTerm() {
        viewModelScope.launch {
            _longTermState.value = ScreenState.Loading
            delay(500)
            _longTermState.value = ScreenState.Content(sampleLongTerm())
        }
    }

    fun loadEpisodic() {
        viewModelScope.launch {
            _episodicState.value = ScreenState.Loading
            delay(500)
            _episodicState.value = ScreenState.Content(sampleEpisodic())
        }
    }

    fun loadWorldBookList() {
        viewModelScope.launch {
            _worldBookListState.value = ScreenState.Loading
            delay(500)
            _worldBookListState.value = ScreenState.Content(sampleWorldBooks())
        }
    }

    fun loadWorldBookDetail(id: String) {
        viewModelScope.launch {
            _worldBookDetailState.value = ScreenState.Loading
            delay(400)
            _worldBookDetailState.value = ScreenState.Content(sampleWorldBookDetail(id))
        }
    }

    fun loadGraph() {
        viewModelScope.launch {
            _graphState.value = ScreenState.Loading
            delay(600)
            _graphState.value = ScreenState.Content(sampleGraph())
        }
    }

    fun loadPending() {
        viewModelScope.launch {
            _pendingState.value = ScreenState.Loading
            delay(500)
            _pendingState.value = ScreenState.Content(samplePending())
        }
    }

    fun loadConflict() {
        viewModelScope.launch {
            _conflictState.value = ScreenState.Loading
            delay(500)
            _conflictState.value = ScreenState.Content(sampleConflicts())
        }
    }

    fun loadImportFiles() {
        viewModelScope.launch {
            _importFilesState.value = ScreenState.Loading
            delay(400)
            _importFilesState.value = ScreenState.Content(sampleImportFiles())
        }
    }

    fun updateSettings(update: (MemorySettingsConfig) -> MemorySettingsConfig) {
        _settingsState.value = update(_settingsState.value)
    }

    fun updateExportConfig(update: (ExportConfig) -> ExportConfig) {
        _exportConfig.value = update(_exportConfig.value)
    }

    private fun sampleTimeline(): List<MemoryTimelineGroup> = listOf(
        MemoryTimelineGroup("今天", listOf(
            MemoryTimelineEntry("1", "用户提到今天有重要会议", "14:30", "对话", "艾米", 4),
            MemoryTimelineEntry("2", "用户偏好简洁回复风格", "13:15", "推断", "艾米", 3)
        )),
        MemoryTimelineGroup("最近七天", listOf(
            MemoryTimelineEntry("3", "用户周末喜欢看电影放松", "7月25日", "对话", "艾米", 2),
            MemoryTimelineEntry("4", "用户的工作领域是软件开发", "7月24日", "对话", "艾米", 5)
        )),
        MemoryTimelineGroup("更早", listOf(
            MemoryTimelineEntry("5", "用户不喜欢被催促", "7月20日", "推断", "艾米", 3)
        )),
        MemoryTimelineGroup("长期节点", listOf(
            MemoryTimelineEntry("6", "用户名是小明", "初始", "初始", "艾米", 5)
        ))
    )

    private fun sampleSearchResults(): List<MemoryDto> = listOf(
        MemoryDto("1", "用户偏好简洁回复风格", "long_term", "global", "1", 3.0, createdAt = "今天 13:15"),
        MemoryDto("2", "用户提到今天有重要会议", "episodic", "global", "1", 4.0, createdAt = "今天 14:30")
    )

    private fun sampleLongTerm(): List<LongTermMemoryGroup> = listOf(
        LongTermMemoryGroup("用户事实", listOf(
            MemoryDto("1", "用户名是小明", "long_term", createdAt = "初始"),
            MemoryDto("2", "用户的工作是软件开发", "long_term", createdAt = "7月24日")
        )),
        LongTermMemoryGroup("用户偏好", listOf(
            MemoryDto("3", "偏好简洁回复风格", "long_term", createdAt = "今天"),
            MemoryDto("4", "不喜欢被催促", "long_term", createdAt = "7月20日")
        )),
        LongTermMemoryGroup("关系事实", listOf(
            MemoryDto("5", "与艾米是朋友关系", "long_term", createdAt = "初始")
        )),
        LongTermMemoryGroup("重要经历", listOf(
            MemoryDto("6", "去年完成了重要项目", "long_term", createdAt = "更早")
        )),
        LongTermMemoryGroup("长期约束", emptyList()),
        LongTermMemoryGroup("角色自身事实", listOf(
            MemoryDto("7", "艾米是温柔知性助手", "long_term", createdAt = "初始")
        ))
    )

    private fun sampleEpisodic(): List<EpisodicMemoryItem> = listOf(
        EpisodicMemoryItem("1", "下午会议讨论", "用户提到有会议", "讨论了新方案但未最终定", "方案待确定", listOf("我", "艾米"), "今天 14:30"),
        EpisodicMemoryItem("2", "周末看电影", "用户周末想放松", "看了科幻电影", "心情愉快", listOf("我", "艾米"), "7月25日")
    )

    private fun sampleWorldBooks(): List<WorldBookGroup> = listOf(
        WorldBookGroup("1", "基础世界观", true, "艾米", 12, "今天"),
        WorldBookGroup("2", "角色背景设定", true, "艾米", 8, "昨天"),
        WorldBookGroup("3", "场景设定", false, "通用", 5, "7月20日")
    )

    private fun sampleWorldBookDetail(id: String): WorldBookDetail = WorldBookDetail(
        id = id,
        name = "基础世界观",
        description = "包含故事背景、地理环境等基础设定",
        enabled = true,
        triggerRule = "关键词匹配",
        priority = 1,
        scope = "艾米",
        entries = listOf(
            WorldBookEntry("1", "城市设定", listOf("城市", "地点"), "故事发生在一座沿海城市", true, 1, "艾米"),
            WorldBookEntry("2", "时间设定", listOf("时间", "年代"), "现代都市背景", true, 2, "艾米"),
            WorldBookEntry("3", "气候设定", listOf("天气", "气候"), "温暖湿润的海洋性气候", false, 3, "艾米")
        )
    )

    private fun sampleGraph(): MemoryGraphView {
        val nodes = listOf(
            MemoryGraphNodeView("1", "小明", "character", 0.5f, 0.2f),
            MemoryGraphNodeView("2", "艾米", "character", 0.5f, 0.5f),
            MemoryGraphNodeView("3", "软件开发", "person", 0.2f, 0.7f),
            MemoryGraphNodeView("4", "沿海城市", "place", 0.8f, 0.7f),
            MemoryGraphNodeView("5", "下午会议", "event", 0.3f, 0.3f)
        )
        val edges = listOf(
            MemoryGraphEdgeView("1", "2", "朋友"),
            MemoryGraphEdgeView("1", "3", "职业"),
            MemoryGraphEdgeView("2", "4", "所在地"),
            MemoryGraphEdgeView("1", "5", "参与")
        )
        return MemoryGraphView(nodes, edges, totalNodes = 28, totalEdges = 42)
    }

    private fun samplePending(): List<PendingMemoryItem> = listOf(
        PendingMemoryItem("1", "用户下周有出差计划", "今日对话推断", "长期记忆", "14:35"),
        PendingMemoryItem("2", "用户喜欢喝咖啡", "昨日对话", "用户偏好", "昨天 10:20"),
        PendingMemoryItem("3", "用户的猫叫橘子", "前日对话", "用户事实", "7月25日")
    )

    private fun sampleConflicts(): List<MemoryConflictItem> = listOf(
        MemoryConflictItem("1", "用户所在地", "北京", "上海", "今日对话", "14:30", 0.8f),
        MemoryConflictItem("2", "用户职业", "设计师", "软件开发", "昨日对话", "昨天", 0.6f)
    )

    private fun sampleImportFiles(): List<ImportFileItem> = listOf(
        ImportFileItem("1", "chat_history.json", "1.2 MB", "JSON"),
        ImportFileItem("2", "amitia_backup.zip", "5.8 MB", "Amitia备份"),
        ImportFileItem("3", "messages.csv", "820 KB", "CSV")
    )
}
