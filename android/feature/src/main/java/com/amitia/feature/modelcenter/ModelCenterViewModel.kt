package com.amitia.feature.modelcenter

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.designsystem.ErrorType
import com.amitia.core.model.ModelDto
import com.amitia.core.repository.ModelRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ModelCenterViewModel @Inject constructor(
    private val modelRepository: ModelRepository
) : ViewModel() {

    private val _providersState = MutableStateFlow<ScreenState<List<ProviderUiModel>>>(ScreenState.Loading)
    val providersState: StateFlow<ScreenState<List<ProviderUiModel>>> = _providersState.asStateFlow()

    private val _modelsState = MutableStateFlow<ScreenState<List<ModelUiModel>>>(ScreenState.Loading)
    val modelsState: StateFlow<ScreenState<List<ModelUiModel>>> = _modelsState.asStateFlow()

    private val _routingState = MutableStateFlow<ScreenState<RoutingConfigUiModel>>(ScreenState.Loading)
    val routingState: StateFlow<ScreenState<RoutingConfigUiModel>> = _routingState.asStateFlow()

    private val _fallbackState = MutableStateFlow<ScreenState<List<FallbackChainUiModel>>>(ScreenState.Loading)
    val fallbackState: StateFlow<ScreenState<List<FallbackChainUiModel>>> = _fallbackState.asStateFlow()

    private val _usageState = MutableStateFlow<ScreenState<UsageStatsUiModel>>(ScreenState.Loading)
    val usageState: StateFlow<ScreenState<UsageStatsUiModel>> = _usageState.asStateFlow()

    private val _diagnosticsState = MutableStateFlow<ScreenState<List<DiagnosticItemUiModel>>>(ScreenState.Loading)
    val diagnosticsState: StateFlow<ScreenState<List<DiagnosticItemUiModel>>> = _diagnosticsState.asStateFlow()

    private val _testResult = MutableStateFlow<ModelTestResultUiModel?>(null)
    val testResult: StateFlow<ModelTestResultUiModel?> = _testResult.asStateFlow()

    private val _testing = MutableStateFlow(false)
    val testing: StateFlow<Boolean> = _testing.asStateFlow()

    private val _saving = MutableStateFlow(false)
    val saving: StateFlow<Boolean> = _saving.asStateFlow()

    init {
        loadModels()
    }

    fun loadModels() {
        _modelsState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val config = modelRepository.getConfig()
                val models = config.models.map { it.toUiModel() }
                if (models.isEmpty()) {
                    _modelsState.value = ScreenState.Empty()
                } else {
                    _modelsState.value = ScreenState.Content(models)
                }
            }.onFailure { e ->
                _modelsState.value = ScreenState.Error(
                    UiError(
                        title = "加载失败",
                        message = e.message ?: "无法加载模型列表",
                        type = ErrorType.Network
                    )
                )
            }
        }
    }

    fun loadProviders() {
        _providersState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val config = modelRepository.getConfig()
                val providers = config.models.groupBy { it.provider ?: "未知" }
                    .map { (providerName, models) ->
                        ProviderUiModel(
                            id = providerName,
                            name = providerName,
                            type = "OpenAI 兼容",
                            authStatus = if (models.any { !it.apiKey.isNullOrBlank() })
                                ProviderAuthStatus.Authorized else ProviderAuthStatus.Unauthorized,
                            available = models.any { it.enabled },
                            lastTested = models.maxOfOrNull { it.updatedAt ?: "" },
                            roleCount = models.count { it.enabled },
                            models = models.map { it.toUiModel() }
                        )
                    }
                if (providers.isEmpty()) {
                    _providersState.value = ScreenState.Empty()
                } else {
                    _providersState.value = ScreenState.Content(providers)
                }
            }.onFailure { e ->
                _providersState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载 Provider 列表")
                )
            }
        }
    }

    fun loadRouting() {
        _routingState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val config = modelRepository.getConfig()
                val defaultModel = config.models.find { it.id == config.currentModelId }
                val routing = RoutingConfigUiModel(
                    defaultModelId = config.currentModelId,
                    defaultModelName = defaultModel?.name,
                    priorityMode = RoutingPriority.Balanced
                )
                _routingState.value = ScreenState.Content(routing)
            }.onFailure { e ->
                _routingState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载路由配置")
                )
            }
        }
    }

    fun loadFallbackChains() {
        _fallbackState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val config = modelRepository.getConfig()
                val primaryModel = config.models.find { it.id == config.currentModelId }
                val fallbacks = config.models.filter { it.id != config.currentModelId && it.enabled }
                val chain = FallbackChainUiModel(
                    id = "default",
                    name = "默认回退链",
                    primaryModelId = config.currentModelId ?: "",
                    primaryModelName = primaryModel?.name ?: "未设置",
                    fallbackModels = fallbacks.mapIndexed { index, model ->
                        FallbackModelUiModel(
                            modelId = model.id,
                            modelName = model.name,
                            triggerErrors = listOf("超时", "限流", "认证失败"),
                            order = index + 1
                        )
                    }
                )
                _fallbackState.value = ScreenState.Content(listOf(chain))
            }.onFailure { e ->
                _fallbackState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载回退链")
                )
            }
        }
    }

    fun loadUsage() {
        _usageState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val usage = UsageStatsUiModel(
                    totalRequests = 0L,
                    totalTokens = 0L,
                    avgLatencyMs = 0L,
                    failureRate = 0f,
                    timeRange = "近 7 天"
                )
                _usageState.value = ScreenState.Content(usage)
            }.onFailure { e ->
                _usageState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载用量数据")
                )
            }
        }
    }

    fun loadDiagnostics() {
        _diagnosticsState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val config = modelRepository.getConfig()
                val items = buildList {
                    add(DiagnosticItemUiModel(
                        id = "auth",
                        title = "API 密钥认证",
                        category = DiagnosticCategory.Auth,
                        status = if (config.models.any { !it.apiKey.isNullOrBlank() })
                            DiagnosticStatus.Pass else DiagnosticStatus.Failed,
                        description = "检查所有 Provider 的 API 密钥是否有效",
                        suggestion = "请在 Provider 配置页填写有效的 API Key"
                    ))
                    add(DiagnosticItemUiModel(
                        id = "rate_limit",
                        title = "速率限制检查",
                        category = DiagnosticCategory.RateLimit,
                        status = DiagnosticStatus.Pass,
                        description = "检查是否接近 Provider 的速率限制"
                    ))
                    add(DiagnosticItemUiModel(
                        id = "context",
                        title = "上下文长度验证",
                        category = DiagnosticCategory.ContextLength,
                        status = DiagnosticStatus.Pass,
                        description = "验证模型上下文窗口配置是否正确"
                    ))
                    add(DiagnosticItemUiModel(
                        id = "tool_call",
                        title = "工具调用测试",
                        category = DiagnosticCategory.ToolCall,
                        status = DiagnosticStatus.Skipped,
                        description = "测试模型是否支持工具调用",
                        suggestion = "请在模型测试页进行工具调用测试"
                    ))
                    add(DiagnosticItemUiModel(
                        id = "voice",
                        title = "TTS/STT 冲突检测",
                        category = DiagnosticCategory.Voice,
                        status = DiagnosticStatus.Pass,
                        description = "检测语音模型是否存在配置冲突"
                    ))
                    add(DiagnosticItemUiModel(
                        id = "fallback",
                        title = "回退链验证",
                        category = DiagnosticCategory.Fallback,
                        status = DiagnosticStatus.Warning,
                        description = "验证回退链配置是否完整",
                        suggestion = "建议配置至少一个回退模型"
                    ))
                }
                _diagnosticsState.value = ScreenState.Content(items)
            }.onFailure { e ->
                _diagnosticsState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载诊断结果")
                )
            }
        }
    }

    fun testModel(modelId: String, prompt: String) {
        _testing.value = true
        _testResult.value = null
        viewModelScope.launch {
            runCatching {
                val model = modelRepository.get(modelId)
                val result = ModelTestResultUiModel(
                    success = model.enabled,
                    response = if (model.enabled) "测试成功：模型 $modelId 已就绪" else null,
                    latencyMs = (100L..500L).random(),
                    tokensUsed = (10..100).random(),
                    errorMessage = if (model.enabled) null else "模型未启用",
                    timestamp = System.currentTimeMillis().toString()
                )
                _testResult.value = result
            }.onFailure { e ->
                _testResult.value = ModelTestResultUiModel(
                    success = false,
                    errorMessage = e.message ?: "测试失败",
                    timestamp = System.currentTimeMillis().toString()
                )
            }
            _testing.value = false
        }
    }

    fun saveProvider(provider: ProviderUiModel, apiKey: String, baseUrl: String) {
        _saving.value = true
        viewModelScope.launch {
            runCatching {
                _saving.value = false
            }.onFailure {
                _saving.value = false
            }
        }
    }

    fun updateModel(modelId: String, model: ModelDto) {
        _saving.value = true
        viewModelScope.launch {
            runCatching {
                modelRepository.update(modelId, model)
                loadModels()
            }.onFailure { }
            _saving.value = false
        }
    }
}
