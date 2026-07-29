package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun EnvironmentStepPage(
    state: OnboardingFlowUiState,
    onCheck: () -> Unit,
    onInstall: () -> Unit,
    onAddressChange: (String) -> Unit,
    onPortChange: (String) -> Unit,
    onTestConnection: () -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    LaunchedEffect(state.currentStep) {
        if (state.envItems.isEmpty() && !state.envChecking) {
            onCheck()
        }
    }

    Box(modifier = modifier) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.Environment)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "2 / 6")
                OnboardingTitle(text = "准备运行环境")
                OnboardingDescription(text = "Amitia 正在完成必要检查，完成后即可继续。")
            }

            Spacer(modifier = Modifier.height(24.dp))

            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                state.envItems.forEachIndexed { index, item ->
                    val firstNotPassed = state.envItems.indexOfFirst { !it.passed }
                    val rowState = when {
                        item.passed -> ProcessState.Done
                        state.envChecking && index == firstNotPassed -> ProcessState.Running
                        else -> ProcessState.Pending
                    }
                    ProcessRowItem(
                        title = item.name,
                        description = item.detail,
                        state = rowState
                    )
                }
            }

            if (state.allEnvRequiredPassed) {
                when (state.mode) {
                    OnboardingRunMode.Local -> {
                        LaunchedEffect(state.allEnvRequiredPassed, state.runtimeItems.isEmpty(), state.runtimeInstalling) {
                            if (state.runtimeItems.isEmpty() && !state.runtimeInstalling) {
                                onInstall()
                            }
                        }

                        if (state.runtimeItems.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(10.dp))
                            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                                state.runtimeItems.forEach { item ->
                                    ProcessRowItem(
                                        title = item.name,
                                        description = installStatusText(item.status),
                                        state = installStatusToProcessState(item.status)
                                    )
                                }
                            }
                        }
                    }

                    OnboardingRunMode.Remote -> {
                        Spacer(modifier = Modifier.height(16.dp))
                        SoftField(
                            label = "服务地址",
                            value = state.remoteAddress,
                            onValueChange = onAddressChange,
                            modifier = Modifier.fillMaxWidth(),
                            placeholder = "https://example.com"
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        SoftField(
                            label = "端口",
                            value = state.remotePort,
                            onValueChange = onPortChange,
                            modifier = Modifier.fillMaxWidth(),
                            placeholder = "8443"
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        PrimaryGlassButton(
                            text = "测试连接",
                            onClick = onTestConnection,
                            modifier = Modifier.fillMaxWidth()
                        )
                        if (state.remoteConnected) {
                            Spacer(modifier = Modifier.height(12.dp))
                            Text(
                                text = "连接成功",
                                color = Color(0xFF5E836F),
                                fontSize = 13.sp
                            )
                        }
                    }

                    null -> {}
                }
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            val buttonText: String
            val buttonEnabled: Boolean

            when {
                state.envChecking -> {
                    buttonText = "正在准备"
                    buttonEnabled = false
                }
                !state.allEnvRequiredPassed -> {
                    buttonText = "正在准备"
                    buttonEnabled = false
                }
                state.mode == OnboardingRunMode.Local -> {
                    val allDone = state.runtimeItems.isNotEmpty() &&
                        state.runtimeItems.all { it.status == InstallStatus.Done }
                    buttonText = if (allDone) "继续" else "正在安装"
                    buttonEnabled = allDone
                }
                state.mode == OnboardingRunMode.Remote -> {
                    buttonText = "继续"
                    buttonEnabled = state.remoteConnected
                }
                else -> {
                    buttonText = "正在准备"
                    buttonEnabled = false
                }
            }

            PrimaryGlassButton(
                text = buttonText,
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = buttonEnabled
            )
        }
    }
}

private fun installStatusToProcessState(status: InstallStatus): ProcessState = when (status) {
    InstallStatus.Pending -> ProcessState.Pending
    InstallStatus.Downloading -> ProcessState.Running
    InstallStatus.Verifying -> ProcessState.Running
    InstallStatus.Installing -> ProcessState.Running
    InstallStatus.Starting -> ProcessState.Running
    InstallStatus.Done -> ProcessState.Done
    InstallStatus.Failed -> ProcessState.Pending
}

private fun installStatusText(status: InstallStatus): String = when (status) {
    InstallStatus.Pending -> "等待中"
    InstallStatus.Downloading -> "下载中"
    InstallStatus.Verifying -> "校验中"
    InstallStatus.Installing -> "安装中"
    InstallStatus.Starting -> "启动中"
    InstallStatus.Done -> "已完成"
    InstallStatus.Failed -> "安装失败"
}
