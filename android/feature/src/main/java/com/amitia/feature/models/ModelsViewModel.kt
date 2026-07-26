package com.amitia.feature.models

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.ModelConfigDto
import com.amitia.core.model.ModelConfigUpdateRequest
import com.amitia.core.model.ModelDto
import com.amitia.core.repository.ModelRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ModelsViewModel @Inject constructor(
    private val modelRepository: ModelRepository
) : ViewModel() {

    private val _state = MutableStateFlow(ModelsUiState())
    val state: StateFlow<ModelsUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val config = modelRepository.getConfig()
                _state.value = _state.value.copy(
                    config = config,
                    models = config.models,
                    loading = false
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "加载模型配置失败"
                )
            }
        }
    }

    fun setCurrentModel(modelId: String) {
        viewModelScope.launch {
            runCatching {
                val updated = modelRepository.updateConfig(
                    ModelConfigUpdateRequest(currentModelId = modelId)
                )
                _state.value = _state.value.copy(config = updated)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "切换模型失败")
            }
        }
    }

    fun setEmbeddingModel(modelId: String) {
        viewModelScope.launch {
            runCatching {
                val updated = modelRepository.updateConfig(
                    ModelConfigUpdateRequest(currentEmbeddingModelId = modelId)
                )
                _state.value = _state.value.copy(config = updated)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "切换 Embedding 模型失败")
            }
        }
    }

    fun setTtsModel(modelId: String) {
        viewModelScope.launch {
            runCatching {
                val updated = modelRepository.updateConfig(
                    ModelConfigUpdateRequest(currentTtsModelId = modelId)
                )
                _state.value = _state.value.copy(config = updated)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "切换 TTS 模型失败")
            }
        }
    }

    fun setVisionModel(modelId: String) {
        viewModelScope.launch {
            runCatching {
                val updated = modelRepository.updateConfig(
                    ModelConfigUpdateRequest(currentVisionModelId = modelId)
                )
                _state.value = _state.value.copy(config = updated)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "切换视觉模型失败")
            }
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
    }
}

data class ModelsUiState(
    val config: ModelConfigDto? = null,
    val models: List<ModelDto> = emptyList(),
    val loading: Boolean = false,
    val error: String? = null
)
