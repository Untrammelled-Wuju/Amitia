package com.amitia.feature.character

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.CharacterUpdateRequest
import com.amitia.core.repository.CharacterRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class CharacterViewModel @Inject constructor(
    private val characterRepository: CharacterRepository
) : ViewModel() {

    private val _state = MutableStateFlow(CharacterUiState())
    val state: StateFlow<CharacterUiState> = _state.asStateFlow()

    private val _detailState = MutableStateFlow(CharacterDetailUiState())
    val detailState: StateFlow<CharacterDetailUiState> = _detailState.asStateFlow()

    init {
        listCharacters()
    }

    fun listCharacters() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val characters = characterRepository.list(page = 1, pageSize = 50)
                val currentId = characters.firstOrNull { it.isCurrent }?.id
                    ?: characterRepository.getCurrent().id
                _state.value = _state.value.copy(
                    characters = characters,
                    currentCharacterId = currentId,
                    loading = false
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "加载角色失败"
                )
            }
        }
    }

    fun loadDetail(characterId: String) {
        viewModelScope.launch {
            _detailState.value = _detailState.value.copy(loading = true, error = null)
            runCatching { characterRepository.get(characterId) }
                .onSuccess { character ->
                    _detailState.value = _detailState.value.copy(
                        character = character,
                        loading = false
                    )
                }
                .onFailure { e ->
                    _detailState.value = _detailState.value.copy(
                        loading = false,
                        error = e.message ?: "加载角色详情失败"
                    )
                }
        }
    }

    fun switchCharacter(id: String) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { characterRepository.switchCurrent(id) }
                .onSuccess {
                    _state.value = _state.value.copy(
                        currentCharacterId = id,
                        loading = false,
                        characters = _state.value.characters.map { c ->
                            c.copy(isCurrent = c.id == id)
                        }
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "切换角色失败"
                    )
                }
        }
    }

    fun createCharacter(request: CharacterCreateRequest, onCreated: (String) -> Unit) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { characterRepository.create(request) }
                .onSuccess { created ->
                    characterRepository.switchCurrent(created.id)
                    _state.value = _state.value.copy(
                        loading = false,
                        characters = _state.value.characters + created,
                        currentCharacterId = created.id
                    )
                    onCreated(created.id)
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "创建角色失败"
                    )
                }
        }
    }

    fun updateCharacter(id: String, request: CharacterUpdateRequest, onUpdated: () -> Unit) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { characterRepository.update(id, request) }
                .onSuccess { updated ->
                    _state.value = _state.value.copy(
                        loading = false,
                        characters = _state.value.characters.map { c ->
                            if (c.id == id) updated else c
                        }
                    )
                    _detailState.value = _detailState.value.copy(character = updated)
                    onUpdated()
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "更新角色失败"
                    )
                }
        }
    }

    fun confirmDelete(id: String) {
        _state.value = _state.value.copy(pendingDeleteId = id)
    }

    fun dismissDelete() {
        _state.value = _state.value.copy(pendingDeleteId = null)
    }

    fun deleteCharacter(id: String, onDeleted: () -> Unit) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null, pendingDeleteId = null)
            runCatching { characterRepository.delete(id) }
                .onSuccess {
                    _state.value = _state.value.copy(
                        loading = false,
                        characters = _state.value.characters.filterNot { it.id == id }
                    )
                    onDeleted()
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "删除角色失败"
                    )
                }
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
        _detailState.value = _detailState.value.copy(error = null)
    }
}

data class CharacterUiState(
    val characters: List<CharacterDto> = emptyList(),
    val currentCharacterId: String? = null,
    val loading: Boolean = false,
    val error: String? = null,
    val pendingDeleteId: String? = null
)

data class CharacterDetailUiState(
    val character: CharacterDto? = null,
    val loading: Boolean = false,
    val error: String? = null
)
