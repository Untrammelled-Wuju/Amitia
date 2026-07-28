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
                OnboardingTitle(text = "你的称呼")
                OnboardingDescription(text = "让 Amitia 知道如何称呼你，以及一些基本偏好。")
            }

            Spacer(modifier = Modifier.height(20.dp))

            SoftField(
                label = "你的昵称",
                value = state.memory.userNickname,
                onValueChange = { onFieldChange("nickname", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "你希望TA怎么叫你？"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "你们的关系",
                value = state.memory.relationship,
                onValueChange = { onFieldChange("relationship", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "如：朋友、师生、伴侣"
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "你的偏好",
                value = state.memory.preferences,
                onValueChange = { onFieldChange("preferences", it) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = "如：喜欢科幻、偏好简洁回复",
                singleLine = false
            )
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "完成设置",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = state.memory.userNickname.isNotBlank()
            )
        }
    }
}
