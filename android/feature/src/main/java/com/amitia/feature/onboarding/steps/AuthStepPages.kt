package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaPasswordField
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun RegisterStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onSubmit: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            StepTitle(text = "创建账号")
            StepDescription(text = "填写以下信息创建你的 Amitia 账号。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaTextField(
                value = state.accountUsername,
                onValueChange = { onFieldChange("username", it) },
                label = "用户名",
                placeholder = "请输入用户名",
                leadingIcon = AmitiaIcons.Person,
                isError = state.registerError?.contains("用户名") == true,
                errorMessage = if (state.registerError?.contains("用户名") == true) state.registerError else null
            )
            AmitiaTextField(
                value = state.accountEmail,
                onValueChange = { onFieldChange("email", it) },
                label = "邮箱",
                placeholder = "请输入邮箱",
                leadingIcon = AmitiaIcons.Email,
                isError = state.registerError?.contains("邮箱") == true,
                errorMessage = if (state.registerError?.contains("邮箱") == true) state.registerError else null
            )
            AmitiaPasswordField(
                value = state.accountPassword,
                onValueChange = { onFieldChange("password", it) },
                label = "密码",
                isError = state.registerError?.contains("密码") == true,
                errorMessage = if (state.registerError?.contains("密码") == true && !state.registerError.contains("确认")) state.registerError else null
            )
            AmitiaPasswordField(
                value = state.accountConfirmPassword,
                onValueChange = { onFieldChange("confirm", it) },
                label = "确认密码",
                isError = state.registerError?.contains("一致") == true,
                errorMessage = if (state.registerError?.contains("一致") == true) state.registerError else null
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "创建账号",
                onClick = onSubmit,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.PersonAdd
            )
            TertiaryButton(
                text = "返回",
                onClick = onBack,
                leadingIcon = AmitiaIcons.ArrowBack
            )
        }
    }
}

@Composable
fun LoginStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onSubmit: () -> Unit,
    onForgotPassword: () -> Unit,
    onBack: () -> Unit,
    remoteAddress: String = "",
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            StepTitle(text = "登录")
            StepDescription(text = "使用已有账号登录 Amitia。")
            if (remoteAddress.isNotBlank()) {
                Text(
                    text = "服务地址：$remoteAddress",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth()
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaTextField(
                value = state.accountEmail,
                onValueChange = { onFieldChange("email", it) },
                label = "账号",
                placeholder = "邮箱或用户名",
                leadingIcon = AmitiaIcons.Person
            )
            AmitiaPasswordField(
                value = state.accountPassword,
                onValueChange = { onFieldChange("password", it) },
                label = "密码"
            )
            TertiaryButton(
                text = "忘记密码",
                onClick = onForgotPassword,
                leadingIcon = AmitiaIcons.LockOutlined
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "登录",
                onClick = onSubmit,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Login
            )
            TertiaryButton(
                text = "返回注册",
                onClick = onBack,
                leadingIcon = AmitiaIcons.ArrowBack
            )
        }
    }
}

@Preview(name = "Register - Light", showBackground = true)
@Composable
private fun RegisterStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RegisterStepPage(
            state = OnboardingFlowUiState(
                accountUsername = "amitia_user",
                accountEmail = "user@example.com"
            ),
            onFieldChange = { _, _ -> },
            onSubmit = {},
            onBack = {}
        )
    }
}

@Preview(name = "Register - Dark", showBackground = true)
@Composable
private fun RegisterStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RegisterStepPage(
            state = OnboardingFlowUiState(
                registerError = "两次密码不一致"
            ),
            onFieldChange = { _, _ -> },
            onSubmit = {},
            onBack = {}
        )
    }
}

@Preview(name = "Login - Light", showBackground = true)
@Composable
private fun LoginStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        LoginStepPage(
            state = OnboardingFlowUiState(),
            onFieldChange = { _, _ -> },
            onSubmit = {},
            onForgotPassword = {},
            onBack = {},
            remoteAddress = "https://amitia.example.com"
        )
    }
}

@Preview(name = "Login - Dark", showBackground = true)
@Composable
private fun LoginStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        LoginStepPage(
            state = OnboardingFlowUiState(),
            onFieldChange = { _, _ -> },
            onSubmit = {},
            onForgotPassword = {},
            onBack = {}
        )
    }
}
