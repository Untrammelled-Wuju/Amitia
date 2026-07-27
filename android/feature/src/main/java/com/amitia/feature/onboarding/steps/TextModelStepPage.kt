package com.amitia.feature.onboarding.steps

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaTheme

@Composable
fun TextModelStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onTest: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    ModelStepScaffold(
        title = "文本模型设置",
        description = "配置对话使用的语言模型，这是其他模型页面的布局基准。",
        characterName = state.character.name.ifBlank { "Amitia" },
        onNext = onNext,
        nextEnabled = state.textModel.tested,
        modifier = modifier
    ) {
        ModelConfigSection(
            model = state.textModel,
            onFieldChange = { field, value -> onFieldChange(field, value) },
            onTest = onTest,
            capabilitySummary = "上下文窗口 128K · 支持流式输出",
            icon = com.amitia.core.designsystem.AmitiaIcons.SmartToy
        )
    }
}

@Preview(name = "TextModel - Light", showBackground = true)
@Composable
private fun TextModelStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        TextModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                textModel = ModelSetupState(
                    provider = "OpenAI",
                    model = "gpt-4o",
                    tested = true
                )
            ),
            onFieldChange = { _, _ -> },
            onTest = {},
            onNext = {}
        )
    }
}

@Preview(name = "TextModel - Dark", showBackground = true)
@Composable
private fun TextModelStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        TextModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onFieldChange = { _, _ -> },
            onTest = {},
            onNext = {}
        )
    }
}
