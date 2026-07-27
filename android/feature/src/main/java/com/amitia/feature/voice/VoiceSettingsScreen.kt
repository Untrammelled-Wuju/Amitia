package com.amitia.feature.voice

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SettingsRow

@Composable
fun VoiceSettingsScreen(
    onBack: () -> Unit,
    viewModel: VoiceViewModel = hiltViewModel()
) {
    val state by viewModel.settingsState.collectAsStateWithLifecycle()
    VoiceSettingsContent(
        state = state,
        onBack = onBack,
        onUpdate = viewModel::updateSettings
    )
}

@Composable
fun VoiceSettingsContent(
    state: VoiceSettingsUiState,
    onBack: () -> Unit,
    onUpdate: ((VoiceSettingsUiState) -> VoiceSettingsUiState) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "实时语音设置", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            AmitiaSection(title = "语音识别 (STT)") {
                Column {
                    SettingsRow(
                        title = "识别引擎",
                        subtitle = state.sttEngine,
                        leadingIcon = AmitiaIcons.GraphicEq,
                        onClick = {}
                    )
                    SettingsRow(
                        title = "识别语言",
                        subtitle = state.sttLanguage,
                        leadingIcon = AmitiaIcons.TextFields,
                        onClick = {}
                    )
                }
            }
            AmitiaSection(title = "语音合成 (TTS)") {
                Column {
                    SettingsRow(
                        title = "合成引擎",
                        subtitle = state.ttsEngine,
                        leadingIcon = AmitiaIcons.MusicNote,
                        onClick = {}
                    )
                    SettingsRow(
                        title = "语音音色",
                        subtitle = state.ttsVoice,
                        leadingIcon = AmitiaIcons.Person,
                        onClick = {}
                    )
                    AmitiaSlider(
                        value = state.speechSpeed,
                        onValueChange = { v -> onUpdate { it.copy(speechSpeed = v) } },
                        label = "语速",
                        valueFormatter = { "${(it * 0.5f + 0.5f * 100).toInt() / 100f}x" }
                    )
                }
            }
            AmitiaSection(title = "打断策略") {
                Column {
                    AmitiaSwitchRow(
                        title = "允许打断",
                        subtitle = "说话时角色可被打断",
                        checked = state.interruptionEnabled,
                        onCheckedChange = { v -> onUpdate { it.copy(interruptionEnabled = v) } },
                        leadingIcon = AmitiaIcons.Stop
                    )
                    AmitiaSlider(
                        value = state.interruptionSensitivity,
                        onValueChange = { v -> onUpdate { it.copy(interruptionSensitivity = v) } },
                        label = "灵敏度",
                        enabled = state.interruptionEnabled,
                        valueFormatter = { "${(it * 100).toInt()}%" }
                    )
                }
            }
            AmitiaSection(title = "静音检测 (VAD)") {
                Column {
                    AmitiaSwitchRow(
                        title = "启用静音检测",
                        subtitle = "自动判断说话结束",
                        checked = state.vadEnabled,
                        onCheckedChange = { v -> onUpdate { it.copy(vadEnabled = v) } },
                        leadingIcon = AmitiaIcons.Mic
                    )
                    AmitiaSlider(
                        value = state.vadThreshold,
                        onValueChange = { v -> onUpdate { it.copy(vadThreshold = v) } },
                        label = "阈值",
                        enabled = state.vadEnabled,
                        valueFormatter = { "${(it * 100).toInt()}%" }
                    )
                }
            }
            AmitiaSection(title = "字幕") {
                Column {
                    AmitiaSwitchRow(
                        title = "显示实时字幕",
                        subtitle = "通话时显示语音转写",
                        checked = state.captionEnabled,
                        onCheckedChange = { v -> onUpdate { it.copy(captionEnabled = v) } },
                        leadingIcon = AmitiaIcons.ChatBubble
                    )
                    AmitiaSlider(
                        value = state.captionFontSize,
                        onValueChange = { v -> onUpdate { it.copy(captionFontSize = v) } },
                        label = "字号",
                        enabled = state.captionEnabled,
                        valueFormatter = { "${(it * 100).toInt() + 80}%" }
                    )
                }
            }
            AmitiaSection(title = "音频设备优先级") {
                Column {
                    state.audioPriority.forEachIndexed { index, device ->
                        SettingsRow(
                            title = "${index + 1}. $device",
                            leadingIcon = AmitiaIcons.VolumeUp,
                            onClick = {}
                        )
                    }
                }
            }
            PrimaryButton(
                text = "保存设置",
                onClick = {},
                modifier = Modifier.padding(vertical = AmitiaSpacing.Sm)
            )
        }
    }
}

@Preview(name = "Voice Settings - Light", showBackground = true)
@Composable
private fun VoiceSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceSettingsContent(
            state = VoiceSettingsUiState(),
            onBack = {},
            onUpdate = {}
        )
    }
}

@Preview(name = "Voice Settings - Dark", showBackground = true)
@Composable
private fun VoiceSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceSettingsContent(
            state = VoiceSettingsUiState(),
            onBack = {},
            onUpdate = {}
        )
    }
}
