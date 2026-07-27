package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun RemoteConfigStepPage(
    state: OnboardingFlowUiState,
    onAddressChange: (String) -> Unit,
    onPortChange: (String) -> Unit,
    onTest: () -> Unit,
    onNext: () -> Unit,
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
            StepTitle(text = "远程服务配置")
            StepDescription(text = "连接已有的 Amitia 服务端，请填写服务地址。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaTextField(
                value = state.remoteAddress,
                onValueChange = onAddressChange,
                label = "服务地址",
                placeholder = "https://example.com",
                leadingIcon = AmitiaIcons.Link
            )
            AmitiaTextField(
                value = state.remotePort,
                onValueChange = onPortChange,
                label = "自定义端口（可选）",
                placeholder = "8443",
                leadingIcon = AmitiaIcons.Router
            )
            HttpsStatusBadge(verified = state.remoteHttpsVerified, connected = state.remoteConnected)
            if (state.remoteTesting) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    AmitiaLoadingIndicator()
                    Text(
                        text = "正在测试连接…",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            state.remoteError?.let { error ->
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = AmitiaCardShape,
                    color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.3f)
                ) {
                    Text(
                        text = error,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            LoadingButton(
                text = "测试连接",
                onClick = onTest,
                modifier = Modifier.fillMaxWidth(),
                enabled = state.remoteAddress.isNotBlank() && !state.remoteTesting,
                loading = state.remoteTesting,
                leadingIcon = AmitiaIcons.WifiOff
            )
            PrimaryButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = state.remoteConnected,
                leadingIcon = AmitiaIcons.ArrowForward
            )
        }
    }
}

@Composable
private fun HttpsStatusBanner(verified: Boolean, connected: Boolean) {
    if (!connected) return
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = if (verified) AmitiaStateColors.Running.copy(alpha = 0.1f)
        else AmitiaStateColors.Degraded.copy(alpha = 0.1f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(24.dp)
                    .clip(CircleShape)
                    .background(if (verified) AmitiaStateColors.Running else AmitiaStateColors.Degraded),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (verified) AmitiaIcons.Lock else AmitiaIcons.Warning,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(16.dp)
                )
            }
            Column {
                Text(
                    text = if (verified) "HTTPS 已验证" else "HTTPS 未验证",
                    style = MaterialTheme.typography.labelMedium,
                    color = if (verified) AmitiaStateColors.Running else AmitiaStateColors.Degraded,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = if (verified) "连接已加密" else "连接未加密，建议使用 HTTPS",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun HttpsStatusBadge(verified: Boolean, connected: Boolean) {
    HttpsStatusBanner(verified = verified, connected = connected)
}

@Preview(name = "RemoteConfig - Light", showBackground = true)
@Composable
private fun RemoteConfigStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RemoteConfigStepPage(
            state = OnboardingFlowUiState(
                remoteAddress = "https://amitia.example.com",
                remotePort = "8443",
                remoteHttpsVerified = true,
                remoteConnected = true
            ),
            onAddressChange = {},
            onPortChange = {},
            onTest = {},
            onNext = {}
        )
    }
}

@Preview(name = "RemoteConfig - Dark", showBackground = true)
@Composable
private fun RemoteConfigStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RemoteConfigStepPage(
            state = OnboardingFlowUiState(
                remoteAddress = "amitia.example.com",
                remoteError = "服务地址无效"
            ),
            onAddressChange = {},
            onPortChange = {},
            onTest = {},
            onNext = {}
        )
    }
}
