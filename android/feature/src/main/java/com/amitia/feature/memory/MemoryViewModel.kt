package com.amitia.feature.memory

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.MemoryDto
import com.amitia.core.model.MemoryGraphDto
import com.amitia.core.model.MemorySearchRequest
import com.amitia.core.model.MemoryTimelineItem
import com.amitia.core.model.MemoryUpdateRequest
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.MemoryRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class MemoryViewModel @Inject constructor(
    private val memoryRepository: MemoryRepository,
    private val characterRepository: CharacterRepository
) : ViewModel() {

    private val _state = MutableStateFlow(MemoryUiState())
    val state: StateFlow<MemoryUiState> = _state.asStateFlow()

    init {
        listMemories()
        loadCharacters()
    }

    fun listMemories() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val memories = memoryRepository.list(
                    page = 1,
                    pageSize = 100,
                    characterId = _state.value.filterCharacterId,
                    type = _state.value.activeTypeFilter()
                )
                _state.value = _state.value.copy(
                    memories = memories,
                    loading = false
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "加载记忆失败"
                )
            }
        }
    }

    fun searchMemory(query: String) {
        viewModelScope.launch {
            _state.value = _state.value.copy(searchQuery = query, loading = true, error = null)
            if (query.isBlank()) {
                listMemories()
                return@launch
            }
            runCatching {
                val results = memoryRepository.search(
                    MemorySearchRequest(
                        query = query,
                        characterId = _state.value.filterCharacterId
                    )
                )
                _state.value = _state.value.copy(
                    memories = results,
                    loading = false
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "搜索失败"
                )
            }
        }
    }

    fun getTimeline() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { memoryRepository.getTimeline(limit = 100) }
                .onSuccess { timeline ->
                    _state.value = _state.value.copy(
                        timeline = timeline,
                        loading = false
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "加载时间线失败"
                    )
                }
        }
    }

    fun getGraph() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { memoryRepository.getGraph(_state.value.filterCharacterId, depth = 2) }
                .onSuccess { graph ->
                    _state.value = _state.value.copy(
                        graphSummary = graph,
                        loading = false
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "加载图谱失败"
                    )
                }
        }
    }

    fun switchTab(tab: MemoryTab) {
        _state.value = _state.value.copy(activeTab = tab)
        when (tab) {
            MemoryTab.LIST -> listMemories()
            MemoryTab.TIMELINE -> getTimeline()
            MemoryTab.GRAPH -> getGraph()
        }
    }

    fun filterByType(type: MemoryTypeFilter) {
        _state.value = _state.value.copy(typeFilter = type)
        listMemories()
    }

    fun filterByCharacter(characterId: String?) {
        _state.value = _state.value.copy(filterCharacterId = characterId)
        listMemories()
    }

    fun createMemory(request: MemoryCreateRequest, onCreated: (String) -> Unit) {
        viewModelScope.launch {
            runCatching {
                val effective = request.copy(
                    characterId = request.characterId ?: _state.value.filterCharacterId
                )
                memoryRepository.create(effective)
            }.onSuccess { created ->
                _state.value = _state.value.copy(
                    memories = listOf(created) + _state.value.memories
                )
                onCreated(created.id)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "创建记忆失败")
            }
        }
    }

    fun updateMemory(id: String, request: MemoryUpdateRequest, onUpdated: () -> Unit) {
        viewModelScope.launch {
            runCatching { memoryRepository.update(id, request) }
                .onSuccess { updated ->
                    _state.value = _state.value.copy(
                        memories = _state.value.memories.map { if (it.id == id) updated else it }
                    )
                    onUpdated()
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(error = e.message ?: "更新记忆失败")
                }
        }
    }

    fun deleteMemory(id: String) {
        viewModelScope.launch {
            runCatching { memoryRepository.delete(id) }
                .onSuccess {
                    _state.value = _state.value.copy(
                        memories = _state.value.memories.filterNot { it.id == id }
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(error = e.message ?: "删除失败")
                }
        }
    }

    private fun loadCharacters() {
        viewModelScope.launch {
            runCatching { characterRepository.list(page = 1, pageSize = 50) }
                .onSuccess { characters ->
                    _state.value = _state.value.copy(characters = characters)
                }
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
    }
}

enum class MemoryTab { LIST, TIMELINE, GRAPH }

enum class MemoryTypeFilter(val value: String?) {
    ALL(null),
    LONG_TERM("long_term"),
    EPISODIC("episodic"),
    INITIAL("initial"),
    WORLD_BOOK("world_book")
}

data class MemoryUiState(
    val memories: List<MemoryDto> = emptyList(),
    val timeline: List<MemoryTimelineItem> = emptyList(),
    val graphSummary: MemoryGraphDto? = null,
    val characters: List<com.amitia.core.model.CharacterDto> = emptyList(),
    val searchQuery: String = "",
    val filterCharacterId: String? = null,
    val typeFilter: MemoryTypeFilter = MemoryTypeFilter.ALL,
    val activeTab: MemoryTab = MemoryTab.LIST,
    val loading: Boolean = false,
    val error: String? = null
) {
    fun activeTypeFilter(): String? = typeFilter.value
}
