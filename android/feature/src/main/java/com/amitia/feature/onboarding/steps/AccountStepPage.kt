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
fun AccountStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onToggleLogin: () -> Unit,
    onSubmit: () -> Boolean,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.Account)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "3 / 6")
                OnboardingTitle(
                    text = if (state.accountLogin) "登录管理账号" else "创建管理账号"
                )
                OnboardingDescription(
                    text = "用于保护角色配置、聊天记录和记忆数据。"
                )
            }

            Spacer(modifier = Modifier.height(18.dp))

            SoftField(
                label = "管理员名称",
                value = state.accountUsername,
                onValueChange = { onFieldChange("username", it) },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(12.dp))

            SoftField(
                label = "管理密码",
                value = state.accountPassword,
                onValueChange = { onFieldChange("password", it) },
                modifier = Modifier.fillMaxWidth(),
                isPassword = true
            )

            if (!state.accountLogin) {
                Spacer(modifier = Modifier.height(12.dp))

                SoftField(
                    label = "确认密码",
                    value = state.accountConfirmPassword,
                    onValueChange = { onFieldChange("confirm", it) },
                    modifier = Modifier.fillMaxWidth(),
                    isPassword = true
                )
            }

            Spacer(modifier = Modifier.height(16.dp))

            InlineLink(
                text = if (state.accountLogin) "还没有账号？创建账号" else "已有账号？直接登录",
                onClick = onToggleLogin
            )

            val error = if (state.accountLogin) state.loginError else state.registerError
            if (error != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = error,
                    color = Color(0xFFD64545),
                    fontSize = 12.sp
                )
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = if (state.accountLogin) "登录并继续" else "注册并继续",
                onClick = { if (onSubmit()) onNext() },
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}
