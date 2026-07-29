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
fun UserInfoStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.UserInfo)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "6 / 6")
                OnboardingTitle(text = "我应该怎么称呼你？")
                OnboardingDescription(text = "只填写角色从一开始就需要知道的信息。")
            }

            Spacer(modifier = Modifier.height(18.dp))

            SoftField(
                label = "对你的称呼",
                value = state.memory.userNickname,
                onValueChange = { onFieldChange("nickname", it) },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "你的身份",
                value = state.memory.userRole,
                onValueChange = { onFieldChange("userRole", it) },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "希望从一开始记住的事",
                value = state.memory.firstMemory,
                onValueChange = { onFieldChange("firstMemory", it) },
                modifier = Modifier.fillMaxWidth(),
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
