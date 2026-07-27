package com.amitia.feature.voicecenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun SttSettingsScreen(
    onBack: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val settings by viewModel.sttSettings.collectAsStateWithLifecycle()
    SttSettingsContent(
        settings = settings,
        onUpdate = viewModel::updateSttSettings,
        onBack = onBack
    )
}

@Composable
fun SttSettingsContent(
    settings: SttSettingsUiModel,
    onUpdate: (SttSettingsUiModel) -> Unit,
    onBack: () -> Unit
) {
    val providers = listOf("Azure STT", "OpenAI Whisper", "本地 Whisper", "Vosk")
    val languages = listOf(
        "zh-CN" to "简体中文",
        "zh-TW" to "繁体中文",
        "en-US" to "英语 (美国)",
        "ja-JP" to "日语",
        "ko-KR" to "韩语"
    )

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "STT 设置", onBack = onBack)
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
                            providers.forEach { provider ->
                                AmitiaSelectionRow(
                                    title = provider,
                                    selected = settings.defaultProvider == provider,
                                    onSelect = { onUpdate(settings.copy(defaultProvider = provider)) },
                                    leadingIcon = AmitiaIcons.Mic
                                )
                            }
                        }
                    }
                }
            }
            item(key = "language_section") {
                AmitiaSection(title = "识别语言") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            languages.forEach { (code, label) ->
                                AmitiaSelectionRow(
                                    title = label,
                                    subtitle = code,
                                    selected = settings.language == code,
                                    onSelect = { onUpdate(settings.copy(language = code)) }
                                )
                            }
                        }
                    }
                }
            }
            item(key = "punctuation_section") {
                AmitiaSection(title = "断句与标点") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            AmitiaSwitchRow(
                                title = "自动断句",
                                subtitle = "在语音停顿处自动添加标点符号",
                                checked = settings.autoPunctuation,
                                onCheckedChange = { onUpdate(settings.copy(autoPunctuation = it)) },
                                leadingIcon = AmitiaIcons.TextFields
                            )
                            AmitiaSwitchRow(
                                title = "静音检测",
                                subtitle = "检测到静音时自动结束当前语句",
                                checked = settings.silenceDetection,
                                onCheckedChange = { onUpdate(settings.copy(silenceDetection = it)) },
                                leadingIcon = AmitiaIcons.GraphicEq
                            )
                        }
                    }
                }
            }
            item(key = "silence_threshold") {
                if (settings.silenceDetection) {
                    AmitiaSection(title = "静音阈值") {
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(12.dp),
                            color = MaterialTheme.colorScheme.surface
                        ) {
                            Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                                AmitiaNumberField(
                                    value = settings.silenceThresholdMs.toString(),
                                    onValueChange = { value ->
                                        value.toIntOrNull()?.let { onUpdate(settings.copy(silenceThresholdMs = it)) }
                                    },
                                    label = "静音时长",
                                    placeholder = "1500",
                                    unit = "ms",
                                    onIncrement = { onUpdate(settings.copy(silenceThresholdMs = (settings.silenceThresholdMs + 100).coerceAtMost(5000))) },
                                    onDecrement = { onUpdate(settings.copy(silenceThresholdMs = (settings.silenceThresholdMs - 100).coerceAtLeast(500))) }
                                )
                                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                                Text(
                                    text = "检测到此时长静音后自动断句，范围 500-5000ms",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
            }
            item(key = "priority_section") {
                AmitiaSection(title = "处理优先") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(12.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Column {
                            AmitiaSelectionRow(
                                title = "本地优先",
                                subtitle = "优先使用本地模型，减少网络延迟",
                                selected = settings.localFirst,
                                onSelect = { onUpdate(settings.copy(localFirst = true)) },
                                leadingIcon = AmitiaIcons.Storage
                            )
                            AmitiaSelectionRow(
                                title = "远程优先",
                                subtitle = "优先使用云端模型，识别精度更高",
                                selected = !settings.localFirst,
                                onSelect = { onUpdate(settings.copy(localFirst = false)) },
                                leadingIcon = AmitiaIcons.CloudDownload
                            )
                        }
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

@Preview(name = "SttSettings - Light", showBackground = true)
@Composable
private fun SttSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SttSettingsContent(
            settings = SttSettingsUiModel(
                defaultProvider = "Azure STT",
                language = "zh-CN",
                autoPunctuation = true,
                silenceDetection = true,
                silenceThresholdMs = 1500,
                localFirst = false
            ),
            onUpdate = {},
            onBack = {}
        )
    }
}

@Preview(name = "SttSettings - Dark", showBackground = true)
@Composable
private fun SttSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SttSettingsContent(
            settings = SttSettingsUiModel(),
            onUpdate = {},
            onBack = {}
        )
    }
}
