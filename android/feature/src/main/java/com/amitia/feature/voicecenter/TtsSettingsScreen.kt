package com.amitia.feature.voicecenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun TtsSettingsScreen(
    onBack: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val settings by viewModel.ttsSettings.collectAsStateWithLifecycle()
    TtsSettingsContent(
        settings = settings,
        onUpdate = viewModel::updateTtsSettings,
        onBack = onBack
    )
}

@Composable
fun TtsSettingsContent(
    settings: TtsSettingsUiModel,
    onUpdate: (TtsSettingsUiModel) -> Unit,
    onBack: () -> Unit
) {
    val providers = listOf("Azure TTS", "OpenAI TTS", "Edge TTS", "本地 TTS")
    val emotions = listOf("neutral", "happy", "sad", "angry", "calm", "fearful")

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "TTS 设置", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "provider_section") {
                AmitiaSection(title = "默认 Provider") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            providers.forEachIndexed { index, provider ->
                                AmitiaSelectionRow(
                                    title = provider,
                                    selected = settings.defaultProvider == provider,
                                    onSelect = { onUpdate(settings.copy(defaultProvider = provider)) },
                                    leadingIcon = AmitiaIcons.GraphicEq
                                )
                            }
                        }
                    }
                }
            }
            item(key = "fallback_section") {
                AmitiaSection(title = "失败回退") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            AmitiaSelectionRow(
                                title = "无回退",
                                subtitle = "主 Provider 失败时不使用 TTS",
                                selected = settings.fallbackProvider == null,
                                onSelect = { onUpdate(settings.copy(fallbackProvider = null)) }
                            )
                            providers.filter { it != settings.defaultProvider }.forEach { provider ->
                                AmitiaSelectionRow(
                                    title = provider,
                                    subtitle = "主 Provider 失败时自动切换",
                                    selected = settings.fallbackProvider == provider,
                                    onSelect = { onUpdate(settings.copy(fallbackProvider = provider)) }
                                )
                            }
                        }
                    }
                }
            }
            item(key = "params_section") {
                AmitiaSection(title = "语音参数") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            AmitiaSlider(
                                value = settings.speed,
                                onValueChange = { onUpdate(settings.copy(speed = it)) },
                                valueRange = 0.5f..2.0f,
                                steps = 14,
                                label = "语速",
                                valueFormatter = { "${String.format("%.1f", it)}x" }
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
                            AmitiaSlider(
                                value = settings.pitch,
                                onValueChange = { onUpdate(settings.copy(pitch = it)) },
                                valueRange = 0.5f..2.0f,
                                steps = 14,
                                label = "音调",
                                valueFormatter = { "${String.format("%.1f", it)}x" }
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
                            AmitiaSlider(
                                value = settings.volume,
                                onValueChange = { onUpdate(settings.copy(volume = it)) },
                                valueRange = 0.0f..1.0f,
                                label = "音量",
                                valueFormatter = { "${(it * 100).toInt()}%" }
                            )
                        }
                    }
                }
            }
            item(key = "emotion_section") {
                AmitiaSection(title = "情感") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            emotions.forEach { emotion ->
                                AmitiaSelectionRow(
                                    title = emotionLabel(emotion),
                                    selected = settings.emotion == emotion,
                                    onSelect = { onUpdate(settings.copy(emotion = emotion)) }
                                )
                            }
                        }
                    }
                }
            }
            item(key = "playback_section") {
                AmitiaSection(title = "播放设置") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        AmitiaSwitchRow(
                            title = "自动播放",
                            subtitle = "消息到达后自动合成并播放语音",
                            checked = settings.autoPlay,
                            onCheckedChange = { onUpdate(settings.copy(autoPlay = it)) },
                            leadingIcon = AmitiaIcons.PlayArrow
                        )
                    }
                }
            }
            item(key = "save") {
                PrimaryButton(
                    text = "保存设置",
                    onClick = {},
                    leadingIcon = AmitiaIcons.Check,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}

private fun emotionLabel(emotion: String): String = when (emotion) {
    "neutral" -> "自然"
    "happy" -> "开心"
    "sad" -> "悲伤"
    "angry" -> "愤怒"
    "calm" -> "平静"
    "fearful" -> "恐惧"
    else -> emotion
}

@Preview(name = "TtsSettings - Light", showBackground = true)
@Composable
private fun TtsSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        TtsSettingsContent(
            settings = TtsSettingsUiModel(
                defaultProvider = "Azure TTS",
                fallbackProvider = "OpenAI TTS",
                speed = 1.0f,
                pitch = 1.0f,
                emotion = "neutral",
                autoPlay = true,
                volume = 1.0f
            ),
            onUpdate = {},
            onBack = {}
        )
    }
}

@Preview(name = "TtsSettings - Dark", showBackground = true)
@Composable
private fun TtsSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        TtsSettingsContent(
            settings = TtsSettingsUiModel(),
            onUpdate = {},
            onBack = {}
        )
    }
}
