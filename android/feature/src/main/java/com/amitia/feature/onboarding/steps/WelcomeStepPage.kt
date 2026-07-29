package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier

@Composable
fun WelcomeStepPage(
    onStart: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.Welcome)
        ) {
            RevealContent(delayMs = 120) {
                StepLabel(text = "欢迎使用 Amitia")
            }
            RevealContent(delayMs = 340) {
                OnboardingTitle(text = "先完成几项设置")
            }
            RevealContent(delayMs = 560) {
                OnboardingTitle(text = "再正式认识彼此")
            }
            RevealContent(delayMs = 800) {
                OnboardingDescription(text = "这里仅设置移动端运行所需的基础能力。角色、记忆和模型配置之后仍可随时修改。")
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            RevealContent(delayMs = 800) {
                PrimaryGlassButton(
                    text = "开始设置",
                    onClick = onStart,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}
