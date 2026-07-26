package com.amitia.feature.startup

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.network.connection.ConnectionManager
import com.amitia.core.network.connection.SessionManager
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.runtime.manager.RuntimeManager
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@HiltViewModel
class StartupViewModel @Inject constructor(
    private val endpointProvider: RuntimeEndpointProvider,
    private val sessionManager: SessionManager,
    private val connectionManager: ConnectionManager,
    private val runtimeManager: RuntimeManager
) : ViewModel() {

    private val _state = MutableStateFlow(StartupUiState())
    val state: StateFlow<StartupUiState> = _state.asStateFlow()

    init {
        bootstrap()
    }

    private fun bootstrap() {
        viewModelScope.launch {
            _state.value = _state.value.copy(initializing = true, message = "正在初始化运行时端点")
            runCatching { endpointProvider.loadInitial() }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        initializing = false,
                        error = e.message ?: "端点加载失败"
                    )
                    return@launch
                }

            _state.value = _state.value.copy(message = "正在加载会话")
            runCatching { sessionManager.loadInitial() }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        initializing = false,
                        error = e.message ?: "会话加载失败"
                    )
                    return@launch
                }

            val mode = endpointProvider.currentEndpoint.value.let { endpoint ->
                if (endpoint is RuntimeEndpoint.Local) StartupMode.LOCAL
                else StartupMode.REMOTE
            }
            _state.value = _state.value.copy(mode = mode, message = "正在检测运行时状态")

            val runtimeState = runtimeManager.state.first()
            val isLoggedIn = sessionManager.current() != null && !sessionManager.isExpired()

            val target = when {
                runtimeState.isOperating -> StartupTarget.HOME
                runtimeState is com.amitia.runtime.api.RuntimeState.NotInstalled -> StartupTarget.ONBOARDING
                runtimeState is com.amitia.runtime.api.RuntimeState.Installed -> StartupTarget.HOME
                !isLoggedIn -> StartupTarget.AUTH
                else -> StartupTarget.HOME
            }

            _state.value = _state.value.copy(
                initializing = false,
                progress = 1f,
                target = target
            )
        }
    }

    fun retry() {
        _state.value = StartupUiState()
        bootstrap()
    }
}

enum class StartupMode { LOCAL, REMOTE }

enum class StartupTarget { HOME, ONBOARDING, AUTH }

data class StartupUiState(
    val initializing: Boolean = true,
    val progress: Float = 0f,
    val message: String = "",
    val error: String? = null,
    val mode: StartupMode = StartupMode.LOCAL,
    val target: StartupTarget? = null
)
