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
                OnboardingDescription(text = "手机既可以独立运行，也可以连接桌面端或已有服务。")
            }

            Spacer(modifier = Modifier.height(24.dp))

            ChoicePill(
                title = "本机运行",
                description = "在手机内启动运行环境，数据默认保存在当前设备。",
                icon = AmitiaIcons.Smartphone,
                selected = selectedMode == OnboardingRunMode.Local,
                onSelect = { onSelect(OnboardingRunMode.Local) }
            )

            Spacer(modifier = Modifier.height(10.dp))

            ChoicePill(
                title = "连接已有服务",
                description = "连接桌面端或服务器，手机作为移动使用端。",
                icon = AmitiaIcons.DesktopWindows,
                selected = selectedMode == OnboardingRunMode.Remote,
                onSelect = { onSelect(OnboardingRunMode.Remote) }
            )
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "继续",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = selectedMode != null
            )
        }
    }
}
