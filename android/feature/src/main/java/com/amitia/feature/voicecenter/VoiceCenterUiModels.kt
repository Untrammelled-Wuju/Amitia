package com.amitia.feature.voicecenter

data class VoiceItemUiModel(
    val id: String,
    val name: String,
    val provider: String,
    val language: String,
    val gender: String? = null,
    val style: String? = null,
    val previewUrl: String? = null,
    val isFavorite: Boolean = false,
    val usedByCharacters: List<String> = emptyList(),
    val customCloned: Boolean = false
)

data class VoiceCloneStepUiModel(
    val step: Int,
    val title: String,
    val description: String,
    val status: VoiceCloneStepStatus
)

enum class VoiceCloneStepStatus(val label: String) {
    Pending("待处理"),
    InProgress("进行中"),
    Completed("已完成"),
    Failed("失败")
}

data class TtsSettingsUiModel(
    val defaultProvider: String = "",
    val fallbackProvider: String? = null,
    val speed: Float = 1.0f,
    val pitch: Float = 1.0f,
    val emotion: String = "neutral",
    val autoPlay: Boolean = true,
    val volume: Float = 1.0f
)

data class SttSettingsUiModel(
    val defaultProvider: String = "",
    val language: String = "zh-CN",
    val autoPunctuation: Boolean = true,
    val silenceDetection: Boolean = true,
    val silenceThresholdMs: Int = 1500,
    val localFirst: Boolean = false
)

data class AudioDiagnosticItemUiModel(
    val id: String,
    val title: String,
    val category: AudioDiagnosticCategory,
    val status: AudioDiagnosticStatus,
    val description: String,
    val detail: String? = null,
    val latencyMs: Long? = null
)

enum class AudioDiagnosticCategory(val label: String) {
    Microphone("麦克风输入"),
    Output("输出设备"),
    AudioFocus("音频焦点"),
    TtsStt("TTS/STT 连接"),
    Latency("延迟"),
    Conflict("冲突检测")
}

enum class AudioDiagnosticStatus(val label: String) {
    Pass("正常"),
    Warning("警告"),
    Failed("异常"),
    Checking("检测中")
}
