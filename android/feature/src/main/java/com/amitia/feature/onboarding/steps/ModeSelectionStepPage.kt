package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons

@Composable
fun ModeSelectionStepPage(
    selectedMode: OnboardingRunMode?,
    onSelect: (OnboardingRunMode) -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.ModeSelection)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "1 / 6")
                OnboardingTitle(text = "选择运行方式")
                OnboardingDescription(text = "你可以稍后在设置中切换，现在请选择一种启动方式。")
            }

            Spacer(modifier = Modifier.height(24.dp))

            ChoicePill(
                title = "本地运行",
                description = "数据优先保存在本机，需要更多存储和运行资源。",
                icon = AmitiaIcons.Storage,
                selected = selectedMode == OnboardingRunMode.Local,
                onSelect = { onSelect(OnboardingRunMode.Local) }
            )

            Spacer(modifier = Modifier.height(12.dp))

            ChoicePill(
                title = "远程连接",
                description = "连接已有 Amitia 服务端，需要服务地址或账号授权。",
                icon = AmitiaIcons.CloudDone,
                selected = selectedMode == OnboardingRunMode.Remote,
                onSelect = { onSelect(OnboardingRunMode.Remote) }
            )
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = selectedMode != null
            )
        }
    }
}
