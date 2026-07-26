package com.amitia.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.AuthLoginRequest
import com.amitia.core.network.api.AuthApi
import com.amitia.core.network.client.AmitiaApiClient
import com.amitia.core.network.client.AmitiaApiException
import com.amitia.core.network.connection.ConnectionManager
import com.amitia.core.network.connection.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class AuthViewModel @Inject constructor(
    private val apiClient: AmitiaApiClient,
    private val sessionManager: SessionManager,
    private val connectionManager: ConnectionManager
) : ViewModel() {

    private val _state = MutableStateFlow(AuthUiState())
    val state: StateFlow<AuthUiState> = _state.asStateFlow()

    private val api: AuthApi by lazy { apiClient.service(AuthApi::class.java) }

    init {
        observeSession()
    }

    private fun observeSession() {
        viewModelScope.launch {
            sessionManager.session.collect { session ->
                _state.value = _state.value.copy(
                    isLoggedIn = session != null && !sessionManager.isExpired()
                )
            }
        }
    }

    fun login(username: String, password: String, token: String) {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val request = if (token.isNotBlank()) {
                    AuthLoginRequest(token = token.trim())
                } else {
                    AuthLoginRequest(
                        username = username.trim().ifBlank { null },
                        password = password.trim().ifBlank { null }
                    )
                }
                val response = api.login(request)
                val expiresAt = System.currentTimeMillis() +
                    (response.expiresIn?.times(1000) ?: DEFAULT_EXPIRES_MS)
                sessionManager.saveSession(
                    token = response.token,
                    expiresAt = expiresAt,
                    userId = response.userId ?: response.username
                )
                connectionManager.markConnected()
                _state.value = _state.value.copy(
                    loading = false,
                    isLoggedIn = true,
                    error = null
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = mapError(e)
                )
            }
        }
    }

    fun logout() {
        viewModelScope.launch {
            runCatching { api.logout("Bearer ${sessionManager.current()?.token}") }
            sessionManager.clearSession()
            connectionManager.markDisconnected()
            _state.value = AuthUiState()
        }
    }

    fun refreshToken() {
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null)
            runCatching {
                val current = sessionManager.current()
                if (current == null) {
                    _state.value = _state.value.copy(
                        loading = false,
                        error = "无活跃会话"
                    )
                    return@runCatching
                }
                val response = api.refresh("Bearer ${current.token}")
                val expiresAt = System.currentTimeMillis() +
                    (response.expiresIn?.times(1000) ?: DEFAULT_EXPIRES_MS)
                sessionManager.saveSession(
                    token = response.token,
                    expiresAt = expiresAt,
                    userId = response.userId ?: current.userId
                )
                _state.value = _state.value.copy(loading = false, error = null)
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = mapError(e)
                )
            }
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
    }

    private fun mapError(throwable: Throwable): String {
        return when (throwable) {
            is AmitiaApiException -> throwable.message ?: "登录失败"
            else -> throwable.message ?: "登录失败"
        }
    }

    companion object {
        private const val DEFAULT_EXPIRES_MS = 24L * 60 * 60 * 1000
    }
}

data class AuthUiState(
    val isLoggedIn: Boolean = false,
    val loading: Boolean = false,
    val error: String? = null
)
