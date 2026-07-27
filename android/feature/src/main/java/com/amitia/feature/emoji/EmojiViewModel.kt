package com.amitia.feature.emoji

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class EmojiViewModel @Inject constructor() : ViewModel() {

    private val _centerState = MutableStateFlow<ScreenState<List<EmojiGroupItem>>>(ScreenState.Loading)
    val centerState: StateFlow<ScreenState<List<EmojiGroupItem>>> = _centerState.asStateFlow()

    private val _groupDetailState = MutableStateFlow<ScreenState<List<EmojiItem>>>(ScreenState.Loading)
    val groupDetailState: StateFlow<ScreenState<List<EmojiItem>>> = _groupDetailState.asStateFlow()

    private val _importState = MutableStateFlow<ScreenState<List<EmojiImportItem>>>(ScreenState.Loading)
    val importState: StateFlow<ScreenState<List<EmojiImportItem>>> = _importState.asStateFlow()

    private val _importResultState = MutableStateFlow<ScreenState<EmojiImportResult>>(ScreenState.Loading)
    val importResultState: StateFlow<ScreenState<EmojiImportResult>> = _importResultState.asStateFlow()

    private val _sendStrategyState = MutableStateFlow(EmojiSendStrategy())
    val sendStrategyState: StateFlow<EmojiSendStrategy> = _sendStrategyState.asStateFlow()

    private val _scopeState = MutableStateFlow(EmojiScopeConfig())
    val scopeState: StateFlow<EmojiScopeConfig> = _scopeState.asStateFlow()

    private val _pickerState = MutableStateFlow<ScreenState<List<EmojiItem>>>(ScreenState.Loading)
    val pickerState: StateFlow<ScreenState<List<EmojiItem>>> = _pickerState.asStateFlow()

    private val _characters = MutableStateFlow(sampleCharacters())
    val characters: StateFlow<List<CharacterOption>> = _characters.asStateFlow()

    init {
        loadCenter()
        loadGroupDetail("")
        loadPicker()
    }

    fun loadCenter() {
        viewModelScope.launch {
            _centerState.value = ScreenState.Loading
            delay(500)
            _centerState.value = ScreenState.Content(sampleGroups())
        }
    }

    fun loadGroupDetail(groupId: String) {
        viewModelScope.launch {
            _groupDetailState.value = ScreenState.Loading
            delay(400)
            _groupDetailState.value = ScreenState.Content(sampleEmojis(groupId))
        }
    }

    fun loadPicker() {
        viewModelScope.launch {
            _pickerState.value = ScreenState.Loading
            delay(300)
            _pickerState.value = ScreenState.Content(samplePickerEmojis())
        }
    }

    fun loadImportResult() {
        viewModelScope.launch {
            _importResultState.value = ScreenState.Loading
            delay(500)
            _importResultState.value = ScreenState.Content(sampleImportResult())
        }
    }

    fun updateSendStrategy(update: (EmojiSendStrategy) -> EmojiSendStrategy) {
        _sendStrategyState.value = update(_sendStrategyState.value)
    }

    fun updateScope(update: (EmojiScopeConfig) -> EmojiScopeConfig) {
        _scopeState.value = update(_scopeState.value)
    }

    fun toggleCharacterSelection(characterId: String) {
        _characters.value = _characters.value.map { c ->
            if (c.id == characterId) c.copy(selected = !c.selected) else c
        }
    }

    private fun sampleGroups(): List<EmojiGroupItem> = listOf(
        EmojiGroupItem("1", "日常表情", 24, null, "今天"),
        EmojiGroupItem("2", "情绪表达", 18, null, "昨天"),
        EmojiGroupItem("3", "节日特别", 8, null, "7月20日"),
        EmojiGroupItem("ungrouped", "未分组", 5, null, "7月15日", isUngrouped = true)
    )

    private fun sampleEmojis(groupId: String): List<EmojiItem> = listOf(
        EmojiItem("1", "", "开心", groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "今天"),
        EmojiItem("2", "", "惊讶", groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "今天"),
        EmojiItem("3", "", "思考", groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "昨天"),
        EmojiItem("4", "", null, groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "昨天", needsMeaning = true),
        EmojiItem("5", "", "无奈", groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "7月20日"),
        EmojiItem("6", "", "赞同", groupId = groupId.ifEmpty { "1" }, groupName = "日常表情", importedAt = "7月20日")
    )

    private fun samplePickerEmojis(): List<EmojiItem> = sampleEmojis("1").take(6)

    private fun sampleImportResult(): EmojiImportResult {
        val items = listOf(
            EmojiImportItem("1", "", EmojiImportStatus.Success, "开心"),
            EmojiImportItem("2", "", EmojiImportStatus.Success, "惊讶"),
            EmojiImportItem("3", "", EmojiImportStatus.Duplicate, duplicateOf = "1"),
            EmojiImportItem("4", "", EmojiImportStatus.NeedsMeaning),
            EmojiImportItem("5", "", EmojiImportStatus.Failed, errorMessage = "文件格式不支持"),
            EmojiImportItem("6", "", EmojiImportStatus.Success, "思考")
        )
        return EmojiImportResult(
            successCount = 3,
            duplicateCount = 1,
            failedCount = 1,
            needsMeaningCount = 1,
            items = items
        )
    }

    private fun sampleCharacters(): List<CharacterOption> = listOf(
        CharacterOption("1", "艾米", selected = true),
        CharacterOption("2", "露娜", selected = false),
        CharacterOption("3", "小薇", selected = false)
    )
}
