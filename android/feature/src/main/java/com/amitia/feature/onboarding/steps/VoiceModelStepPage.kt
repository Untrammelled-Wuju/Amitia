package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun VoiceModelStepPage(
    state: OnboardingFlowUiState,
    onTtsFieldChange: (String, String) -> Unit,
    onSttFieldChange: (String, String) -> Unit,
    onTtsTest: () -> Unit,
    onSttTest: () -> Unit,
    onVoiceSelect: (String) -> Unit,
    onPreview: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    ModelStepScaffold(
        title = "语音模型设置",
        description = "配置语音合成与识别，角色形象保持固定。",
        characterName = state.character.name.ifBlank { "Amitia" },
        onNext = onNext,
        nextEnabled = state.voiceTts.tested && state.voiceStt.tested,
        modifier = modifier
    ) {
        Text(
            text = "TTS 语音合成",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        ModelConfigSection(
            model = state.voiceTts,
            onFieldChange = { field, value -> onTtsFieldChange(field, value) },
            onTest = onTtsTest,
            providerLabel = "TTS Provider",
            modelLabel = "TTS 模型",
            apiKeyLabel = "API Key",
            testLabel = "测试合成",
            icon = AmitiaIcons.VolumeUp
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        Text(
            text = "STT 语音识别",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        ModelConfigSection(
            model = state.voiceStt,
            onFieldChange = { field, value -> onSttFieldChange(field, value) },
            onTest = onSttTest,
            providerLabel = "STT Provider",
            modelLabel = "STT 模型",
            apiKeyLabel = "API Key",
            testLabel = "测试识别",
            icon = AmitiaIcons.Mic
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        VoiceSelectionSection(
            selected = state.voiceSelected,
            onSelect = onVoiceSelect,
            onPreview = onPreview
        )
    }
}

@Composable
private fun VoiceSelectionSection(
    selected: String,
    onSelect: (String) -> Unit,
    onPreview: () -> Unit
) {
    val voices = listOf("柔和女声", "沉稳男声", "清亮少女", "温和少年")
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "声音选择",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        voices.forEach { voice ->
            ChipFillOption(
                label = voice,
                selected = selected == voice,
                onSelect = { onSelect(voice) }
            )
        }
        TertiaryButton(
            text = "试听",
            onClick = onPreview,
            leadingIcon = AmitiaIcons.PlayArrow
        )
    }
}

@Preview(name = "VoiceModel - Light", showBackground = true)
@Composable
private fun VoiceModelStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                voiceTts = ModelSetupState(provider = "Azure", model = "tts-1", tested = true),
                voiceStt = ModelSetupState(provider = "Azure", model = "whisper-1", tested = true),
                voiceSelected = "柔和女声"
            ),
            onTtsFieldChange = { _, _ -> },
            onSttFieldChange = { _, _ -> },
            onTtsTest = {},
            onSttTest = {},
            onVoiceSelect = {},
            onPreview = {},
            onNext = {}
        )
    }
}

@Preview(name = "VoiceModel - Dark", showBackground = true)
@Composable
private fun VoiceModelStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onTtsFieldChange = { _, _ -> },
            onSttFieldChange = { _, _ -> },
            onTtsTest = {},
            onSttTest = {},
            onVoiceSelect = {},
            onPreview = {},
            onNext = {}
        )
    }
}
