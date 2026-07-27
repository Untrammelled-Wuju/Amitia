package com.amitia.feature.chat

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.model.CharacterDto
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ChatExtendViewModel @Inject constructor() : ViewModel() {

    private val _conversationListState = MutableStateFlow<ScreenState<List<ConversationListItem>>>(ScreenState.Loading)
    val conversationListState: StateFlow<ScreenState<List<ConversationListItem>>> = _conversationListState.asStateFlow()

    private val _conversationQuery = MutableStateFlow("")
    val conversationQuery: StateFlow<String> = _conversationQuery.asStateFlow()

    private val _searchState = MutableStateFlow(MessageSearchUiState())
    val searchState: StateFlow<MessageSearchUiState> = _searchState.asStateFlow()

    private val _toolExecutionState = MutableStateFlow<ScreenState<ToolExecutionDetail>>(ScreenState.Loading)
    val toolExecutionState: StateFlow<ScreenState<ToolExecutionDetail>> = _toolExecutionState.asStateFlow()

    private val _contextState = MutableStateFlow<ScreenState<ContextSummary>>(ScreenState.Loading)
    val contextState: StateFlow<ScreenState<ContextSummary>> = _contextState.asStateFlow()

    private val _memoryReferencesState = MutableStateFlow<ScreenState<List<MemoryReferenceDetail>>>(ScreenState.Loading)
    val memoryReferencesState: StateFlow<ScreenState<List<MemoryReferenceDetail>>> = _memoryReferencesState.asStateFlow()

    private val _mediaState = MutableStateFlow<ScreenState<List<MediaItem>>>(ScreenState.Loading)
    val mediaState: StateFlow<ScreenState<List<MediaItem>>> = _mediaState.asStateFlow()

    private val _messageDetailState = MutableStateFlow<ScreenState<MessageDetailData>>(ScreenState.Loading)
    val messageDetailState: StateFlow<ScreenState<MessageDetailData>> = _messageDetailState.asStateFlow()

    private val _fileDetailState = MutableStateFlow<ScreenState<ChatFileInfo>>(ScreenState.Loading)
    val fileDetailState: StateFlow<ScreenState<ChatFileInfo>> = _fileDetailState.asStateFlow()

    private val _promptTraceState = MutableStateFlow<ScreenState<PromptTraceData>>(ScreenState.Loading)
    val promptTraceState: StateFlow<ScreenState<PromptTraceData>> = _promptTraceState.asStateFlow()

    private val _settings = MutableStateFlow(sampleSettings())
    val settings: StateFlow<ConversationSettings> = _settings.asStateFlow()

    private val _exportConfig = MutableStateFlow(ExportConfig(ExportFormat.Markdown, null, null, true, false, true))
    val exportConfig: StateFlow<ExportConfig> = _exportConfig.asStateFlow()

    private val _mergeHint = MutableStateFlow(MergeHintState(false, null, 0))
    val mergeHint: StateFlow<MergeHintState> = _mergeHint.asStateFlow()

    private val _availableCharacters = MutableStateFlow<List<CharacterDto>>(emptyList())
    val availableCharacters: StateFlow<List<CharacterDto>> = _availableCharacters.asStateFlow()

    private val _availableChannels = MutableStateFlow<List<ChannelOption>>(emptyList())
    val availableChannels: StateFlow<List<ChannelOption>> = _availableChannels.asStateFlow()

    init {
        loadConversations()
        loadCharacters()
        loadChannels()
    }

    fun loadConversations() {
        viewModelScope.launch {
            _conversationListState.value = ScreenState.Loading
            delay(300)
            runCatching { sampleConversations() }
                .onSuccess { _conversationListState.value = ScreenState.Content(it) }
                .onFailure { _conversationListState.value = ScreenState.Error(loadError("加载会话列表失败")) }
        }
    }

    fun updateConversationQuery(query: String) { _conversationQuery.value = query }

    fun pinConversation(id: String) = updateConversation(id) { it.copy(pinned = !it.pinned) }
    fun muteConversation(id: String) = updateConversation(id) { it.copy(muted = !it.muted) }
    fun archiveConversation(id: String) = updateConversation(id) { it.copy(archived = !it.archived) }

    fun deleteConversation(id: String) {
        val current = (_conversationListState.value as? ScreenState.Content)?.data ?: return
        _conversationListState.value = ScreenState.Content(current.filterNot { it.id == id })
    }

    private fun updateConversation(id: String, block: (ConversationListItem) -> ConversationListItem) {
        val current = (_conversationListState.value as? ScreenState.Content)?.data ?: return
        _conversationListState.value = ScreenState.Content(current.map { if (it.id == id) block(it) else it })
    }

    fun searchMessages(filter: MessageSearchFilter) {
        viewModelScope.launch {
            _searchState.value = _searchState.value.copy(searching = true, searched = false)
            delay(400)
            runCatching { sampleSearchResults(filter) }
                .onSuccess { _searchState.value = MessageSearchUiState(results = it, searching = false, searched = true) }
                .onFailure { _searchState.value = MessageSearchUiState(searching = false, searched = true, error = "搜索失败") }
        }
    }

    fun clearSearch() { _searchState.value = MessageSearchUiState() }

    fun loadToolExecution(id: String) {
        viewModelScope.launch {
            _toolExecutionState.value = ScreenState.Loading
            delay(300)
            _toolExecutionState.value = ScreenState.Content(sampleToolExecution(id))
        }
    }

    fun loadContext(conversationId: String) {
        viewModelScope.launch {
            _contextState.value = ScreenState.Loading
            delay(300)
            _contextState.value = ScreenState.Content(sampleContext())
        }
    }

    fun toggleContextItem(id: String) {
        val current = (_contextState.value as? ScreenState.Content)?.data ?: return
        _contextState.value = ScreenState.Content(
            current.copy(items = current.items.map { if (it.id == id) it.copy(included = !it.included) else it })
        )
    }

    fun loadMemoryReferences(messageId: String) {
        viewModelScope.launch {
            _memoryReferencesState.value = ScreenState.Loading
            delay(300)
            _memoryReferencesState.value = ScreenState.Content(sampleMemoryReferences())
        }
    }

    fun loadMedia(conversationId: String) {
        viewModelScope.launch {
            _mediaState.value = ScreenState.Loading
            delay(300)
            _mediaState.value = ScreenState.Content(sampleMedia())
        }
    }

    fun loadMessageDetail(messageId: String) {
        viewModelScope.launch {
            _messageDetailState.value = ScreenState.Loading
            delay(300)
            _messageDetailState.value = ScreenState.Content(sampleMessageDetail(messageId))
        }
    }

    fun loadFileDetail(fileId: String) {
        viewModelScope.launch {
            _fileDetailState.value = ScreenState.Loading
            delay(300)
            _fileDetailState.value = ScreenState.Content(sampleFile(fileId))
        }
    }

    fun loadPromptTrace(conversationId: String) {
        viewModelScope.launch {
            _promptTraceState.value = ScreenState.Loading
            delay(300)
            _promptTraceState.value = ScreenState.Content(samplePromptTrace())
        }
    }

    fun updateSetting(transform: (ConversationSettings) -> ConversationSettings) {
        _settings.value = transform(_settings.value)
    }

    fun updateExportConfig(transform: (ExportConfig) -> ExportConfig) {
        _exportConfig.value = transform(_exportConfig.value)
    }

    fun startMergeTimer() {
        viewModelScope.launch {
            _mergeHint.value = MergeHintState(true, "现在", 5)
            for (i in 5 downTo 1) {
                _mergeHint.value = _mergeHint.value.copy(remainingSeconds = i)
                delay(1000)
            }
            _mergeHint.value = MergeHintState(false, null, 0)
        }
    }

    fun cancelMerge() { _mergeHint.value = MergeHintState(false, null, 0) }

    private fun loadCharacters() {
        _availableCharacters.value = listOf(
            CharacterDto(id = "char_1", name = "艾米", isCurrent = true, description = "温柔知性的陪伴者"),
            CharacterDto(id = "char_2", name = "星野", description = "活泼开朗的助手"),
            CharacterDto(id = "char_3", name = "云溪", description = "沉稳冷静的顾问")
        )
    }

    private fun loadChannels() {
        _availableChannels.value = listOf(
            ChannelOption("web", "Web 对话", "应用内对话", available = true, isLastUsed = true),
            ChannelOption("wechat", "微信", "微信渠道消息", available = true),
            ChannelOption("qq", "QQ", "QQ 渠道消息", available = false)
        )
    }
}

data class MessageSearchUiState(
    val results: List<MessageSearchResult> = emptyList(),
    val searching: Boolean = false,
    val searched: Boolean = false,
    val error: String? = null
)

private fun loadError(message: String) = UiError(title = "出错了", message = message, retryable = true)

private fun sampleConversations() = listOf(
    ConversationListItem("c1", "与艾米的对话", "char_1", "艾米", null, "想和你确认明天会议的时间", "14:20", "web", unreadCount = 2, pinned = true, isCurrent = true),
    ConversationListItem("c2", "与星野的对话", "char_2", "星野", null, "今天天气真不错！", "13:05", "wechat", unreadCount = 0),
    ConversationListItem("c3", "与云溪的对话", "char_3", "云溪", null, "分析报告已发送", "昨天", "web", unreadCount = 5, muted = true),
    ConversationListItem("c4", "群聊讨论", null, "设计组", null, "新版本已发布", "昨天", "wechat", archived = true)
)

private fun sampleSearchResults(filter: MessageSearchFilter) = listOf(
    MessageSearchResult(
        com.amitia.core.model.MessageDto(id = "m1", role = "assistant", content = "关于${filter.keyword}的内容"),
        "与艾米的对话", "艾米", "...关于${filter.keyword}的讨论..."
    )
)

private fun sampleToolExecution(id: String) = ToolExecutionDetail(
    id = id, toolName = "weather_query", purpose = "查询用户所在城市天气",
    inputSummary = "city: 上海", outputSummary = "上海今日晴，28°C",
    status = com.amitia.core.designsystem.component.MessageStatus.Sent,
    duration = "0.3s", approved = true, requiresApproval = false,
    sensitiveFields = listOf("location")
)

private fun sampleContext() = ContextSummary(
    totalTokens = 8200, maxTokens = 32000,
    items = listOf(
        ContextItem("ctx1", "角色设定", "艾米的性格与设定", ContextType.Character, 500),
        ContextItem("ctx2", "近期对话", "最近10条消息", ContextType.RecentMessage, 3200),
        ContextItem("ctx3", "用户偏好", "喜欢简洁回复风格", ContextType.LongTermMemory, 800),
        ContextItem("ctx4", "项目文档.pdf", "上传的文件内容", ContextType.FileContext, 2500),
        ContextItem("ctx5", "天气查询结果", "工具返回数据", ContextType.ToolResult, 1200)
    )
)

private fun sampleMemoryReferences() = listOf(
    MemoryReferenceDetail("mr1", "用户位置偏好", "用户通常在上海", "对话记录", 0.95f, "昨天", "第3条回复"),
    MemoryReferenceDetail("mr2", "回复风格偏好", "用户偏好简洁回复", "用户反馈", 0.88f, "3天前", "第5条回复")
)

private fun sampleMedia() = listOf(
    MediaItem("med1", MediaType.Image, "https://example.com/1.png", null, "截图1", "今天 14:30"),
    MediaItem("med2", MediaType.File, "https://example.com/doc.pdf", null, "项目文档.pdf", "昨天", "1.2MB"),
    MediaItem("med3", MediaType.Voice, "https://example.com/voice.aac", null, "语音消息", "昨天", "15s"),
    MediaItem("med4", MediaType.Link, "https://example.com", null, "参考链接", "3天前")
)

private fun sampleMessageDetail(messageId: String) = MessageDetailData(
    message = com.amitia.core.model.MessageDto(id = messageId, role = "assistant", content = "这是一条测试消息，用于展示消息详情功能。", channel = "web", createdAt = "2026-07-27T14:30:00"),
    channel = "Web 对话", characterName = "艾米",
    referencedBy = listOf("后续追问消息1", "后续追问消息2"),
    relatedMemories = listOf("用户偏好", "回复风格"),
    toolExecutions = listOf(sampleToolExecution("t1"))
)

private fun sampleFile(fileId: String) = ChatFileInfo(
    id = fileId, name = "项目文档.pdf", mimeType = "application/pdf",
    sizeBytes = 1258291, url = "https://example.com/doc.pdf",
    uploadedAt = "今天 14:00", uploadStatus = FileUploadStatus.Uploaded, modelRead = true
)

private fun samplePromptTrace() = PromptTraceData(
    stages = listOf(
        PromptTraceStage("s1", "角色提示词", "注入角色设定", 500, "14:30:00", "[角色设定内容...]"),
        PromptTraceStage("s2", "记忆注入", "检索相关记忆", 800, "14:30:01", "[记忆内容...]"),
        PromptTraceStage("s3", "工具声明", "声明可用工具", 300, "14:30:01", "[工具列表...]"),
        PromptTraceStage("s4", "对话历史", "近期消息", 3200, "14:30:02", "[对话历史...]")
    ),
    totalTokens = 4800, recordedAt = "14:30:02"
)

private fun sampleSettings() = ConversationSettings(
    channel = "web", modelRoute = "默认路由", autoPlayVoice = false,
    mergeConsecutiveMessages = true, memoryWriteStrategy = MemoryWriteStrategy.Auto,
    notificationsEnabled = true
)
