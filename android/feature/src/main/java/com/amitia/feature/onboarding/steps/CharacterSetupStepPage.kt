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
                OnboardingTitle(text = "角色描述")
                OnboardingDescription(text = "为你的 AI 伙伴设定身份和性格。")
            }

            Spacer(modifier = Modifier.height(20.dp))

            SoftField(
                label = "角色名称",
                value = state.character.name,
                onValueChange = { onFieldChange("name", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "给TA起个名字"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "身份设定",
                value = state.character.identity,
                onValueChange = { onFieldChange("identity", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "如：学妹、助手、朋友"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "性格特点",
                value = state.character.personality,
                onValueChange = { onFieldChange("personality", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "如：温柔、活泼、理性"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "详细描述",
                value = state.character.description,
                onValueChange = { onFieldChange("description", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "描述角色的外貌、背景等",
                singleLine = false
            )
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = state.character.name.isNotBlank()
            )
        }
    }
}
