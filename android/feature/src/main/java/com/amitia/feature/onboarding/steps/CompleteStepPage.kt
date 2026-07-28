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
fun CompleteStepPage(
    onEnter: () -> Unit,
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
                OnboardingTitle(text = "一切就绪")
            }
            RevealContent(delayMs = 200) {
                OnboardingDescription(text = "Amitia 已经准备好陪伴你，点击下方按钮开始体验。")
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            RevealContent(delayMs = 400) {
                PrimaryGlassButton(
                    text = "进入 Amitia",
                    onClick = onEnter,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}
