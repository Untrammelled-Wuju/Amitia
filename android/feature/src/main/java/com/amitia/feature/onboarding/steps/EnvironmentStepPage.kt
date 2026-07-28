package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
    Box(modifier = modifier) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.Environment)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "2 / 6")
                OnboardingTitle(text = "准备运行环境")
                OnboardingDescription(text = "确认设备满足运行条件，然后安装所需组件。")
            }

            Spacer(modifier = Modifier.height(24.dp))

            when (state.mode) {
                OnboardingRunMode.Local -> {
                    if (state.envItems.isEmpty()) {
                        PrimaryGlassButton(
                            text = "开始检查",
                            onClick = onCheck,
                            modifier = Modifier.fillMaxWidth()
                        )
                    } else {
                        state.envItems.forEach { item ->
                            ProcessRowItem(
                                title = item.name,
                                description = item.detail,
                                state = if (item.passed) ProcessState.Done else ProcessState.Pending
                            )
                        }
                    }

                    if (state.envChecking) {
                        ProcessRowItem(
                            title = "正在检查...",
                            description = "",
                            state = ProcessState.Running
                        )
                    }

                    if (state.allEnvRequiredPassed) {
                        Spacer(modifier = Modifier.height(20.dp))

                        if (state.runtimeItems.isEmpty()) {
                            PrimaryGlassButton(
                                text = "开始安装",
                                onClick = onInstall,
                                modifier = Modifier.fillMaxWidth()
                            )
                        } else {
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

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            val nextEnabled = when (state.mode) {
                OnboardingRunMode.Local -> state.runtimeItems.isNotEmpty() &&
                    state.runtimeItems.all { it.status == InstallStatus.Done }
                OnboardingRunMode.Remote -> state.remoteConnected
                null -> false
            }
            PrimaryGlassButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = nextEnabled
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
