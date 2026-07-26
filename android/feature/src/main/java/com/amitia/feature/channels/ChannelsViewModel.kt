package com.amitia.feature.channels

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.ChannelDto
import com.amitia.core.model.ChannelStatusDto
import com.amitia.core.repository.ChannelRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ChannelsViewModel @Inject constructor(
    private val channelRepository: ChannelRepository
) : ViewModel() {

    private val _state = MutableStateFlow(ChannelsUiState())
    val state: StateFlow<ChannelsUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val status: ChannelStatusDto = channelRepository.getStatus()
                _state.value = _state.value.copy(
                    channels = status.channels,
                    status = status,
                    loading = false
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "加载渠道失败"
                )
            }
        }
    }

    fun bind(channelType: String, config: Map<String, String>) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { channelRepository.bind(channelType, config) }
                .onSuccess { bound ->
                    _state.value = _state.value.copy(
                        channels = _state.value.channels.map {
                            if (it.type == bound.type) bound else it
                        },
                        loading = false
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "绑定渠道失败"
                    )
                }
        }
    }

    fun unbind(channelType: String, config: Map<String, String>) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching { channelRepository.unbind(channelType, config) }
                .onSuccess { unbound ->
                    _state.value = _state.value.copy(
                        channels = _state.value.channels.map {
                            if (it.type == unbound.type) unbound else it
                        },
                        loading = false
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "解绑渠道失败"
                    )
                }
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
    }
}

data class ChannelsUiState(
    val channels: List<ChannelDto> = emptyList(),
    val status: ChannelStatusDto? = null,
    val loading: Boolean = false,
    val error: String? = null
)
