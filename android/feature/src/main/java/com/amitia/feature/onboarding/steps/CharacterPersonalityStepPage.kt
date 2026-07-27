package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.PrimaryButton

private val personalityPresets = listOf(
    "温柔体贴",
    "活泼开朗",
    "冷静理性",
    "幽默风趣",
    "沉稳内敛",
    "热情大方"
)

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun CharacterPersonalityStepPage(
    state: OnboardingFlowUiState,
    onPersonalityChange: (String) -> Unit,
    onCustomChange: (String) -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    val hasSelection = state.character.personality.isNotBlank() || state.character.customPersonality.isNotBlank()
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            CharacterAvatar(name = state.character.name.ifBlank { "Amitia" }, size = 96)
            Text(
                text = "你希望我是什么性格？",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Text(
                text = "选择预设或自行描述，首次引导仅设置简化性格，完整性格可在角色详情中编辑。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaContentPadding.Horizontal),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                FlowRow(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    personalityPresets.forEach { preset ->
                        ChipFillOption(
                            label = preset,
                            selected = state.character.personality == preset,
                            onSelect = { onPersonalityChange(preset) }
                        )
                    }
                }
                AmitiaMultilineField(
                    value = state.character.customPersonality,
                    onValueChange = onCustomChange,
                    label = "自定义性格描述",
                    placeholder = "或用你自己的话描述角色的性格特点",
                    minLines = 3,
                    maxLines = 5,
                    charLimit = 200
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                PrimaryButton(
                    text = "下一步",
                    onClick = onNext,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = hasSelection,
                    leadingIcon = AmitiaIcons.ArrowForward
                )
            }
        }
    }
}

@Preview(name = "CharacterPersonality - Light", showBackground = true)
@Composable
private fun CharacterPersonalityStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterPersonalityStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米", personality = "温柔体贴")
            ),
            onPersonalityChange = {},
            onCustomChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "CharacterPersonality - Dark", showBackground = true)
@Composable
private fun CharacterPersonalityStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterPersonalityStepPage(
            state = OnboardingFlowUiState(),
            onPersonalityChange = {},
            onCustomChange = {},
            onNext = {}
        )
    }
}
