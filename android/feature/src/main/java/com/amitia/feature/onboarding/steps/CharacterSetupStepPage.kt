package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun CharacterSetupStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.CharacterSetup)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "5 / 6")
                OnboardingTitle(text = "我应该是谁？")
                OnboardingDescription(text = "确定名字、身份和最基础的表达倾向。")
            }

            Spacer(modifier = Modifier.height(18.dp))

            SoftField(
                label = "角色描述",
                value = state.character.description,
                onValueChange = { onFieldChange("description", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "简要描述角色的身份、背景或与你的关系"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "角色提示词",
                value = state.character.prompt,
                onValueChange = { onFieldChange("prompt", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "输入角色的表达方式、行为规则、边界和需要长期遵循的要求",
                singleLine = false
            )
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "继续",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}
