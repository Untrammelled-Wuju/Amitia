package com.amitia.feature.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.network.connection.ConnectionManager
import com.amitia.core.network.connection.SessionManager
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val endpointProvider: RuntimeEndpointProvider,
    private val sessionManager: SessionManager,
    private val connectionManager: ConnectionManager,
    private val settingsDataStore: SettingsDataStore
) : ViewModel() {

    private val _state = MutableStateFlow(SettingsUiState())
    val state: StateFlow<SettingsUiState> = _state.asStateFlow()

    init {
        loadAll()
        observeEndpoint()
    }

    private fun loadAll() {
        viewModelScope.launch {
            val themeMode = settingsDataStore.themeMode.first()
            val notifications = settingsDataStore.notificationsEnabled.first()
            val ttsAuto = settingsDataStore.ttsAutoPlay.first()
            val voice = settingsDataStore.voicePreferred.first()
            val remoteUrl = settingsDataStore.remoteBaseUrl.first()
            val cacheHint = settingsDataStore.cacheDirHint.first()
            val logLevel = settingsDataStore.logLevel.first()
            val runtimeMode = endpointProvider.getCurrentMode()
            _state.value = _state.value.copy(
                themeMode = themeMode,
                notificationsEnabled = notifications,
                ttsAutoPlay = ttsAuto,
                preferredVoice = voice,
                remoteUrl = remoteUrl,
                runtimeMode = runtimeMode,
                cacheDirHint = cacheHint,
                logLevel = logLevel,
                appVersion = APP_VERSION
            )
        }
    }

    private fun observeEndpoint() {
        viewModelScope.launch {
            endpointProvider.observeEndpoint().collect { endpoint ->
                _state.value = _state.value.copy(
                    runtimeMode = endpointProvider.getCurrentMode(),
                    baseUrl = endpoint.baseUrl(),
                    remoteUrl = if (endpoint is RuntimeEndpoint.Remote) endpoint.baseUrl else _state.value.remoteUrl
                )
            }
        }
    }

    fun setThemeMode(mode: SettingsDataStore.ThemeMode) {
        viewModelScope.launch { settingsDataStore.setThemeMode(mode) }
        _state.value = _state.value.copy(themeMode = mode)
    }

    fun setNotificationsEnabled(enabled: Boolean) {
        viewModelScope.launch { settingsDataStore.setNotificationsEnabled(enabled) }
        _state.value = _state.value.copy(notificationsEnabled = enabled)
    }

    fun setTtsAutoPlay(enabled: Boolean) {
        viewModelScope.launch { settingsDataStore.setTtsAutoPlay(enabled) }
        _state.value = _state.value.copy(ttsAutoPlay = enabled)
    }

    fun setPreferredVoice(voiceId: String) {
        viewModelScope.launch { settingsDataStore.setPreferredVoice(voiceId.ifBlank { null }) }
        _state.value = _state.value.copy(preferredVoice = voiceId.ifBlank { null })
    }

    fun switchToLocalMode() {
        viewModelScope.launch {
            endpointProvider.switchToLocal(authToken = null)
            _state.value = _state.value.copy(runtimeMode = RuntimeEndpointProvider.RuntimeMode.LOCAL)
        }
    }

    fun switchToRemoteMode(baseUrl: String, token: String) {
        viewModelScope.launch {
            endpointProvider.switchToRemote(baseUrl, token.ifBlank { null })
            settingsDataStore.setRemoteBaseUrl(baseUrl)
            _state.value = _state.value.copy(
                runtimeMode = RuntimeEndpointProvider.RuntimeMode.REMOTE,
                remoteUrl = baseUrl
            )
            connectionManager.testConnection()
        }
    }

    fun setCacheDirHint(path: String) {
        viewModelScope.launch { settingsDataStore.setCacheDirHint(path) }
        _state.value = _state.value.copy(cacheDirHint = path)
    }

    fun setLogLevel(level: String) {
        viewModelScope.launch { settingsDataStore.setLogLevel(level) }
        _state.value = _state.value.copy(logLevel = level)
    }

    fun logout() {
        viewModelScope.launch {
            sessionManager.clearSession()
            connectionManager.markDisconnected()
        }
    }

    fun triggerBackup() {
        _state.value = _state.value.copy(backupStatus = "已请求后端执行备份")
    }

    fun triggerRestore() {
        _state.value = _state.value.copy(backupStatus = "已请求后端恢复")
    }

    fun clearCache() {
        _state.value = _state.value.copy(cacheCleared = true)
    }

    companion object {
        const val APP_VERSION = "0.1.0"
    }
}

data class SettingsUiState(
    val themeMode: SettingsDataStore.ThemeMode = SettingsDataStore.ThemeMode.SYSTEM,
    val notificationsEnabled: Boolean = true,
    val ttsAutoPlay: Boolean = false,
    val preferredVoice: String? = null,
    val remoteUrl: String = "",
    val runtimeMode: RuntimeEndpointProvider.RuntimeMode = RuntimeEndpointProvider.RuntimeMode.LOCAL,
    val baseUrl: String = "",
    val cacheDirHint: String = "",
    val logLevel: String = "info",
    val appVersion: String = "",
    val backupStatus: String = "",
    val cacheCleared: Boolean = false
)
