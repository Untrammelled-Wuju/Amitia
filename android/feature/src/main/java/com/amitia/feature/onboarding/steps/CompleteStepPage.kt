package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun CompleteStepPage(
    onEnter: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.Complete)
        ) {
            Column(
                modifier = Modifier.fillMaxWidth(),
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                RevealContent(delayMs = 120) {
                    CompletionCheckIcon()
                }
                Spacer(modifier = Modifier.height(18.dp))
                RevealContent(delayMs = 340) {
                    StepLabel(text = "设置完成")
                }
                RevealContent(delayMs = 560) {
                    OnboardingTitle(text = "一切已经准备好了。")
                }
                RevealContent(delayMs = 800) {
                    OnboardingDescription(text = "点击下方开始使用")
                }
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            RevealContent(delayMs = 800) {
                PrimaryGlassButton(
                    text = "完成设置",
                    onClick = onEnter,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}
