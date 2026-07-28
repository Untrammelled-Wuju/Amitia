package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun WelcomeStepPage(
    onStart: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            RevealContent(delayMs = 0) {
                OnboardingTitle(text = "Amitia")
            }
            RevealContent(delayMs = 200) {
                OnboardingDescription(text = "你的专属 AI 伙伴，已经在这里等你了。")
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            RevealContent(delayMs = 600) {
                PrimaryGlassButton(
                    text = "开始设置",
                    onClick = onStart,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}
