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
                    text = if (state.accountLogin) "登录" else "创建管理账号"
                )
                OnboardingDescription(
                    text = if (state.accountLogin) "填写以下信息继续。"
                    else "创建你的管理员账号以管理 Amitia。"
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            if (state.accountLogin) {
                SoftField(
                    label = "账号",
                    value = state.accountEmail,
                    onValueChange = { onFieldChange("email", it) },
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(12.dp))
                SoftField(
                    label = "密码",
                    value = state.accountPassword,
                    onValueChange = { onFieldChange("password", it) },
                    modifier = Modifier.fillMaxWidth(),
                    isPassword = true
                )
            } else {
                SoftField(
                    label = "用户名",
                    value = state.accountUsername,
                    onValueChange = { onFieldChange("username", it) },
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(12.dp))
                SoftField(
                    label = "邮箱",
                    value = state.accountEmail,
                    onValueChange = { onFieldChange("email", it) },
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(12.dp))
                SoftField(
                    label = "密码",
                    value = state.accountPassword,
                    onValueChange = { onFieldChange("password", it) },
                    modifier = Modifier.fillMaxWidth(),
                    isPassword = true
                )
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
                text = if (state.accountLogin) "没有账号？注册" else "已有账号？登录",
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
                text = if (state.accountLogin) "登录" else "创建账号",
                onClick = { if (onSubmit()) onNext() },
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}
