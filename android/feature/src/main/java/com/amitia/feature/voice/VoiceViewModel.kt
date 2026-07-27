package com.amitia.feature.voice

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AudioDeviceItem
import com.amitia.core.designsystem.component.AudioDeviceType
import com.amitia.core.designsystem.component.VoiceCallStatus
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class VoiceViewModel @Inject constructor() : ViewModel() {

    private val _callState = MutableStateFlow(VoiceCallUiState())
    val callState: StateFlow<VoiceCallUiState> = _callState.asStateFlow()

    private val _captionState = MutableStateFlow<ScreenState<VoiceCaptionUiState>>(ScreenState.Loading)
    val captionState: StateFlow<ScreenState<VoiceCaptionUiState>> = _captionState.asStateFlow()

    private val _historyState = MutableStateFlow<ScreenState<VoiceHistoryUiState>>(ScreenState.Loading)
    val historyState: StateFlow<ScreenState<VoiceHistoryUiState>> = _historyState.asStateFlow()

    private val _detailState = MutableStateFlow<ScreenState<VoiceCallDetailUiState>>(ScreenState.Loading)
    val detailState: StateFlow<ScreenState<VoiceCallDetailUiState>> = _detailState.asStateFlow()

    private val _settingsState = MutableStateFlow(VoiceSettingsUiState())
    val settingsState: StateFlow<VoiceSettingsUiState> = _settingsState.asStateFlow()

    init {
        initCallState()
        loadCaptions()
        loadHistory()
        startCallTimer()
    }

    private fun initCallState() {
        _callState.value = VoiceCallUiState(
            status = VoiceCallStatus.Active,
            durationSeconds = 155,
            isMuted = false,
            isSpeakerOn = true,
            isCaptionOn = false,
            micLevel = 0.6f,
            waveformProgress = 0.5f,
            isPlaying = true,
            secondaryStatus = VoiceSecondaryStatus.Speaking,
            signalQuality = VoiceSignalQuality.Good,
            audioDevices = sampleDevices(),
            selectedDeviceId = "2"
        )
    }

    private fun sampleDevices() = listOf(
        AudioDeviceItem("1", "手机听筒", AudioDeviceType.Earpiece),
        AudioDeviceItem("2", "扬声器", AudioDeviceType.Speakerphone),
        AudioDeviceItem("3", "蓝牙耳机 AirPods Pro", AudioDeviceType.Bluetooth),
        AudioDeviceItem("4", "有线耳机", AudioDeviceType.WiredHeadset, isConnected = false)
    )

    private fun startCallTimer() {
        viewModelScope.launch {
            while (true) {
                delay(1000)
                val current = _callState.value
                if (current.status == VoiceCallStatus.Active) {
                    _callState.value = current.copy(durationSeconds = current.durationSeconds + 1)
                }
            }
        }
    }

    fun toggleMute() {
        _callState.value = _callState.value.copy(
            isMuted = !_callState.value.isMuted,
            status = if (!_callState.value.isMuted) VoiceCallStatus.Muted else VoiceCallStatus.Active
        )
    }

    fun toggleSpeaker() {
        _callState.value = _callState.value.copy(isSpeakerOn = !_callState.value.isSpeakerOn)
    }

    fun toggleCaption() {
        _callState.value = _callState.value.copy(isCaptionOn = !_callState.value.isCaptionOn)
    }

    fun showDevicePicker(show: Boolean) {
        _callState.value = _callState.value.copy(showDevicePicker = show)
    }

    fun selectDevice(device: AudioDeviceItem) {
        _callState.value = _callState.value.copy(selectedDeviceId = device.id)
    }

    fun retryConnection() {
        _callState.value = _callState.value.copy(
            connectionFailed = false,
            status = VoiceCallStatus.Connecting
        )
        viewModelScope.launch {
            delay(1500)
            _callState.value = _callState.value.copy(status = VoiceCallStatus.Active)
        }
    }

    fun switchToTextChat() {
        _callState.value = _callState.value.copy(status = VoiceCallStatus.Ended)
    }

    private fun loadCaptions() {
        viewModelScope.launch {
            _captionState.value = ScreenState.Loading
            delay(600)
            _captionState.value = ScreenState.Content(
                VoiceCaptionUiState(
                    captions = listOf(
                        VoiceCaptionItem("1", "艾米", "你好，今天感觉怎么样？", "14:30:01", isUser = false),
                        VoiceCaptionItem("2", "我", "还不错，刚忙完手头的事。", "14:30:08", isUser = true),
                        VoiceCaptionItem("3", "艾米", "那很好，记得放松一下。要不要聊聊今天发生的事？", "14:30:14", isUser = false),
                        VoiceCaptionItem("4", "我", "下午开会讨论了新方案，", "14:30:22", isUser = true, isUncertain = true),
                        VoiceCaptionItem("5", "我", "不过还没最终定下来。", "14:30:26", isUser = true),
                        VoiceCaptionItem("6", "艾米", "听起来进展顺利，需要我帮你梳理思路吗？", "14:30:33", isUser = false)
                    )
                )
            )
        }
    }

    private fun loadHistory() {
        viewModelScope.launch {
            _historyState.value = ScreenState.Loading
            delay(500)
            _historyState.value = ScreenState.Content(
                VoiceHistoryUiState(
                    items = listOf(
                        VoiceCallHistoryItem("1", "艾米", "温柔知性助手", "今天 14:30", "05:23", VoiceCallResult.Completed, true),
                        VoiceCallHistoryItem("2", "艾米", "温柔知性助手", "昨天 21:15", "12:08", VoiceCallResult.Completed, false),
                        VoiceCallHistoryItem("3", "艾米", "温柔知性助手", "昨天 09:42", "00:00", VoiceCallResult.Missed, false),
                        VoiceCallHistoryItem("4", "艾米", "温柔知性助手", "7月24日 20:30", "08:45", VoiceCallResult.Completed, true),
                        VoiceCallHistoryItem("5", "艾米", "温柔知性助手", "7月23日 18:12", "00:00", VoiceCallResult.Failed, false)
                    )
                )
            )
        }
    }

    fun loadDetail(callId: String) {
        viewModelScope.launch {
            _detailState.value = ScreenState.Loading
            delay(500)
            _detailState.value = ScreenState.Content(
                VoiceCallDetailUiState(
                    callId = callId,
                    characterName = "艾米",
                    time = "今天 14:30",
                    duration = "05:23",
                    summary = "本次通话围绕今日工作安排与情绪状态展开，用户分享了下午会议的进展，艾米提供了梳理建议。",
                    captions = listOf(
                        VoiceCaptionItem("1", "艾米", "你好，今天感觉怎么样？", "14:30:01", isUser = false),
                        VoiceCaptionItem("2", "我", "还不错，刚忙完手头的事。", "14:30:08", isUser = true)
                    ),
                    keyMemories = listOf(
                        "用户下午有会议讨论新方案",
                        "用户倾向于在忙碌后放松交流",
                        "艾米建议帮助梳理思路"
                    ),
                    audioDiagnostics = "平均延迟 180ms · 丢包率 0.2% · 采样率 16kHz",
                    hasRecording = true
                )
            )
        }
    }

    fun updateSettings(update: (VoiceSettingsUiState) -> VoiceSettingsUiState) {
        _settingsState.value = update(_settingsState.value)
    }
}
