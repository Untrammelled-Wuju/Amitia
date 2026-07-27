package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme

@Composable
fun VisionModelStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onTest: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    ModelStepScaffold(
        title = "视觉模型设置",
        description = "配置图像理解模型，角色形象保持固定。",
        characterName = state.character.name.ifBlank { "Amitia" },
        onNext = onNext,
        nextEnabled = state.visionModel.tested,
        modifier = modifier
    ) {
        ModelConfigSection(
            model = state.visionModel,
            onFieldChange = { field, value -> onFieldChange(field, value) },
            onTest = onTest,
            providerLabel = "Provider",
            modelLabel = "视觉模型",
            testLabel = "图片理解测试",
            capabilitySummary = "支持图片输入 · 最大分辨率 2048x2048",
            icon = AmitiaIcons.Image
        )
        VisionFallbackNotice()
    }
}

@Composable
private fun VisionFallbackNotice() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = AmitiaStateColors.Degraded.copy(alpha = 0.08f)
    ) {
        Text(
            text = "若视觉模型不可用，将自动回退到纯文本模式",
            style = MaterialTheme.typography.labelSmall,
            color = AmitiaStateColors.Degraded,
            modifier = Modifier.padding(AmitiaSpacing.Base)
        )
    }
}

@Preview(name = "VisionModel - Light", showBackground = true)
@Composable
private fun VisionModelStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VisionModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                visionModel = ModelSetupState(provider = "OpenAI", model = "gpt-4o", tested = true)
            ),
            onFieldChange = { _, _ -> },
            onTest = {},
            onNext = {}
        )
    }
}

@Preview(name = "VisionModel - Dark", showBackground = true)
@Composable
private fun VisionModelStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VisionModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onFieldChange = { _, _ -> },
            onTest = {},
            onNext = {}
        )
    }
}
