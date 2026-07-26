package com.amitia.feature.onboarding

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.network.connection.ConnectionManager
import com.amitia.core.network.connection.SessionManager
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.MemoryRepository
import com.amitia.core.repository.ModelRepository
import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.ModelConfigUpdateRequest
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.manager.RuntimeManager
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@HiltViewModel
class OnboardingViewModel @Inject constructor(
    private val endpointProvider: RuntimeEndpointProvider,
    private val runtimeManager: RuntimeManager,
    private val sessionManager: SessionManager,
    private val characterRepository: CharacterRepository,
    private val memoryRepository: MemoryRepository,
    private val modelRepository: ModelRepository,
    private val connectionManager: ConnectionManager,
    private val dataStore: OnboardingDataStore
) : ViewModel() {

    private val _state = MutableStateFlow(OnboardingUiState())
    val state: StateFlow<OnboardingUiState> = _state.asStateFlow()

    private val _currentStep = MutableStateFlow<OnboardingStep>(OnboardingStep.Welcome)
    val currentStep: StateFlow<OnboardingStep> = _currentStep.asStateFlow()

    init {
        observeRuntimeProgress()
        restoreIfInterrupted()
    }

    private fun restoreIfInterrupted() {
        viewModelScope.launch {
            val savedName = dataStore.currentStepName.first()
            if (!savedName.isNullOrBlank()) {
                val restored = runCatching {
                    OnboardingStepName.valueOf(savedName)
                }.getOrNull()?.let { fromName(it) }
                if (restored != null) {
                    _currentStep.value = restored
                }
            }
        }
    }

    private fun observeRuntimeProgress() {
        viewModelScope.launch {
            runtimeManager.observeState().collect { rs ->
                val installing = _currentStep.value is OnboardingStep.RuntimeInstall
                if (installing) {
                    val progress = rs.progress
                    val message = rs.readableMessage
                    _state.value = _state.value.copy(
                        runtimeProgress = progress,
                        runtimeMessage = message,
                        runtimeError = rs.error
                    )
                    if (rs is RuntimeState.Running) {
                        _state.value = _state.value.copy(
                            runtimeProgress = 1f,
                            runtimeMessage = "运行时已就绪"
                        )
                    } else if (rs is RuntimeState.Failed) {
                        _state.value = _state.value.copy(
                            runtimeError = rs.errorMessage
                        )
                    }
                }
            }
        }
    }

    fun selectMode(mode: OnboardingMode) {
        _state.value = _state.value.copy(selectedMode = mode)
        viewModelScope.launch {
            if (mode == OnboardingMode.REMOTE) {
                val snapshot = dataStore.remoteConfig.first()
                _state.value = _state.value.copy(
                    remoteBaseUrl = snapshot.baseUrl,
                    remoteToken = snapshot.authToken
                )
            }
        }
    }

    fun startRuntimeInstall() {
        viewModelScope.launch {
            _state.value = _state.value.copy(runtimeError = null, runtimeProgress = 0f)
            runCatching { runtimeManager.start() }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        runtimeError = e.message ?: "运行时安装失败"
                    )
                }
        }
    }

    fun configureRemote(baseUrl: String, token: String) {
        viewModelScope.launch {
            runCatching {
                endpointProvider.switchToRemote(baseUrl, token.ifBlank { null })
                dataStore.saveRemoteConfig(baseUrl, token, "remote")
            }.onSuccess {
                _state.value = _state.value.copy(
                    remoteBaseUrl = baseUrl,
                    remoteToken = token,
                    remoteConfigured = true
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "远程配置失败")
            }
        }
    }

    fun checkEnv() {
        viewModelScope.launch {
            _state.value = _state.value.copy(envChecking = true, envError = null)
            val result = connectionManager.testConnection()
            result
                .onSuccess { r ->
                    _state.value = _state.value.copy(
                        envChecking = false,
                        envConnected = r.success,
                        serverVersion = r.serverVersion,
                        latencyMs = r.latencyMs
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        envChecking = false,
                        envConnected = false,
                        envError = e.message ?: "环境检查失败"
                    )
                }
        }
    }

    fun authInit(token: String, userId: String?) {
        viewModelScope.launch {
            val expiresAt = System.currentTimeMillis() + 24L * 60 * 60 * 1000
            sessionManager.saveSession(token, expiresAt, userId)
            _state.value = _state.value.copy(authInitialized = true)
        }
    }

    fun configureModel(provider: String, modelName: String, apiKey: String, endpoint: String) {
        viewModelScope.launch {
            runCatching {
                dataStore.saveModelConfigSnapshot(
                    OnboardingDataStore.ModelConfigSnapshot(
                        provider = provider,
                        modelName = modelName,
                        apiKey = apiKey,
                        endpoint = endpoint
                    )
                )
                modelRepository.updateConfig(
                    ModelConfigUpdateRequest(currentModelId = modelName.ifBlank { null })
                )
            }.onSuccess {
                _state.value = _state.value.copy(
                    modelProvider = provider,
                    modelName = modelName
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "模型配置失败")
            }
        }
    }

    fun setupCharacter(name: String, personality: String, systemPrompt: String, greeting: String) {
        viewModelScope.launch {
            runCatching {
                characterRepository.create(
                    CharacterCreateRequest(
                        name = name,
                        personality = personality.ifBlank { null },
                        systemPrompt = systemPrompt.ifBlank { null },
                        greeting = greeting.ifBlank { null }
                    )
                )
            }.onSuccess { created ->
                characterRepository.switchCurrent(created.id)
                _state.value = _state.value.copy(characterId = created.id)
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "角色创建失败")
            }
        }
    }

    fun createInitialMemory(content: String, scope: String) {
        viewModelScope.launch {
            val characterId = _state.value.characterId ?: return@launch
            runCatching {
                memoryRepository.create(
                    MemoryCreateRequest(
                        content = content,
                        type = "initial",
                        scope = scope,
                        characterId = characterId,
                        importance = 1.0
                    )
                )
            }.onFailure { e ->
                _state.value = _state.value.copy(error = e.message ?: "初始记忆创建失败")
            }
        }
    }

    fun skipCurrent() {
        next()
    }

    fun next() {
        val nextStep = _currentStep.value.next()
        viewModelScope.launch {
            dataStore.saveStep(nextStep.name().toString())
            _currentStep.value = nextStep
            when (nextStep) {
                OnboardingStep.RuntimeInstall -> {
                    val current = runtimeManager.state.first()
                    if (current is RuntimeState.Running || current is RuntimeState.Installed) {
                        next()
                    } else {
                        startRuntimeInstall()
                    }
                }
                OnboardingStep.EnvCheck -> checkEnv()
                else -> Unit
            }
        }
    }

    fun previous() {
        val previousStep = _currentStep.value.previous()
        if (previousStep != null) {
            viewModelScope.launch {
                dataStore.saveStep(previousStep.name().toString())
                _currentStep.value = previousStep
            }
        }
    }

    fun complete(onFinished: () -> Unit) {
        viewModelScope.launch {
            dataStore.clear()
            onFinished()
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null, runtimeError = null)
    }

    private fun fromName(name: OnboardingStepName): OnboardingStep = when (name) {
        OnboardingStepName.WELCOME -> OnboardingStep.Welcome
        OnboardingStepName.MODE_SELECTION -> OnboardingStep.ModeSelection
        OnboardingStepName.RUNTIME_INSTALL -> OnboardingStep.RuntimeInstall
        OnboardingStepName.REMOTE_CONFIG -> OnboardingStep.RemoteConfig
        OnboardingStepName.ENV_CHECK -> OnboardingStep.EnvCheck
        OnboardingStepName.AUTH_INIT -> OnboardingStep.AuthInit
        OnboardingStepName.MODEL_CONFIG -> OnboardingStep.ModelConfig
        OnboardingStepName.CHARACTER_SETUP -> OnboardingStep.CharacterSetup
        OnboardingStepName.INITIAL_MEMORY -> OnboardingStep.InitialMemory
        OnboardingStepName.COMPLETE -> OnboardingStep.Complete
    }
}

enum class OnboardingMode { LOCAL, REMOTE }

enum class OnboardingStepName {
    WELCOME, MODE_SELECTION, RUNTIME_INSTALL, REMOTE_CONFIG,
    ENV_CHECK, AUTH_INIT, MODEL_CONFIG, CHARACTER_SETUP, INITIAL_MEMORY, COMPLETE
}

sealed interface OnboardingStep {

    data object Welcome : OnboardingStep
    data object ModeSelection : OnboardingStep
    data object RuntimeInstall : OnboardingStep
    data object RemoteConfig : OnboardingStep
    data object EnvCheck : OnboardingStep
    data object AuthInit : OnboardingStep
    data object ModelConfig : OnboardingStep
    data object CharacterSetup : OnboardingStep
    data object InitialMemory : OnboardingStep
    data object Complete : OnboardingStep

    fun next(): OnboardingStep = when (this) {
        Welcome -> ModeSelection
        ModeSelection -> RuntimeInstall
        RuntimeInstall -> RemoteConfig
        RemoteConfig -> EnvCheck
        EnvCheck -> AuthInit
        AuthInit -> ModelConfig
        ModelConfig -> CharacterSetup
        CharacterSetup -> InitialMemory
        InitialMemory -> Complete
        Complete -> Complete
    }

    fun previous(): OnboardingStep? = when (this) {
        Welcome -> null
        ModeSelection -> Welcome
        RuntimeInstall -> ModeSelection
        RemoteConfig -> RuntimeInstall
        EnvCheck -> RemoteConfig
        AuthInit -> EnvCheck
        ModelConfig -> AuthInit
        CharacterSetup -> ModelConfig
        InitialMemory -> CharacterSetup
        Complete -> InitialMemory
    }

    fun name(): OnboardingStepName = when (this) {
        Welcome -> OnboardingStepName.WELCOME
        ModeSelection -> OnboardingStepName.MODE_SELECTION
        RuntimeInstall -> OnboardingStepName.RUNTIME_INSTALL
        RemoteConfig -> OnboardingStepName.REMOTE_CONFIG
        EnvCheck -> OnboardingStepName.ENV_CHECK
        AuthInit -> OnboardingStepName.AUTH_INIT
        ModelConfig -> OnboardingStepName.MODEL_CONFIG
        CharacterSetup -> OnboardingStepName.CHARACTER_SETUP
        InitialMemory -> OnboardingStepName.INITIAL_MEMORY
        Complete -> OnboardingStepName.COMPLETE
    }

    fun index(): Int = when (this) {
        Welcome -> 0
        ModeSelection -> 1
        RuntimeInstall -> 2
        RemoteConfig -> 3
        EnvCheck -> 4
        AuthInit -> 5
        ModelConfig -> 6
        CharacterSetup -> 7
        InitialMemory -> 8
        Complete -> 9
    }
}

data class OnboardingUiState(
    val selectedMode: OnboardingMode? = null,
    val runtimeProgress: Float = 0f,
    val runtimeMessage: String = "",
    val runtimeError: String? = null,
    val remoteConfigured: Boolean = false,
    val remoteBaseUrl: String = "",
    val remoteToken: String = "",
    val envChecking: Boolean = false,
    val envConnected: Boolean = false,
    val envError: String? = null,
    val serverVersion: String? = null,
    val latencyMs: Long = 0L,
    val authInitialized: Boolean = false,
    val modelProvider: String = "",
    val modelName: String = "",
    val characterId: String? = null,
    val error: String? = null
)
