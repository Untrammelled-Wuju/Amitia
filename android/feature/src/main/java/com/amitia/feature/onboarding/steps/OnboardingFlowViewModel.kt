package com.amitia.feature.onboarding.steps

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

enum class OnboardingFlowStep(val index: Int) {
    Welcome(0),
    EnvCheck(1),
    ModeSelection(2),
    RuntimeInstall(3),
    RemoteConfig(4),
    AccountEntry(5),
    Register(6),
    Login(7),
    Permissions(8),
    ModelText(9),
    ModelVision(10),
    ModelVoice(11),
    ModelVector(12),
    CharacterAppearance(13),
    CharacterName(14),
    CharacterIdentity(15),
    CharacterPersonality(16),
    InitialMemory1(17),
    InitialMemory2(18),
    InitialMemory3(19),
    SetupSummary(20),
    CharacterComplete(21),
    EnterAmitia(22),
    DataImport(23);

    fun next(): OnboardingFlowStep? = entries.getOrNull(ordinal + 1)
    fun previous(): OnboardingFlowStep? = entries.getOrNull(ordinal - 1)
    val isEntry: Boolean get() = this == Welcome
}

enum class OnboardingRunMode { Local, Remote }

data class EnvCheckItem(
    val name: String,
    val passed: Boolean,
    val detail: String = "",
    val required: Boolean = true
)

data class RuntimeInstallItem(
    val name: String,
    val status: InstallStatus
)

enum class InstallStatus { Pending, Downloading, Verifying, Installing, Starting, Done, Failed }

data class ModelSetupState(
    val provider: String = "",
    val model: String = "",
    val apiKey: String = "",
    val tested: Boolean = false,
    val testing: Boolean = false,
    val failed: Boolean = false
)

data class CharacterSetupState(
    val appearance: String = "default",
    val name: String = "",
    val identity: String = "",
    val personality: String = "",
    val customPersonality: String = ""
)

data class InitialMemoryState(
    val userNickname: String = "",
    val relationship: String = "",
    val preferences: String = ""
)

data class OnboardingFlowUiState(
    val currentStep: OnboardingFlowStep = OnboardingFlowStep.Welcome,
    val transitioning: Boolean = false,
    val mode: OnboardingRunMode? = null,
    val envChecking: Boolean = false,
    val envItems: List<EnvCheckItem> = emptyList(),
    val runtimeInstalling: Boolean = false,
    val runtimeItems: List<RuntimeInstallItem> = emptyList(),
    val remoteAddress: String = "",
    val remotePort: String = "",
    val remoteHttpsVerified: Boolean = false,
    val remoteTesting: Boolean = false,
    val remoteConnected: Boolean = false,
    val remoteError: String? = null,
    val accountUsername: String = "",
    val accountEmail: String = "",
    val accountPassword: String = "",
    val accountConfirmPassword: String = "",
    val registerError: String? = null,
    val loginError: String? = null,
    val permissionNotifications: Boolean = false,
    val permissionMicrophone: Boolean = false,
    val permissionFiles: Boolean = false,
    val permissionAutoStart: Boolean = false,
    val textModel: ModelSetupState = ModelSetupState(),
    val visionModel: ModelSetupState = ModelSetupState(),
    val voiceTts: ModelSetupState = ModelSetupState(),
    val voiceStt: ModelSetupState = ModelSetupState(),
    val voiceSelected: String = "",
    val vectorModel: ModelSetupState = ModelSetupState(),
    val vectorDimension: String = "",
    val vectorQdrantConnected: Boolean = false,
    val character: CharacterSetupState = CharacterSetupState(),
    val memory: InitialMemoryState = InitialMemoryState(),
    val enterAnimationPlaying: Boolean = false,
    val error: String? = null
) {
    val allEnvRequiredPassed: Boolean
        get() = envItems.isNotEmpty() && envItems.filter { it.required }.all { it.passed }
}

@HiltViewModel
class OnboardingFlowViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow(OnboardingFlowUiState())
    val state: StateFlow<OnboardingFlowUiState> = _state.asStateFlow()

    fun goToStep(step: OnboardingFlowStep) {
        viewModelScope.launch {
            _state.value = _state.value.copy(transitioning = true)
            delay(220)
            _state.value = _state.value.copy(currentStep = step, transitioning = false)
        }
    }

    fun next() {
        _state.value.currentStep.next()?.let { goToStep(it) }
    }

    fun previous() {
        _state.value.currentStep.previous()?.let { goToStep(it) }
    }

    fun selectMode(mode: OnboardingRunMode) {
        _state.value = _state.value.copy(mode = mode)
    }

    fun checkEnvironment() {
        viewModelScope.launch {
            _state.value = _state.value.copy(envChecking = true, envItems = emptyList())
            val items = listOf(
                EnvCheckItem("Android 版本", true, "Android 13+"),
                EnvCheckItem("CPU 架构", true, "arm64-v8a"),
                EnvCheckItem("可用存储", true, "剩余 4.2 GB"),
                EnvCheckItem("内存", true, "6 GB"),
                EnvCheckItem("通知能力", true, "支持"),
                EnvCheckItem("麦克风能力", true, "可用"),
                EnvCheckItem("本地运行时", false, "未安装", required = false)
            )
            items.forEachIndexed { index, item ->
                delay(260)
                _state.value = _state.value.copy(
                    envItems = _state.value.envItems + items.subList(0, index + 1)
                )
            }
            _state.value = _state.value.copy(envChecking = false)
        }
    }

    fun startRuntimeInstall() {
        viewModelScope.launch {
            _state.value = _state.value.copy(runtimeInstalling = true, runtimeItems = emptyList())
            val names = listOf("内嵌 Linux 环境", "Amitia Go Backend", "Qdrant", "SurrealDB", "SQLite 数据目录")
            names.forEach { name ->
                val statuses = InstallStatus.entries.filter { it != InstallStatus.Pending }
                statuses.forEach { status ->
                    delay(180)
                    _state.value = _state.value.copy(
                        runtimeItems = updateRuntimeItem(_state.value.runtimeItems, name, status)
                    )
                }
            }
            _state.value = _state.value.copy(runtimeInstalling = false)
        }
    }

    private fun updateRuntimeItem(items: List<RuntimeInstallItem>, name: String, status: InstallStatus): List<RuntimeInstallItem> {
        val existing = items.indexOfFirst { it.name == name }
        return if (existing >= 0) {
            items.mapIndexed { i, item -> if (i == existing) item.copy(status = status) else item }
        } else {
            items + RuntimeInstallItem(name, status)
        }
    }

    fun updateRemoteAddress(value: String) {
        _state.value = _state.value.copy(remoteAddress = value, remoteError = null, remoteConnected = false)
    }

    fun updateRemotePort(value: String) {
        _state.value = _state.value.copy(remotePort = value)
    }

    fun testRemoteConnection() {
        viewModelScope.launch {
            _state.value = _state.value.copy(remoteTesting = true, remoteError = null)
            delay(900)
            val address = _state.value.remoteAddress
            if (address.isBlank() || !address.contains(".")) {
                _state.value = _state.value.copy(remoteTesting = false, remoteConnected = false, remoteError = "服务地址无效")
            } else {
                _state.value = _state.value.copy(
                    remoteTesting = false,
                    remoteConnected = true,
                    remoteHttpsVerified = address.startsWith("https")
                )
            }
        }
    }

    fun updateAccountField(field: String, value: String) {
        _state.value = when (field) {
            "username" -> _state.value.copy(accountUsername = value, registerError = null)
            "email" -> _state.value.copy(accountEmail = value, registerError = null)
            "password" -> _state.value.copy(accountPassword = value, registerError = null)
            "confirm" -> _state.value.copy(accountConfirmPassword = value, registerError = null)
            else -> _state.value
        }
    }

    fun submitRegister(): Boolean {
        val s = _state.value
        when {
            s.accountUsername.isBlank() -> {
                _state.value = s.copy(registerError = "请输入用户名")
                return false
            }
            s.accountEmail.isBlank() -> {
                _state.value = s.copy(registerError = "请输入邮箱")
                return false
            }
            s.accountPassword.length < 6 -> {
                _state.value = s.copy(registerError = "密码至少 6 位")
                return false
            }
            s.accountPassword != s.accountConfirmPassword -> {
                _state.value = s.copy(registerError = "两次密码不一致")
                return false
            }
        }
        return true
    }

    fun togglePermission(name: String) {
        _state.value = when (name) {
            "notifications" -> _state.value.copy(permissionNotifications = !_state.value.permissionNotifications)
            "microphone" -> _state.value.copy(permissionMicrophone = !_state.value.permissionMicrophone)
            "files" -> _state.value.copy(permissionFiles = !_state.value.permissionFiles)
            "autostart" -> _state.value.copy(permissionAutoStart = !_state.value.permissionAutoStart)
            else -> _state.value
        }
    }

    fun updateModelConfig(type: String, field: String, value: String) {
        val updater: (ModelSetupState) -> ModelSetupState = { it.copy(tested = false, failed = false) }
        _state.value = when (type) {
            "text" -> _state.value.copy(textModel = updateModelField(_state.value.textModel, field, value).let(updater))
            "vision" -> _state.value.copy(visionModel = updateModelField(_state.value.visionModel, field, value).let(updater))
            "tts" -> _state.value.copy(voiceTts = updateModelField(_state.value.voiceTts, field, value).let(updater))
            "stt" -> _state.value.copy(voiceStt = updateModelField(_state.value.voiceStt, field, value).let(updater))
            "vector" -> _state.value.copy(vectorModel = updateModelField(_state.value.vectorModel, field, value).let(updater))
            else -> _state.value
        }
    }

    private fun updateModelField(model: ModelSetupState, field: String, value: String): ModelSetupState {
        return when (field) {
            "provider" -> model.copy(provider = value)
            "model" -> model.copy(model = value)
            "apiKey" -> model.copy(apiKey = value)
            else -> model
        }
    }

    fun testModel(type: String) {
        viewModelScope.launch {
            setModelTesting(type, true)
            delay(900)
            val model = getModel(type)
            if (model.provider.isNotBlank() && model.model.isNotBlank()) {
                setModelTested(type, true)
            } else {
                setModelFailed(type, true)
            }
        }
    }

    private fun setModelTesting(type: String, testing: Boolean) {
        _state.value = when (type) {
            "text" -> _state.value.copy(textModel = _state.value.textModel.copy(testing = testing))
            "vision" -> _state.value.copy(visionModel = _state.value.visionModel.copy(testing = testing))
            "tts" -> _state.value.copy(voiceTts = _state.value.voiceTts.copy(testing = testing))
            "stt" -> _state.value.copy(voiceStt = _state.value.voiceStt.copy(testing = testing))
            "vector" -> _state.value.copy(vectorModel = _state.value.vectorModel.copy(testing = testing))
            else -> _state.value
        }
    }

    private fun setModelTested(type: String, tested: Boolean) {
        _state.value = when (type) {
            "text" -> _state.value.copy(textModel = _state.value.textModel.copy(testing = false, tested = tested, failed = false))
            "vision" -> _state.value.copy(visionModel = _state.value.visionModel.copy(testing = false, tested = tested, failed = false))
            "tts" -> _state.value.copy(voiceTts = _state.value.voiceTts.copy(testing = false, tested = tested, failed = false))
            "stt" -> _state.value.copy(voiceStt = _state.value.voiceStt.copy(testing = false, tested = tested, failed = false))
            "vector" -> _state.value.copy(vectorModel = _state.value.vectorModel.copy(testing = false, tested = tested, failed = false))
            else -> _state.value
        }
    }

    private fun setModelFailed(type: String, failed: Boolean) {
        _state.value = when (type) {
            "text" -> _state.value.copy(textModel = _state.value.textModel.copy(testing = false, failed = failed))
            "vision" -> _state.value.copy(visionModel = _state.value.visionModel.copy(testing = false, failed = failed))
            "tts" -> _state.value.copy(voiceTts = _state.value.voiceTts.copy(testing = false, failed = failed))
            "stt" -> _state.value.copy(voiceStt = _state.value.voiceStt.copy(testing = false, failed = failed))
            "vector" -> _state.value.copy(vectorModel = _state.value.vectorModel.copy(testing = false, failed = failed))
            else -> _state.value
        }
    }

    private fun getModel(type: String): ModelSetupState = when (type) {
        "text" -> _state.value.textModel
        "vision" -> _state.value.visionModel
        "tts" -> _state.value.voiceTts
        "stt" -> _state.value.voiceStt
        "vector" -> _state.value.vectorModel
        else -> ModelSetupState()
    }

    fun updateVoiceSelected(value: String) {
        _state.value = _state.value.copy(voiceSelected = value)
    }

    fun updateVectorDimension(value: String) {
        _state.value = _state.value.copy(vectorDimension = value)
    }

    fun testVectorQdrant() {
        viewModelScope.launch {
            delay(700)
            _state.value = _state.value.copy(vectorQdrantConnected = true)
        }
    }

    fun updateCharacter(field: String, value: String) {
        _state.value = when (field) {
            "appearance" -> _state.value.copy(character = _state.value.character.copy(appearance = value))
            "name" -> _state.value.copy(character = _state.value.character.copy(name = value))
            "identity" -> _state.value.copy(character = _state.value.character.copy(identity = value))
            "personality" -> _state.value.copy(character = _state.value.character.copy(personality = value))
            "customPersonality" -> _state.value.copy(character = _state.value.character.copy(customPersonality = value))
            else -> _state.value
        }
    }

    fun updateMemory(field: String, value: String) {
        _state.value = when (field) {
            "nickname" -> _state.value.copy(memory = _state.value.memory.copy(userNickname = value))
            "relationship" -> _state.value.copy(memory = _state.value.memory.copy(relationship = value))
            "preferences" -> _state.value.copy(memory = _state.value.memory.copy(preferences = value))
            else -> _state.value
        }
    }

    fun playEnterAnimation(onDone: () -> Unit) {
        viewModelScope.launch {
            _state.value = _state.value.copy(enterAnimationPlaying = true)
            delay(1200)
            _state.value = _state.value.copy(enterAnimationPlaying = false)
            onDone()
        }
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null, registerError = null, loginError = null, remoteError = null)
    }
}
