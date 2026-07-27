package com.amitia.feature.channel

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.designsystem.EmptyReason
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class ChannelHomeViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ChannelHomeData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ChannelHomeData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                delay(400)
                ChannelMockData.home
            }.onSuccess { data ->
                _state.value = if (data.systemChannels.isEmpty() && data.publicChannels.isEmpty()) {
                    ScreenState.Empty(EmptyReason.NoData)
                } else ScreenState.Content(data)
            }.onFailure { _state.value = ScreenState.Error(UiError(title = "加载渠道失败", message = it.message ?: "")) }
        }
    }
}

@HiltViewModel
class WebChannelViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<WebChannelConfig>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<WebChannelConfig>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(300); ChannelMockData.webConfig }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载 Web 渠道失败", message = it.message ?: "")) }
        }
    }

    fun update(transform: (WebChannelConfig) -> WebChannelConfig) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(transform(current))
    }
}

@HiltViewModel
class WeChatChannelViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<WeChatChannelDetail>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<WeChatChannelDetail>> = _state.asStateFlow()
    private val _reconnecting = MutableStateFlow(false)
    val reconnecting: StateFlow<Boolean> = _reconnecting.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(300); ChannelMockData.weChatDetail }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载微信渠道失败", message = it.message ?: "")) }
        }
    }

    fun reconnect() {
        viewModelScope.launch {
            _reconnecting.value = true
            delay(800)
            val current = (_state.value as? ScreenState.Content)?.data ?: run { _reconnecting.value = false; return@launch }
            _state.value = ScreenState.Content(current.copy(online = true, lastHeartbeat = "刚刚"))
            _reconnecting.value = false
        }
    }

    fun unbind(onDone: () -> Unit) {
        viewModelScope.launch { delay(300); onDone() }
    }
}

@HiltViewModel
class QQChannelViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<QQChannelDetail>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<QQChannelDetail>> = _state.asStateFlow()
    private val _reconnecting = MutableStateFlow(false)
    val reconnecting: StateFlow<Boolean> = _reconnecting.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(300); ChannelMockData.qqDetail }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载 QQ 渠道失败", message = it.message ?: "")) }
        }
    }

    fun reconnect() {
        viewModelScope.launch {
            _reconnecting.value = true
            delay(800)
            val current = (_state.value as? ScreenState.Content)?.data ?: run { _reconnecting.value = false; return@launch }
            _state.value = ScreenState.Content(current.copy(online = true, lastHeartbeat = "刚刚", messageLinkAbnormal = false, abnormalReason = null))
            _reconnecting.value = false
        }
    }

    fun unbind(onDone: () -> Unit) {
        viewModelScope.launch { delay(300); onDone() }
    }
}

@HiltViewModel
class ApiChannelViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ApiChannelConfig>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ApiChannelConfig>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(300); ChannelMockData.apiConfig }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载 API 渠道失败", message = it.message ?: "")) }
        }
    }

    fun update(transform: (ApiChannelConfig) -> ApiChannelConfig) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(transform(current))
    }
}

@HiltViewModel
class ChannelCreateViewModel @Inject constructor() : ViewModel() {

    private val _step = MutableStateFlow(0)
    val step: StateFlow<Int> = _step.asStateFlow()
    private val _selectedType = MutableStateFlow<ChannelType?>(null)
    val selectedType: StateFlow<ChannelType?> = _selectedType.asStateFlow()
    val steps = ChannelMockData.createSteps

    fun selectType(type: ChannelType) { _selectedType.value = type }
    fun next() { _step.value = (_step.value + 1).coerceAtMost(steps.lastIndex) }
    fun back() { _step.value = (_step.value - 1).coerceAtLeast(0) }
}

@HiltViewModel
class ChannelEditViewModel @Inject constructor() : ViewModel() {

    private val _form = MutableStateFlow(ChannelEditForm())
    val form: StateFlow<ChannelEditForm> = _form.asStateFlow()
    private val _saving = MutableStateFlow(false)
    val saving: StateFlow<Boolean> = _saving.asStateFlow()

    fun update(transform: (ChannelEditForm) -> ChannelEditForm) { _form.value = transform(_form.value) }

    fun save(onDone: () -> Unit) {
        viewModelScope.launch {
            _saving.value = true
            delay(500)
            _saving.value = false
            onDone()
        }
    }
}

@HiltViewModel
class ChannelBindViewModel @Inject constructor() : ViewModel() {

    private val _bindState = MutableStateFlow(ChannelBindState(false, 120, false, null, null))
    val bindState: StateFlow<ChannelBindState> = _bindState.asStateFlow()

    fun startScan() {
        viewModelScope.launch {
            _bindState.value = _bindState.value.copy(scanning = true, scanned = false, success = null, failReason = null, countdownSeconds = 120)
            tick()
        }
    }

    private fun tick() {
        viewModelScope.launch {
            while (_bindState.value.scanning && _bindState.value.countdownSeconds > 0) {
                delay(1000)
                _bindState.value = _bindState.value.copy(countdownSeconds = _bindState.value.countdownSeconds - 1)
            }
            if (_bindState.value.scanning && _bindState.value.countdownSeconds == 0) {
                _bindState.value = _bindState.value.copy(scanning = false, success = false, failReason = "二维码已过期，请刷新后重试")
            }
        }
    }

    fun markScanned(success: Boolean) {
        _bindState.value = _bindState.value.copy(scanning = false, scanned = true, success = success, failReason = if (success) null else "授权失败")
    }

    fun refresh() { startScan() }
}

@HiltViewModel
class ChannelDiagnosticsViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ChannelDiagnosticData>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ChannelDiagnosticData>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(400); ChannelMockData.diagnostics }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "诊断失败", message = it.message ?: "")) }
        }
    }
}

@HiltViewModel
class ChannelNotificationViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow<ScreenState<ChannelNotificationSettings>>(ScreenState.Loading)
    val state: StateFlow<ScreenState<ChannelNotificationSettings>> = _state.asStateFlow()

    init { load() }

    fun load() {
        _state.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching { delay(300); ChannelMockData.notificationSettings }
                .onSuccess { _state.value = ScreenState.Content(it) }
                .onFailure { _state.value = ScreenState.Error(UiError(title = "加载通知设置失败", message = it.message ?: "")) }
        }
    }

    fun update(transform: (ChannelNotificationSettings) -> ChannelNotificationSettings) {
        val current = (_state.value as? ScreenState.Content)?.data ?: return
        _state.value = ScreenState.Content(transform(current))
    }
}
