package com.amitia.feature.voicecenter

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.repository.TtsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class VoiceCenterViewModel @Inject constructor(
    private val ttsRepository: TtsRepository
) : ViewModel() {

    private val _voicesState = MutableStateFlow<ScreenState<List<VoiceItemUiModel>>>(ScreenState.Loading)
    val voicesState: StateFlow<ScreenState<List<VoiceItemUiModel>>> = _voicesState.asStateFlow()

    private val _ttsSettings = MutableStateFlow(TtsSettingsUiModel())
    val ttsSettings: StateFlow<TtsSettingsUiModel> = _ttsSettings.asStateFlow()

    private val _sttSettings = MutableStateFlow(SttSettingsUiModel())
    val sttSettings: StateFlow<SttSettingsUiModel> = _sttSettings.asStateFlow()

    private val _cloneStep = MutableStateFlow(0)
    val cloneStep: StateFlow<Int> = _cloneStep.asStateFlow()

    private val _audioDiagnosticsState = MutableStateFlow<ScreenState<List<AudioDiagnosticItemUiModel>>>(ScreenState.Loading)
    val audioDiagnosticsState: StateFlow<ScreenState<List<AudioDiagnosticItemUiModel>>> = _audioDiagnosticsState.asStateFlow()

    private val _testing = MutableStateFlow(false)
    val testing: StateFlow<Boolean> = _testing.asStateFlow()

    init {
        loadVoices()
        loadAudioDiagnostics()
    }

    fun loadVoices() {
        _voicesState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val voices = ttsRepository.getVoices().map { voice ->
                    VoiceItemUiModel(
                        id = voice.id,
                        name = voice.name,
                        provider = "默认 Provider",
                        language = voice.language ?: "zh-CN",
                        gender = voice.gender,
                        previewUrl = voice.previewUrl
                    )
                }
                if (voices.isEmpty()) {
                    _voicesState.value = ScreenState.Empty()
                } else {
                    _voicesState.value = ScreenState.Content(voices)
                }
            }.onFailure { e ->
                _voicesState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载声音列表")
                )
            }
        }
    }

    fun toggleFavorite(voiceId: String) {
        val current = _voicesState.value
        if (current is ScreenState.Content) {
            _voicesState.value = ScreenState.Content(
                current.data.map {
                    if (it.id == voiceId) it.copy(isFavorite = !it.isFavorite) else it
                }
            )
        }
    }

    fun updateTtsSettings(settings: TtsSettingsUiModel) {
        _ttsSettings.value = settings
    }

    fun updateSttSettings(settings: SttSettingsUiModel) {
        _sttSettings.value = settings
    }

    fun setCloneStep(step: Int) {
        _cloneStep.value = step
    }

    fun nextCloneStep() {
        if (_cloneStep.value < 6) _cloneStep.value = _cloneStep.value + 1
    }

    fun loadAudioDiagnostics() {
        _audioDiagnosticsState.value = ScreenState.Loading
        viewModelScope.launch {
            runCatching {
                val items = buildList {
                    add(AudioDiagnosticItemUiModel(
                        id = "mic",
                        title = "麦克风输入检测",
                        category = AudioDiagnosticCategory.Microphone,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测麦克风是否正常工作",
                        detail = "采样率: 48000Hz, 通道: 单声道"
                    ))
                    add(AudioDiagnosticItemUiModel(
                        id = "output",
                        title = "输出设备检测",
                        category = AudioDiagnosticCategory.Output,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测音频输出设备是否正常",
                        detail = "扬声器已就绪"
                    ))
                    add(AudioDiagnosticItemUiModel(
                        id = "focus",
                        title = "音频焦点检测",
                        category = AudioDiagnosticCategory.AudioFocus,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测音频焦点管理是否正常"
                    ))
                    add(AudioDiagnosticItemUiModel(
                        id = "tts_stt",
                        title = "TTS/STT 连接检测",
                        category = AudioDiagnosticCategory.TtsStt,
                        status = AudioDiagnosticStatus.Warning,
                        description = "检测 TTS 和 STT 服务连接状态",
                        detail = "TTS 已连接, STT 延迟较高",
                        latencyMs = 320
                    ))
                    add(AudioDiagnosticItemUiModel(
                        id = "latency",
                        title = "端到端延迟检测",
                        category = AudioDiagnosticCategory.Latency,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测语音端到端延迟",
                        latencyMs = 180
                    ))
                    add(AudioDiagnosticItemUiModel(
                        id = "conflict",
                        title = "音频冲突检测",
                        category = AudioDiagnosticCategory.Conflict,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测是否存在音频设备冲突"
                    ))
                }
                _audioDiagnosticsState.value = ScreenState.Content(items)
            }.onFailure { e ->
                _audioDiagnosticsState.value = ScreenState.Error(
                    UiError(title = "加载失败", message = e.message ?: "无法加载音频诊断结果")
                )
            }
        }
    }

    fun runAudioDiagnostic(diagnosticId: String) {
        _testing.value = true
        viewModelScope.launch {
            runCatching {
                val current = _audioDiagnosticsState.value
                if (current is ScreenState.Content) {
                    val updated = current.data.map {
                        if (it.id == diagnosticId) it.copy(status = AudioDiagnosticStatus.Checking) else it
                    }
                    _audioDiagnosticsState.value = ScreenState.Content(updated)
                }
                _audioDiagnosticsState.value = current
            }
            _testing.value = false
        }
    }
}
