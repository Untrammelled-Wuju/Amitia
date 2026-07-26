package com.amitia.feature.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ProactiveMessageDto
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ChatRepository
import com.amitia.core.repository.ProactiveRepository
import com.amitia.platform.notification.ProactiveMessageObserver
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.manager.RuntimeManager
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val characterRepository: CharacterRepository,
    private val chatRepository: ChatRepository,
    private val proactiveRepository: ProactiveRepository,
    private val runtimeManager: RuntimeManager,
    private val proactiveMessageObserver: ProactiveMessageObserver
) : ViewModel() {

    private val _state = MutableStateFlow(HomeUiState())
    val state: StateFlow<HomeUiState> = _state.asStateFlow()

    init {
        observeRuntime()
        observeProactiveStream()
        refresh()
    }

    private fun observeProactiveStream() {
        viewModelScope.launch {
            proactiveMessageObserver.observeProactiveMessages().collectLatest { messages ->
                _state.value = _state.value.copy(proactiveMessages = messages)
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            _state.value = _state.value.copy(refreshing = true)
            loadCurrentCharacter()
            loadRecentConversations()
            loadProactiveMessages()
            _state.value = _state.value.copy(refreshing = false)
        }
    }

    private fun observeRuntime() {
        viewModelScope.launch {
            runtimeManager.observeState().collect { rs ->
                _state.value = _state.value.copy(runtimeState = rs)
            }
        }
    }

    private suspend fun loadCurrentCharacter() {
        runCatching { characterRepository.getCurrent() }
            .onSuccess { character ->
                _state.value = _state.value.copy(
                    currentCharacter = character,
                    characterStatus = if (character.isCurrent) "active" else "idle"
                )
            }
            .onFailure { e ->
                _state.value = _state.value.copy(
                    errors = _state.value.errors + (e.message ?: "加载当前角色失败")
                )
            }
    }

    private suspend fun loadRecentConversations() {
        runCatching { chatRepository.listConversations(page = 1, pageSize = 10) }
            .onSuccess { response ->
                _state.value = _state.value.copy(
                    recentConversations = response.items
                )
            }
            .onFailure { e ->
                _state.value = _state.value.copy(
                    errors = _state.value.errors + (e.message ?: "加载会话列表失败")
                )
            }
    }

    private suspend fun loadProactiveMessages() {
        runCatching { proactiveRepository.list(page = 1, pageSize = 5) }
            .onSuccess { response ->
                _state.value = _state.value.copy(proactiveMessages = response.items)
            }
            .onFailure { e ->
                _state.value = _state.value.copy(
                    errors = _state.value.errors + (e.message ?: "加载主动消息失败")
                )
            }
    }

    fun consumeError(index: Int) {
        _state.value = _state.value.copy(
            errors = _state.value.errors.toMutableList().apply {
                if (index in indices) removeAt(index)
            }
        )
    }
}

data class HomeUiState(
    val currentCharacter: CharacterDto? = null,
    val characterStatus: String? = null,
    val recentConversations: List<ConversationDto> = emptyList(),
    val proactiveMessages: List<ProactiveMessageDto> = emptyList(),
    val runtimeState: RuntimeState = RuntimeState.NotInstalled,
    val errors: List<String> = emptyList(),
    val refreshing: Boolean = false
)
