package com.amitia.feature.voice

import com.amitia.core.designsystem.component.AudioDeviceItem
import com.amitia.core.designsystem.component.VoiceCallStatus

data class VoiceCallUiState(
    val characterName: String = "艾米",
    val characterIdentity: String = "温柔知性助手",
    val status: VoiceCallStatus = VoiceCallStatus.Connecting,
    val durationSeconds: Int = 0,
    val isMuted: Boolean = false,
    val isSpeakerOn: Boolean = true,
    val isCaptionOn: Boolean = false,
    val micLevel: Float = 0f,
    val waveformProgress: Float = 0f,
    val isPlaying: Boolean = false,
    val secondaryStatus: VoiceSecondaryStatus = VoiceSecondaryStatus.Idle,
    val signalQuality: VoiceSignalQuality = VoiceSignalQuality.Good,
    val audioDevices: List<AudioDeviceItem> = emptyList(),
    val selectedDeviceId: String? = null,
    val showDevicePicker: Boolean = false,
    val showCaptionPanel: Boolean = false,
    val connectionFailed: Boolean = false
) {
    val durationText: String
        get() {
            val m = durationSeconds / 60
            val s = durationSeconds % 60
            return "%02d:%02d".format(m, s)
        }
}

enum class VoiceSecondaryStatus(val text: String) {
    Idle(""),
    Listening("正在听"),
    Thinking("正在思考"),
    Speaking("正在说话"),
    PoorNetwork("网络较差")
}

enum class VoiceSignalQuality(val label: String) {
    Good("信号良好"),
    Fair("信号一般"),
    Poor("信号较差")
}

data class VoiceCaptionItem(
    val id: String,
    val speaker: String,
    val text: String,
    val timestamp: String,
    val isUser: Boolean,
    val isUncertain: Boolean = false
)

data class VoiceCaptionUiState(
    val captions: List<VoiceCaptionItem> = emptyList(),
    val autoScroll: Boolean = true,
    val isLive: Boolean = true
)

data class VoiceCallHistoryItem(
    val id: String,
    val characterName: String,
    val characterIdentity: String,
    val time: String,
    val duration: String,
    val result: VoiceCallResult,
    val hasCaption: Boolean
)

enum class VoiceCallResult(val label: String) {
    Completed("已接通"),
    Missed("未接听"),
    Declined("已拒接"),
    Failed("连接失败")
}

data class VoiceHistoryUiState(
    val items: List<VoiceCallHistoryItem> = emptyList()
)

data class VoiceCallDetailUiState(
    val callId: String = "",
    val characterName: String = "",
    val time: String = "",
    val duration: String = "",
    val summary: String = "",
    val captions: List<VoiceCaptionItem> = emptyList(),
    val keyMemories: List<String> = emptyList(),
    val audioDiagnostics: String = "",
    val hasRecording: Boolean = false
)

data class VoiceSettingsUiState(
    val sttEngine: String = "Whisper",
    val sttLanguage: String = "中文",
    val ttsEngine: String = "Edge TTS",
    val ttsVoice: String = "温柔女声",
    val speechSpeed: Float = 0.5f,
    val interruptionEnabled: Boolean = true,
    val interruptionSensitivity: Float = 0.4f,
    val vadEnabled: Boolean = true,
    val vadThreshold: Float = 0.3f,
    val captionEnabled: Boolean = true,
    val captionFontSize: Float = 0.5f,
    val audioPriority: List<String> = listOf("蓝牙", "有线", "听筒", "扬声器")
)
